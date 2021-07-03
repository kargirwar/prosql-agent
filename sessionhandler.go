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
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//goroutine to deal with one session
func sessionHandler(ctx context.Context, s *session) {
	Dbg(ctx, fmt.Sprintf("Starting session handler for %s\n", s.id))
	defer s.pool.Close()

	ticker := time.NewTicker(CURSOR_CLEANUP_INTERVAL)

loop:
	for {
		select {
		case req := <-s.in:
			if req.code == CMD_CLEANUP {
				Dbg(ctx, fmt.Sprintf("Shutting down session handler for %s\n", s.id))
				handleCleanup(ctx, s, req)
				break loop
			}
			go handleSessionRequest(ctx, s, req)

		case <-ticker.C:
			cleanupCursors(ctx, s)
		}
	}
}

//cleanup cursors which have not been accessed for CURSOR_CLEANUP_INTERVAL
func cleanupCursors(ctx context.Context, s *session) {
	Dbg(ctx, fmt.Sprintf("%s: Starting cleanup", s.id))
	keys := s.cursorStore.getKeys()
	for _, k := range keys {
		Dbg(ctx, fmt.Sprintf("%s: Checking cursor: %s\n", s.id, k))
		c, err := s.cursorStore.get(k)
		if err != nil {
			Dbg(ctx, fmt.Sprintf("%s: Skipping cursor: %s\n", s.id, k))
			continue
		}

		now := time.Now()
		if now.Sub(c.getAccessTime()) > CURSOR_CLEANUP_INTERVAL {
			Dbg(ctx, fmt.Sprintf("%s: Cleaning up cursor: %s\n", s.id, k))
			//This will handle both cases: either the cursor is in the middle of a query
			//or waiting for a command from session handler
			c.cancel()

			Dbg(ctx, fmt.Sprintf("%s: Cleanup done for cursor: %s\n", s.id, k))
			s.cursorStore.clear(k)
			Dbg(ctx, fmt.Sprintf("%s: Clear done for cursor: %s\n", s.id, k))
		}
	}

	Dbg(ctx, fmt.Sprintf("%s: Done cleanup", s.id))
}

func handleSessionRequest(ctx context.Context, s *session, req *Req) {
	defer TimeTrack(req.ctx, time.Now())

	s.setAccessTime()
	switch req.code {
	case CMD_SET_DB:
		handleSetDb(s, req)

	case CMD_QUERY:
		handleQuery(s, req)

	case CMD_EXECUTE:
		handleExecute(s, req)

	case CMD_FETCH:
		handleFetch(s, req)

	case CMD_FETCH_WS:
		handleFetch_ws(s, req)

	case CMD_CANCEL:
		handleCancel(s, req)
	}
}

//cleanup cursors *unconditionally*
func handleCleanup(ctx context.Context, s *session, req *Req) {
	Dbg(ctx, fmt.Sprintf("%s: Handling CMD_CLEANUP\n", s.id))

	keys := s.cursorStore.getKeys()
	for _, k := range keys {
		c, err := s.cursorStore.get(k)
		Dbg(ctx, fmt.Sprintf("%s: Cleaning up cursor: %s\n", s.id, k))
		//This will handle both cases: either the cursor is in the middle of a query
		//or waiting for a command from session handler
		if err != nil {
			s.cursorStore.clear(k)
			Dbg(ctx, fmt.Sprintf("%s cleaned up invalid cursor: %s\n", s.id, k))
			continue
		}
		c.cancel()

		Dbg(ctx, fmt.Sprintf("%s: Cleanup done for cursor: %s\n", s.id, k))
		s.cursorStore.clear(k)
		Dbg(ctx, fmt.Sprintf("%s: Clear done for cursor: %s\n", s.id, k))
	}

	req.resChan <- &Res{
		code: CLEANUP_DONE,
	}

	Dbg(ctx, fmt.Sprintf("%s: Done CMD_CLEANUP\n", s.id))
}

func handleSetDb(s *session, req *Req) {
	db, _ := req.data.(string)
	Dbg(req.ctx, fmt.Sprintf("%s: Handling CMD_SET_DB for: %s\n", s.id, db))
	//clear all existing cursors
	cleanupCursors(req.ctx, s)

	rows, err := s.pool.QueryContext(context.Background(), "use `"+db+"`")
	if err != nil {
		req.resChan <- &Res{
			code: ERROR,
			data: err,
		}

		return
	}

	rows.Close()

	req.resChan <- &Res{
		code: SUCCESS,
	}

	Dbg(req.ctx, fmt.Sprintf("%s: Done CMD_SET_DB for: %s\n", s.id, db))
}

func handleQuery(s *session, req *Req) {
	defer TimeTrack(req.ctx, time.Now())

	query, _ := req.data.(string)
	Dbg(req.ctx, fmt.Sprintf("%s: Handling CMD_QUERY for: %s\n", s.id, query))

	c := NewQueryCursor(req.ctx)
	c.start(req.ctx, s, query)
	s.cursorStore.set(c.id, c)

	Dbg(req.ctx, fmt.Sprintf("%s: Done CMD_QUERY for: %s\n", s.id, query))

	req.resChan <- &Res{
		code: SUCCESS,
		data: c.id,
	}

	Dbg(req.ctx, fmt.Sprintf("%s: Sent Response CMD_QUERY for: %s\n", s.id, query))
}

func handleExecute(s *session, req *Req) {
	defer TimeTrack(req.ctx, time.Now())

	query, _ := req.data.(string)
	Dbg(req.ctx, fmt.Sprintf("%s: Handling CMD_EXECUTE for: %s\n", s.id, query))

	c := NewExecuteCursor(req.ctx)
	n, err := c.execute(req.ctx, s, query)

	if err != nil {
		req.resChan <- &Res{
			code: ERROR,
			data: err,
		}
		return
	}

	Dbg(req.ctx, fmt.Sprintf("%s: Done CMD_EXECUTE for: %s\n", s.id, query))

	req.resChan <- &Res{
		code: SUCCESS,
		data: n,
	}

	Dbg(req.ctx, fmt.Sprintf("%s: Sent Response CMD_EXECUTE for: %s\n", s.id, query))
}

func handleFetch_ws(s *session, req *Req) {
	//just pass on to appropriate cursor
	fetchReq, _ := req.data.(FetchReq)
	c, err := s.cursorStore.get(fetchReq.cid)

	if err != nil {
		req.resChan <- &Res{
			code: ERROR,
			data: err,
		}
		return
	}

	Dbg(req.ctx, fmt.Sprintf("%s: Handling CMD_FETCH_WS for: %s\n", s.id, c.id))

	//send fetch request to cursor
	c.in <- req
	res := <-c.out

	//Dbg(req.ctx, fmt.Sprintf("%s: Done CMD_FETCH for: %s with code: %s\n", s.id, c.id, res.code))
	if res.code == ERROR || res.code == EOF {
		Dbg(req.ctx, fmt.Sprintf("%s: clearing cursor %s\n", s.id, c.id))
		s.cursorStore.clear(c.id)
	}
	req.resChan <- res
}

func handleFetch(s *session, req *Req) {
	//just pass on to appropriate cursor and wait for results
	fetchReq, _ := req.data.(FetchReq)
	c, err := s.cursorStore.get(fetchReq.cid)

	if err != nil {
		req.resChan <- &Res{
			code: ERROR,
			data: err,
		}
		return
	}

	Dbg(req.ctx, fmt.Sprintf("%s: Handling CMD_FETCH for: %s\n", s.id, c.id))

	//send fetch request to cursor
	c.in <- req
	res := <-c.out

	Dbg(req.ctx, fmt.Sprintf("%s: Done CMD_FETCH for: %s with code: %s\n", s.id, c.id, res.code))
	if res.code == ERROR || res.code == EOF {
		Dbg(req.ctx, fmt.Sprintf("%s: clearing cursor %s\n", s.id, c.id))
		s.cursorStore.clear(c.id)
	}
	req.resChan <- res
}

func handleCancel(s *session, req *Req) {
	cid := req.data.(string)
	c, err := s.cursorStore.get(cid)

	Dbg(req.ctx, fmt.Sprintf("%s: Handling CMD_CANCEL for: %s\n", s.id, c.id))

	if err != nil {
		req.resChan <- &Res{
			code: ERROR,
			data: err,
		}
		return
	}

	c.cancel()
	s.cursorStore.clear(cid)

	req.resChan <- &Res{
		code: SUCCESS,
	}
	Dbg(req.ctx, fmt.Sprintf("%s: Done CMD_CANCEL for: %s\n", s.id, c.id))
}
