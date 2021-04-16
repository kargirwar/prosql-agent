package main

import (
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//goroutine to deal with one session
func sessionHandler(s *session) {
	log.Printf("Starting session handler for %s\n", s.id)
	defer s.pool.Close()

	ticker := time.NewTicker(CURSOR_CLEANUP_INTERVAL)

loop:
	for {
		select {
		case req := <-s.in:
			handleSessionRequest(s, req)

			if req.code == CMD_CLEANUP {
				log.Printf("Shutting down session handler for %s\n", s.id)
				break loop
			}

		case <-ticker.C:
			cleanupCursors(s)
		}
	}
}

//cleanup cursors which have not been accessed for CURSOR_CLEANUP_INTERVAL
func cleanupCursors(s *session) {
	log.Printf("%s: Starting cleanup\n", s.id)
	keys := s.cursorStore.getKeys()
	for _, k := range keys {
		log.Printf("%s: Checking cursor: %s\n", s.id, k)
		c, _ := s.cursorStore.get(k)
		if c == nil {
			log.Printf("%s: Skipping cursor: %s\n", s.id, k)
			continue
		}

		now := time.Now()
		if now.Sub(c.getAccessTime()) > CURSOR_CLEANUP_INTERVAL {
			log.Printf("%s: Cleaning up cursor: %s\n", s.id, k)
			//This will handle both cases: either the cursor is in the middle of a query
			//or waiting for a command from session handler
			c.cancel()

			log.Printf("%s: Cleanup done for cursor: %s\n", s.id, k)
			s.cursorStore.clear(k)
			log.Printf("%s: Clear done for cursor: %s\n", s.id, k)
		}
	}

	log.Printf("%s: Done cleanup\n", s.id)
}

func handleSessionRequest(s *session, req *Req) {
	s.setAccessTime()
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

//cleanup cursors *unconditionally*
func handleCleanup(s *session, req *Req) {
	log.Printf("%s: Handling CMD_CLEANUP\n", s.id)

	keys := s.cursorStore.getKeys()
	for _, k := range keys {
		c, _ := s.cursorStore.get(k)
		log.Printf("%s: Cleaning up cursor: %s\n", s.id, k)
		//This will handle both cases: either the cursor is in the middle of a query
		//or waiting for a command from session handler
		c.cancel()

		log.Printf("%s: Cleanup done for cursor: %s\n", s.id, k)
		s.cursorStore.clear(k)
		log.Printf("%s: Clear done for cursor: %s\n", s.id, k)
	}

	s.out <- &Res{
		code: CLEANUP_DONE,
	}

	log.Printf("%s: Done CMD_CLEANUP\n", s.id)
}

func handleExecute(s *session, req *Req) {
	query, _ := req.data.(string)
	log.Printf("%s: Handling CMD_EXECUTE for: %s\n", s.id, query)

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

	log.Printf("%s: Done CMD_EXECUTE for: %s\n", s.id, query)
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

	log.Printf("%s: Handling CMD_FETCH for: %s\n", s.id, c.id)

	//send fetch request to cursor
	c.in <- req
	res := <-c.out
	if res.code == ERROR {
		//if there is error the cursorHandler must have exited. No need to
		//keep reference to it
		s.cursorStore.clear(fetchReq.cid)
	}
	s.out <- res
	log.Printf("%s: Done CMD_FETCH for: %s\n", s.id, c.id)
}

func handleCancel(s *session, req *Req) {
	cid := req.data.(string)
	c, err := s.cursorStore.get(cid)

	log.Printf("%s: Handling CMD_CANCEL for: %s\n", s.id, c.id)

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
	log.Printf("%s: Done CMD_CANCEL for: %s\n", s.id, c.id)
}
