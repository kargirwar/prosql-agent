/* One session corresponds to one tab in the browser. If the tab is duplicated
they will share the session. Each session can execute multiple queries. Each
query will be associated with a cursor. A cursor can be queried untill all
rows have been read*/
/* There is one goroutine corresponding to each session. Similarly there
is one goroutine per cursor */

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
)

//==============================================================//
//          session structs and methods
//==============================================================//
type session struct {
	id          string
	pool        *sql.DB
	in          chan *Req
	out         chan *Res
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
	ctx  context.Context
	code string
	data interface{}
}

type Res struct {
	ctx  context.Context
	code string
	data interface{}
}

type FetchReq struct {
	cid string
	n   int
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
			log.Printf("Starting session cleanup")
			keys := sessionStore.getKeys()
			for _, k := range keys {
				log.Printf("%s: Checking session\n", k)
				s, err := sessionStore.get(k)
				if err != nil {
					log.Printf("%s: Skipping session\n", k)
					continue
				}

				now := time.Now()
				if now.Sub(s.getAccessTime()) > SESSION_CLEANUP_INTERVAL {
					log.Printf("%s: Cleaning up session\n", k)
					s.in <- &Req{
						code: CMD_CLEANUP,
					}

					res := <-s.out
					if res.code == ERROR {
						//TODO: What are we going to do here?
						log.Printf("%s: Unable to cleanup\n", k)
						continue
					}

					log.Printf("%s: Cleanup done\n", k)
					sessionStore.clear(k)
					log.Printf("%s: Clear done\n", k)
				}
			}

			log.Printf("Done session cleanup")
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

	go sessionHandler(s)
	return s.id, nil
}

func SetDb(sid string, db string) error {

	s, err := sessionStore.get(sid)
	if err != nil {
		return err
	}

	s.in <- &Req{
		code: CMD_SET_DB,
		data: db,
	}

	res := <-s.out
	if res.code == ERROR {
		return res.data.(error)
	}

	return nil
}

//execute a query and create a cursor for the results
//results must be retrieved by calling fetch later with the cursor id
func Execute(ctx context.Context, sid string, query string) (string, error) {
	defer TimeTrack(ctx, time.Now())

	s, err := sessionStore.get(sid)
	if err != nil {
		return "", err
	}

	Dbg(ctx, fmt.Sprintf("session id: %s", s.id))

	//we ask the session handler to create a new cursor and return its id
	s.in <- &Req{
		ctx:  ctx,
		code: CMD_EXECUTE,
		data: query,
	}

	Dbg(ctx, fmt.Sprintf("%s Send CMD_EXECUTE for %s", s.id, query))

	res := <-s.out
	if res.code == ERROR {
		return "", res.data.(error)
	}

	Dbg(ctx, fmt.Sprintf("%s Received Response for %s", s.id, query))
	return res.data.(string), nil
}

//fetch n rows from session sid using cursor cid
func Fetch(ctx context.Context, sid string, cid string, n int) (*[][]string, bool, error) {
	defer TimeTrack(ctx, time.Now())

	s, err := sessionStore.get(sid)
	if err != nil {
		return nil, false, err
	}

	s.in <- &Req{
		ctx:  ctx,
		code: CMD_FETCH,
		data: FetchReq{
			cid: cid,
			n:   n,
		},
	}

	res := <-s.out
	log.Printf("FETCH s: %s c: %s code %s\n", s.id, cid, res.code)

	if res.code == ERROR {
		return nil, false, res.data.(error)
	}

	var eof bool
	if res.code == EOF {
		eof = true
	}

	return res.data.(*[][]string), eof, nil
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

	var s session
	s.pool = pool
	s.accessTime = time.Now()
	s.in = make(chan *Req)
	s.out = make(chan *Res)
	s.id = uniuri.New()
	s.cursorStore = NewCursorStore()

	return &s, nil
}
