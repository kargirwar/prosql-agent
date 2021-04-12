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
	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"sync"
	"time"
)

//==============================================================//
//          session structs and methods
//==============================================================//
type session struct {
	id         string
	pool       *sql.DB
	in         chan *Req
	out        chan *Res
	accessTime time.Time
	cursors    map[string]*cursor
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

var allsessions *sessions
var ticker *time.Ticker

func init() {
	store := make(map[string]*session)
	allsessions = &sessions{
		store: store,
	}
	ticker = time.NewTicker(CLEANUP_INTERVAL)
	go cleanup()
}

//for all sessions whose accesstime is older than CLEANUP_INTERVAL,
//clean up active cursors if any and then delete the session itself
func cleanup() {
	for {
		select {
		case <-ticker.C:
            log.Println("Starting cleanup")
            keys := allsessions.getKeys()
			for _, k := range keys {
                log.Println("Checking session: " + k)
                s, _ := allsessions.get(k)
                if s == nil {
                    log.Println("Skipping session: " + k)
                    continue
                }

				now := time.Now()
				if now.Sub(s.accessTime) > CLEANUP_INTERVAL {
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

					allsessions.clear(k)
                    log.Println("Cleanup done for session: " + k)
				}
			}
		}
	}
}

func NewSession(dbtype string, dsn string) (string, error) {
	s, err := createSession(dbtype, dsn)
	if err != nil {
		return "", err
	}

	allsessions.set(s.id, s)

	go sessionHandler(s)
	return s.id, nil
}

//execute a query and create a cursor for the results
//results must be retrieved by calling fetch later with the cursor id
func Execute(sid string, query string) (string, error) {
	s, err := allsessions.get(sid)
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
	s, err := allsessions.get(sid)
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

func Cancel(sid string, cid string) error {
	s, err := allsessions.get(sid)
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
	s.cursors = make(map[string]*cursor)

	return &s, nil
}

func sessionHandler(s *session) {
	log.Printf("Starting handler for %s\n", s.id)
	defer s.pool.Close()

	for {
		select {
		case req := <-s.in:
			handleSessionRequest(s, req)
		}
	}
}

func handleSessionRequest(s *session, req *Req) {
    s.accessTime = time.Now()
	switch req.code {
	case CMD_EXECUTE:
		handleExecute(s, req)

	case CMD_FETCH:
		handleFetch(s, req)

	case CMD_CANCEL:
		handleCancel(s, req)

    case CMD_CLEANUP:
		handleCleanup(s, req)
	}
}

//send cleanup command to all the cursors
func handleCleanup(s *session, req *Req) {
	log.Println("Handling " + CMD_CLEANUP)
    for _, c := range(s.cursors) {
        c.in <- &Req{
            code: CMD_CANCEL,
        }
        <-c.out
    }

    s.out <- &Res{
        code: CLEANUP_DONE,
    }
}

func handleExecute(s *session, req *Req) {
	query, _ := req.data.(string)
	log.Println("Handling " + CMD_EXECUTE + ":" + query)

	c, err := NewCursor(s, query)
	if err != nil {
		s.out <- &Res{
			code: ERROR,
			data: err,
		}

		return
	}

	s.cursors[c.id] = c
	s.out <- &Res{
		code: SUCCESS,
		data: c.id,
	}
}

func handleFetch(s *session, req *Req) {
	//just pass on to appropriate cursor and wait for results
	fetchReq, _ := req.data.(FetchReq)
	c, present := s.cursors[fetchReq.cid]

	if !present {
		s.out <- &Res{
			code: ERROR,
			data: errors.New(ERR_INVALID_CURSOR_ID),
		}
		return
	}

	//send fetch request to cursor
	c.in <- req
	res := <-c.out
	if res.code == ERROR {
		//if there is error the cursorHandler must have exited. No need to
		//keep reference to it
		delete(s.cursors, fetchReq.cid)
	}
	s.out <- res

}

func handleCancel(s *session, req *Req) {
	log.Println("Handling CMD_CANCEL")
	cid := req.data.(string)
	c, present := s.cursors[cid]

	if !present {
		s.out <- &Res{
			code: ERROR,
			data: errors.New(ERR_INVALID_CURSOR_ID),
		}
		return
	}

	c.cancel()
	s.out <- &Res{
		code: SUCCESS,
	}
}
