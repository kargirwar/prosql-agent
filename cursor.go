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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
)

//==============================================================//
//          cursor structs and methods
//==============================================================//
type cursor struct {
	id         string
	rows       *sql.Rows
	in         chan *Req
	out        chan *Res
	accessTime time.Time
	cancel     context.CancelFunc
	ctx        context.Context
	mutex      sync.Mutex
	err        error
	query      string
}

func (pc *cursor) start(ctx context.Context, s *session, query string) error {
	defer TimeTrack(ctx, time.Now())

	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	Dbg(ctx, "Starting query: "+query)

	rows, err := s.pool.QueryContext(pc.ctx, query)
	if err != nil {
		pc.err = err
		return err
	}

	Dbg(ctx, "Done query: "+query)

	pc.rows = rows
	pc.query = query

	return nil
}

func (pc *cursor) execute(ctx context.Context, s *session, query string) (int64, error) {
	defer TimeTrack(ctx, time.Now())

	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	Dbg(ctx, "Starting query: "+query)

	result, err := s.pool.ExecContext(pc.ctx, query)
	if err != nil {
		pc.err = err
		return -1, err
	}

	Dbg(ctx, "Done query: "+query)

	rows, err := result.RowsAffected()
	if err != nil {
		pc.err = err
		return -1, err
	}

	return rows, nil
}

func (pc *cursor) setAccessTime() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	pc.accessTime = time.Now()
}

func (pc *cursor) getAccessTime() time.Time {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	return pc.accessTime
}

type cursors struct {
	store map[string]*cursor
	mutex sync.Mutex
}

func (pc *cursors) set(cid string, c *cursor) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	pc.store[cid] = c
}

func (pc *cursors) get(cid string) (*cursor, error) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	c, present := pc.store[cid]
	if !present {
		return nil, errors.New(ERR_INVALID_CURSOR_ID)
	}

	if c.err != nil {
		return nil, c.err
	}

	return c, nil
}

func (pc *cursors) getKeys() []string {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	keys := make([]string, len(pc.store))

	i := 0
	for k := range pc.store {
		keys[i] = k
		i++
	}

	return keys
}

func (pc *cursors) clear(k string) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	c, present := pc.store[k]
	if present {
		if c.rows != nil {
			c.rows.Close()
		}
		delete(pc.store, k)
	}
}

func NewCursorStore() *cursors {
	store := make(map[string]*cursor)
	return &cursors{
		store: store,
	}
}

//==============================================================//
//          cursor structs and methods end
//==============================================================//

func NewQueryCursor(reqCtx context.Context) *cursor {
	c := createCursor(reqCtx)
	go cursorHandler(reqCtx, c)
	return c
}

func NewExecuteCursor(reqCtx context.Context) *cursor {
	c := createCursor(reqCtx)
	return c
}

func createCursor(reqCtx context.Context) *cursor {
	var c cursor
	ctx, cancel := context.WithCancel(context.Background())
	c.id = uniuri.New()
	c.in = make(chan *Req)
	c.out = make(chan *Res)
	c.accessTime = time.Now()
	c.ctx = ctx
	c.cancel = cancel

	return &c
}

//goroutine to handle a single cursor
func cursorHandler(reqCtx context.Context, c *cursor) {
	Dbg(reqCtx, fmt.Sprintf("Starting cursorHandler for %s\n", c.id))

loop:
	for {
		select {
		case req := <-c.in:
			ctx, cancel := context.WithCancel(reqCtx)
			go startTimer(ctx, c)

			res := handleCursorRequest(c, req)
			c.out <- res

			cancel()

			if res.code == ERROR || res.code == EOF {
				//Whatever the error we should exit
				Dbg(req.ctx, fmt.Sprintf("%s: Shutting down cursorHandler due to %s\n", c.id, res.code))
				break loop
			}

		case <-c.ctx.Done():
			Dbg(reqCtx, fmt.Sprintf("%s: Shutting down cursorHandler due to ctx.Done", c.id))
			break loop
		}
	}
}

func handleCursorRequest(c *cursor, req *Req) *Res {
	defer TimeTrack(req.ctx, time.Now())

	switch req.code {
	case CMD_FETCH_WS:
		return handle_ws(c, req)

	case CMD_FETCH:
		return handle_ajax(c, req)

	default:
		Dbg(req.ctx, fmt.Sprintf("%s: Invalid Command\n", c.id))
		return &Res{
			code: ERROR,
			data: errors.New(ERR_INVALID_CURSOR_CMD),
		}
	}
}

func handle_ws(c *cursor, req *Req) *Res {
	Dbg(req.ctx, fmt.Sprintf("%s: Handling CMD_FETCH\n", c.id))
	fetchReq, _ := req.data.(FetchReq)
	err := fetchRows_ws(req.ctx, c, fetchReq)
	if err != nil {
		Dbg(req.ctx, fmt.Sprintf("%s: %s\n", c.id, err.Error()))
		return &Res{
			code: ERROR,
			data: err,
		}
	}

	Dbg(req.ctx, fmt.Sprintf("%s: Done CMD_FETCH\n", c.id))

	return &Res{
		code: SUCCESS,
	}
}

func handle_ajax(c *cursor, req *Req) *Res {
	Dbg(req.ctx, fmt.Sprintf("%s: Handling CMD_FETCH\n", c.id))
	fetchReq, _ := req.data.(FetchReq)
	rows, err := fetchRows(req.ctx, c, fetchReq)
	if err != nil {
		Dbg(req.ctx, fmt.Sprintf("%s: %s\n", c.id, err.Error()))
		return &Res{
			code: ERROR,
			data: err,
		}
	}

	Dbg(req.ctx, fmt.Sprintf("%s: Done CMD_FETCH\n", c.id))

	var code string
	if len(*rows) < fetchReq.n {
		code = EOF
	} else {
		code = SUCCESS
	}

	return &Res{
		code: code,
		data: rows,
	}
}

type res struct {
	K []string `json:"k"`
}

func getExportFile(ctx context.Context) (string, *os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	t := time.Now()
	now := fmt.Sprintf("%d-%02d-%02dT%02d-%02d-%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())

	f := filepath.FromSlash(home + "/Downloads/" + "query-results-" + now + ".csv")
	csvFile, err := os.Create(f)

	if err != nil {
		Dbg(ctx, fmt.Sprintf("failed creating file: %s", err))
		return "", nil, err
	}

	return f, csvFile, nil
}

func fetchRows_ws(ctx context.Context, c *cursor, fetchReq FetchReq) error {
	if fetchReq.cid != c.id {
		return errors.New(ERR_INVALID_CURSOR_ID)
	}

	cols, err := c.rows.Columns()
	if err != nil {
		return err
	}

	//if results are to be exported, create the output file
	var csvWriter *csv.Writer
	var fileName string
	var csvFile *os.File

	if fetchReq.export {
		fileName, csvFile, err = getExportFile(ctx)

		if err != nil {
			Dbg(ctx, fmt.Sprintf("failed creating file: %s", err))
			return err
		}

		csvWriter = csv.NewWriter(csvFile)

		if err := csvWriter.Write(cols); err != nil {
			Dbg(ctx, fmt.Sprintf("error writing record to csv: %s", err))
		}

		defer func() {
			csvWriter.Flush()
			csvFile.Close()
		}()
	}

	n := 0
	ws := fetchReq.ws
	vals := make([]interface{}, len(cols))

	for c.rows.Next() {
		for i := range cols {
			vals[i] = &vals[i]
		}

		err = c.rows.Scan(vals...)
		// Now you can check each element of vals for nil-ness,
		if err != nil {
			return err
		}

		var r []string
		for i, c := range cols {
			r = append(r, c)
			var v string

			if vals[i] == nil {
				v = "NULL"
			} else {
				b, _ := vals[i].([]byte)
				v = string(b)
			}

			r = append(r, v)
		}

		err = processRow(ctx, c.id, r, (n + 1), ws, fileName, csvWriter, fetchReq.export)
		if err != nil {
			return err
		}

		n++
		if n == fetchReq.n {
			break
		}
	}

	if c.rows.Err() != nil {
		return c.rows.Err()
	}

	if n < 1000 && fetchReq.export {
		str, _ := json.Marshal(&res{K: []string{"current-row", strconv.Itoa(n)}})
		err := ws.WriteMessage(websocket.TextMessage, []byte(str))
		if err != nil {
			return err
		}
	}

	str, _ := json.Marshal(&res{K: []string{"eos"}})
	err = ws.WriteMessage(websocket.TextMessage, []byte(str))
	if err != nil {
		return err
	}

	return nil
}

func processRow(ctx context.Context, cursorId string, row []string, currRow int,
	ws *websocket.Conn, fileName string, csvWriter *csv.Writer, export bool) error {

	if export {
		var r []string
		for i := 1; i <= len(row); i += 2 {
			r = append(r, row[i])
		}

		if err := csvWriter.Write(r); err != nil {
			Dbg(ctx, fmt.Sprintf("error writing record to csv: %s", err))
			return err
		}

		if currRow == 1 {
			str, _ := json.Marshal(&res{K: []string{"header", cursorId, fileName}})
			err := ws.WriteMessage(websocket.TextMessage, []byte(str))
			if err != nil {
				return err
			}
		}

		if currRow%1000 == 0 {
			str, _ := json.Marshal(&res{K: []string{"current-row", strconv.Itoa(currRow)}})
			err := ws.WriteMessage(websocket.TextMessage, []byte(str))
			if err != nil {
				return err
			}
		}

		return nil
	}

	str, _ := json.Marshal(&res{K: row})
	err := ws.WriteMessage(websocket.TextMessage, []byte(str))
	if err != nil {
		return err
	}

	return nil
}

func fetchRows(ctx context.Context, c *cursor, fetchReq FetchReq) (*[][]string, error) {
	if fetchReq.cid != c.id {
		return nil, errors.New(ERR_INVALID_CURSOR_ID)
	}

	ws := fetchReq.ws

	cols, err := c.rows.Columns()
	if err != nil {
		return nil, err
	}

	vals := make([]interface{}, len(cols))
	var results [][]string

	n := 0
	for c.rows.Next() {
		for i := range cols {
			vals[i] = &vals[i]
		}

		err = c.rows.Scan(vals...)
		// Now you can check each element of vals for nil-ness,
		if err != nil {
			return nil, err
		}

		var r []string
		for i, c := range cols {
			r = append(r, c)
			var v string

			if vals[i] == nil {
				v = "NULL"
			} else {
				b, _ := vals[i].([]byte)
				v = string(b)
			}

			r = append(r, v)
		}

		if ws != nil {
			str, _ := json.Marshal(&res{K: r})
			err = ws.WriteMessage(websocket.TextMessage, []byte(str))
			if err != nil {
				return nil, err
			}
		}

		results = append(results, r)

		n++
		if n == fetchReq.n {
			break
		}
	}

	if c.rows.Err() != nil {
		return nil, c.rows.Err()
	}

	if ws != nil {
		str, _ := json.Marshal(&res{K: []string{"eos"}})
		err = ws.WriteMessage(websocket.TextMessage, []byte(str))
		if err != nil {
			return nil, err
		}
	}

	return &results, nil
}
