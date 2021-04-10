/* One session corresponds to one tab in the browser. If the tab is duplicated
they will share the session. Each session can execute multiple queries. Each
query will be associated with a cursor. A cursor can be queried untill all
rows have been read*/
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

var smutex sync.Mutex

type session struct {
	id         string
	pool       *sql.DB
	in         chan *Req
	out        chan *Res
	accessTime time.Time
	cursors    map[string]*cursor
}

var sessions map[string]*session

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

func init() {
	sessions = make(map[string]*session)
}

func NewSession(dbtype string, dsn string) (string, error) {
	//type = mysql for the time being
	//create a session goroutine and assign an id to it
	smutex.Lock()
	defer smutex.Unlock()

	s, err := createSession(dbtype, dsn)
	if err != nil {
		return "", err
	}

	sessions[s.id] = s

	go sessionHandler(s)
	return s.id, nil
}

//execute a query and create a cursor for the results
//results must be retrieved by calling fetch later with the cursor id
func Execute(sid string, query string) (string, error) {
	s, present := sessions[sid]
	if !present {
		return "", errors.New(ERR_INVALID_SESSION_ID)
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
	s, present := sessions[sid]
	if !present {
		return nil, false, errors.New(ERR_INVALID_SESSION_ID)
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
	s, present := sessions[sid]
	if !present {
		return errors.New(ERR_INVALID_SESSION_ID)
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
	switch req.code {
	case CMD_EXECUTE:
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

	case CMD_FETCH:
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

	case CMD_CANCEL:
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
}
