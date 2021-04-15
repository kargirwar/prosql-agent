/* One session corresponds to one tab in the browser. If the tab is duplicated
they will share the session. Each session can execute multiple queries. Each
query will be associated with a cursor. A cursor can be queried untill all
rows have been read*/
/* There is one goroutine corresponding to each session. Similarly there
is one goroutine per cursor */

package main

import (
	"database/sql"
	"errors"
	"log"
	"runtime/pprof"
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
}

type sessions struct {
	store  map[string]*session
	smutex sync.Mutex
}

func (ps *sessions) set(sid string, s *session) {
	ps.smutex.Lock()
	defer ps.smutex.Unlock()

	ps.store[sid] = s
}

func (ps *sessions) get(sid string) (*session, error) {
	ps.smutex.Lock()
	defer ps.smutex.Unlock()

	s, present := ps.store[sid]
	if !present {
		return nil, errors.New(ERR_INVALID_SESSION_ID)
	}

	return s, nil
}

func (ps *sessions) getKeys() []string {
	ps.smutex.Lock()
	defer ps.smutex.Unlock()

	keys := make([]string, len(ps.store))

	i := 0
	for k := range ps.store {
		keys[i] = k
		i++
	}

	return keys
}

func (ps *sessions) clear(k string) {
	ps.smutex.Lock()
	defer ps.smutex.Unlock()

	_, present := ps.store[k]
	if present {
		delete(ps.store, k)
	}
}

//==============================================================//
//          session structs and methods end
//==============================================================//

type Req struct {
	code string
	data interface{}
}

type Res struct {
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
			log.Println("Starting cleanup")
			keys := sessionStore.getKeys()
			for _, k := range keys {
				log.Println("Checking session: " + k)
				s, _ := sessionStore.get(k)
				if s == nil {
					log.Println("Skipping session: " + k)
					continue
				}

				now := time.Now()
				if now.Sub(s.accessTime) > SESSION_CLEANUP_INTERVAL {
					log.Println("Cleaning up session: " + k)
					s.in <- &Req{
						code: CMD_CLEANUP,
					}

					res := <-s.out
					if res.code == ERROR {
						//TODO: What are we going to do here?
						log.Println("Unable to cleanup " + k)
						continue
					}

					sessionStore.clear(k)
					log.Println("Cleanup done for session: " + k)
				}
			}

			if err := pprof.WriteHeapProfile(mp); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}
		}
	}
}

func NewSession(dbtype string, dsn string) (string, error) {
	s, err := createSession(dbtype, dsn)
	if err != nil {
		return "", err
	}

	sessionStore.set(s.id, s)

	go sessionHandler(s)
	return s.id, nil
}

//execute a query and create a cursor for the results
//results must be retrieved by calling fetch later with the cursor id
func Execute(sid string, query string) (string, error) {
	s, err := sessionStore.get(sid)
	if err != nil {
		return "", err
	}
	//we ask the session handler to create a new cursor and return its id
	s.in <- &Req{
		code: CMD_EXECUTE,
		data: query,
	}

	res := <-s.out
	if res.code == ERROR {
		return "", res.data.(error)
	}

	return res.data.(string), nil
}

//fetch n rows from session sid using cursor cid
func Fetch(sid string, cid string, n int) (*[][]string, bool, error) {
	s, err := sessionStore.get(sid)
	if err != nil {
		return nil, false, err
	}

	s.in <- &Req{
		code: CMD_FETCH,
		data: FetchReq{
			cid: cid,
			n:   n,
		},
	}

	res := <-s.out
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
func Cancel(sid string, cid string) error {
	s, err := sessionStore.get(sid)
	if err != nil {
		return err
	}

	s.in <- &Req{
		code: CMD_CANCEL,
		data: cid,
	}

	res := <-s.out
	if res.code == ERROR {
		return res.data.(error)
	}

	return nil
}

func createSession(dbtype string, dsn string) (*session, error) {
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
