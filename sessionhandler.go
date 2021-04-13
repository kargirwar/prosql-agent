package main

import (
	_ "github.com/go-sql-driver/mysql"
	"log"
	"time"
)

//goroutine to deal with one session
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
	keys := s.cursorStore.getKeys()
	for _, k := range keys {
		c, _ := s.cursorStore.get(k)
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

	s.cursorStore.set(c.id, c)
	s.out <- &Res{
		code: SUCCESS,
		data: c.id,
	}
}

func handleFetch(s *session, req *Req) {
	//just pass on to appropriate cursor and wait for results
	fetchReq, _ := req.data.(FetchReq)
	c, err := s.cursorStore.get(fetchReq.cid)

	if err != nil {
		s.out <- &Res{
			code: ERROR,
			data: err,
		}
		return
	}

	//send fetch request to cursor
	c.in <- req
	res := <-c.out
	if res.code == ERROR {
		//if there is error the cursorHandler must have exited. No need to
		//keep reference to it
		s.cursorStore.clear(fetchReq.cid)
	}
	s.out <- res
}

func handleCancel(s *session, req *Req) {
	log.Println("Handling CMD_CANCEL")
	cid := req.data.(string)
	c, err := s.cursorStore.get(cid)

	if err != nil {
		s.out <- &Res{
			code: ERROR,
			data: err,
		}
		return
	}

	c.cancel()
	s.cursorStore.clear(cid)

	s.out <- &Res{
		code: SUCCESS,
	}
}
