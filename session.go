/* One session corresponds to one tab in the browser. If the tab is duplicated
they will share the session. Each session can execute multiple queries. Each
query will be associated with a cursor. A cursor can be queried untill all
rows have been read*/
/* There is one goroutine corresponding to each session. Similarly there
is one goroutine per cursor */

/* Copyright (C) 2021 Pankaj Kargirwar <kargirwar@protonmail.com>

   This file is part of prosql-agent

   prosql-agent is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   prosql-agent is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with prosql-agent.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

//==============================================================//
//          session structs and methods
//==============================================================//
type session struct {
	id          string
	pool        *sql.DB
	in          chan *Req
	accessTime  time.Time
	cursorStore *cursors
	mutex       sync.Mutex
}

func (ps *session) setAccessTime() {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.accessTime = time.Now()
}

func (ps *session) getAccessTime() time.Time {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	return ps.accessTime
}

func (ps *session) String() string {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	stats := ps.pool.Stats()
	return fmt.Sprintf("max %d open %d inuse %d idle %d wc %d wd %s maxclosed %d",
		stats.MaxOpenConnections, stats.OpenConnections, stats.InUse, stats.Idle,
		stats.WaitCount, stats.WaitDuration, stats.MaxIdleClosed)
}

type sessions struct {
	store map[string]*session
	mutex sync.Mutex
}

func (ps *sessions) set(sid string, s *session) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.store[sid] = s
}

func (ps *sessions) get(sid string) (*session, error) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	s, present := ps.store[sid]
	if !present {
		return nil, errors.New(ERR_INVALID_SESSION_ID)
	}

	return s, nil
}

func (ps *sessions) getKeys() []string {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	keys := make([]string, len(ps.store))

	i := 0
	for k := range ps.store {
		keys[i] = k
		i++
	}

	return keys
}

func (ps *sessions) clear(k string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	_, present := ps.store[k]
	if present {
		delete(ps.store, k)
	}
}

//==============================================================//
//          session structs and methods end
//==============================================================//

type Req struct {
	ctx     context.Context
	code    string
	data    interface{}
	resChan chan *Res
}

type Res struct {
	ctx  context.Context
	code string
	data interface{}
}

type FetchReq struct {
	cid string
	n   int
	ws  *websocket.Conn
}

//==============================================================//
//          globals
//==============================================================//
var sessionStore *sessions
var ticker *time.Ticker

func init() {
	store := make(map[string]*session)
	sessionStore = &sessions{
		store: store,
	}
	ticker = time.NewTicker(SESSION_CLEANUP_INTERVAL)
	go cleanupSessions()
}

//for all sessions whose accesstime is older than SESSION_CLEANUP_INTERVAL,
//clean up active cursors if any and then delete the session itself
func cleanupSessions() {
	for {
		select {
		case <-ticker.C:
			log.Debug("Starting session cleanup")
			keys := sessionStore.getKeys()
			for _, k := range keys {
				log.WithFields(log.Fields{
					"session-id": k,
				}).Debug("Checking session")

				s, err := sessionStore.get(k)
				if err != nil {
					log.WithFields(log.Fields{
						"session-id": k,
					}).Debug("Skipping session")
					continue
				}

				now := time.Now()
				if now.Sub(s.getAccessTime()) > SESSION_CLEANUP_INTERVAL {

					log.WithFields(log.Fields{
						"session-id": k,
					}).Debug("Cleaning up session")

					ch := make(chan *Res)
					s.in <- &Req{
						code:    CMD_CLEANUP,
						resChan: ch,
					}

					res := <-ch
					if res.code == ERROR {
						//TODO: What are we going to do here?
						log.WithFields(log.Fields{
							"session-id": k,
						}).Debug("Unable to cleanup")

						continue
					}

					log.WithFields(log.Fields{
						"session-id": k,
					}).Debug("Cleanup done")

					sessionStore.clear(k)
					log.WithFields(log.Fields{
						"session-id": k,
					}).Debug("Clear done")
				}
			}

			log.Debug("Done session cleanup")
		}
	}
}

//==============================================================//
//         External Interface
//==============================================================//

func NewSession(ctx context.Context, dbtype string, dsn string) (string, error) {
	defer TimeTrack(ctx, time.Now())

	s, err := createSession(ctx, dbtype, dsn)
	if err != nil {
		return "", err
	}

	sessionStore.set(s.id, s)

	go sessionHandler(ctx, s)
	return s.id, nil
}

//execute a query and create a cursor for the results
//results must be retrieved by calling fetch later with the cursor id
func Query(ctx context.Context, sid string, query string) (string, error) {
	defer TimeTrack(ctx, time.Now())

	s, err := sessionStore.get(sid)
	if err != nil {
		return "", err
	}

	Dbg(ctx, fmt.Sprintf("%s", s))

	//we ask the session handler to create a new cursor and return its id
	ch := make(chan *Res)
	s.in <- &Req{
		ctx:     ctx,
		code:    CMD_QUERY,
		data:    query,
		resChan: ch,
	}

	Dbg(ctx, fmt.Sprintf("%s Send CMD_QUERY for %s", s.id, query))

	res := <-ch
	if res.code == ERROR {
		return "", res.data.(error)
	}

	Dbg(ctx, fmt.Sprintf("%s Received Response for %s", s.id, query))
	return res.data.(string), nil
}

//fetch n rows from session sid using cursor cid. The cursor will directly
//send data on websocket channel ws
func Fetch_ws(ctx context.Context, sid string, cid string, ws *websocket.Conn, n int) error {
	defer TimeTrack(ctx, time.Now())

	s, err := sessionStore.get(sid)
	if err != nil {
		return err
	}

	ch := make(chan *Res)
	s.in <- &Req{
		ctx:  ctx,
		code: CMD_FETCH_WS,
		data: FetchReq{
			cid: cid,
			n:   n,
			ws:  ws,
		},
		resChan: ch,
	}

	res := <-ch
	Dbg(ctx, fmt.Sprintf("FETCH s: %s c: %s code %s\n", s.id, cid, res.code))

	if res.code == ERROR {
		return res.data.(error)
	}

	return nil
}

//fetch n rows from session sid using cursor cid
func Fetch(ctx context.Context, sid string, cid string, n int) (*[][]string, bool, error) {
	defer TimeTrack(ctx, time.Now())

	s, err := sessionStore.get(sid)
	if err != nil {
		return nil, false, err
	}

	ch := make(chan *Res)
	s.in <- &Req{
		ctx:  ctx,
		code: CMD_FETCH,
		data: FetchReq{
			cid: cid,
			n:   n,
		},
		resChan: ch,
	}

	res := <-ch
	Dbg(ctx, fmt.Sprintf("FETCH s: %s c: %s code %s\n", s.id, cid, res.code))

	if res.code == ERROR {
		return nil, false, res.data.(error)
	}

	var eof bool
	if res.code == EOF {
		eof = true
	}

	return res.data.(*[][]string), eof, nil
}

func Execute(ctx context.Context, sid string, query string) (int64, error) {
	defer TimeTrack(ctx, time.Now())

	s, err := sessionStore.get(sid)
	if err != nil {
		return -1, err
	}

	Dbg(ctx, fmt.Sprintf("%s", s))

	//we ask the session handler to create a new cursor and return its id
	ch := make(chan *Res)
	s.in <- &Req{
		ctx:     ctx,
		code:    CMD_EXECUTE,
		data:    query,
		resChan: ch,
	}

	Dbg(ctx, fmt.Sprintf("%s Send CMD_EXECUTE for %s", s.id, query))

	res := <-ch
	if res.code == ERROR {
		return -1, res.data.(error)
	}

	Dbg(ctx, fmt.Sprintf("%s Received Response for %s", s.id, query))
	return res.data.(int64), nil
}

//cancel a running query
func Cancel(ctx context.Context, sid string, cid string) error {
	defer TimeTrack(ctx, time.Now())

	s, err := sessionStore.get(sid)
	if err != nil {
		return err
	}

	c, err := s.cursorStore.get(cid)
	if err != nil {
		return err
	}

	c.cancel()

	return nil
}

//==============================================================//
//         External Interface End
//==============================================================//

func createSession(ctx context.Context, dbtype string, dsn string) (*session, error) {
	defer TimeTrack(ctx, time.Now())

	pool, err := sql.Open(dbtype, dsn)

	if err != nil {
		return nil, err
	}

	pool.SetMaxOpenConns(MAX_OPEN_CONNS)
	pool.SetMaxIdleConns(MAX_IDLE_CONNS)

	ctx1, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx1); err != nil {
		return nil, err
	}

	var s session
	s.pool = pool
	s.accessTime = time.Now()
	s.in = make(chan *Req, 100)
	s.id = uniuri.New()
	s.cursorStore = NewCursorStore()

	return &s, nil
}
