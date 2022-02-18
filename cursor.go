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
	"runtime"
	"strconv"
	"time"

	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
	"github.com/kargirwar/prosql-agent/constants"
	"github.com/kargirwar/prosql-agent/utils"
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
	execute    bool
}

func (pc *cursor) start(ctx context.Context, db *sql.DB) error {
	defer utils.TimeTrack(ctx, time.Now())

	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	if pc.rows != nil {
		utils.Dbg(ctx, "Continue with current query")
		return nil
	}

	utils.Dbg(ctx, "Starting query: "+pc.query)

	rows, err := db.QueryContext(pc.ctx, pc.query)
	if err != nil {
		pc.err = err
		return err
	}

	utils.Dbg(ctx, "Done query: "+pc.query)

	pc.rows = rows
	return nil
}

func (pc *cursor) exec(ctx context.Context, db *sql.DB) (int64, error) {
	defer utils.TimeTrack(ctx, time.Now())

	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	utils.Dbg(ctx, "Starting query: "+pc.query)

	result, err := db.ExecContext(pc.ctx, pc.query)
	if err != nil {
		pc.err = err
		return -1, err
	}

	utils.Dbg(ctx, "Done query: "+pc.query)

	rows, err := result.RowsAffected()
	if err != nil {
		pc.err = err
		return -1, err
	}

	return rows, nil
}

func (pc *cursor) getId() string {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	return pc.id
}

func (pc *cursor) isExecute() bool {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	return pc.execute
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
		return nil, errors.New(constants.ERR_INVALID_CURSOR_ID)
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

func NewQueryCursor(reqCtx context.Context, query string) *cursor {
	c := createCursor(reqCtx, query, false)
	go cursorHandler(reqCtx, c)
	return c
}

func NewExecuteCursor(reqCtx context.Context, query string) *cursor {
	c := createCursor(reqCtx, query, true)
	return c
}

func createCursor(reqCtx context.Context, query string, isExecute bool) *cursor {
	var c cursor
	ctx, cancel := context.WithCancel(context.Background())
	c.id = uniuri.New()
	c.in = make(chan *Req)
	c.out = make(chan *Res)
	c.accessTime = time.Now()
	c.ctx = ctx
	c.cancel = cancel
	c.query = query
	c.execute = isExecute

	return &c
}

//goroutine to handle a single cursor
func cursorHandler(reqCtx context.Context, c *cursor) {
	utils.Dbg(reqCtx, fmt.Sprintf("Starting cursorHandler for %s\n", c.id))

loop:
	for {
		select {
		case req := <-c.in:
			ctx, cancel := context.WithCancel(req.ctx)
			go startTimer(ctx, c)

			res := handleCursorRequest(c, req)
			c.out <- res

			cancel()

			if res.code == constants.ERROR || res.code == constants.EOF {
				//Whatever the error we should exit
				utils.Dbg(req.ctx, fmt.Sprintf("%s: Shutting down cursorHandler due to %s\n", c.id, res.code))
				break loop
			}

		case <-c.ctx.Done():
			utils.Dbg(reqCtx, fmt.Sprintf("%s: Shutting down cursorHandler due to ctx.Done", c.id))
			break loop
		}
	}
}

func handleCursorRequest(c *cursor, req *Req) *Res {
	defer utils.TimeTrack(req.ctx, time.Now())

	switch req.code {
	case constants.CMD_FETCH_WS:
		return handle_ws(c, req)

	case constants.CMD_FETCH:
		return handle_ajax(c, req)

	default:
		utils.Dbg(req.ctx, fmt.Sprintf("%s: Invalid Command\n", c.id))
		return &Res{
			code: constants.ERROR,
			data: errors.New(constants.ERR_INVALID_CURSOR_CMD),
		}
	}
}

func handle_ws(c *cursor, req *Req) *Res {
	utils.Dbg(req.ctx, fmt.Sprintf("%s: Handling constants.CMD_FETCH_WS\n", c.id))
	fetchReq, _ := req.data.(FetchReq)
	err := fetchRows_ws(req.ctx, c, fetchReq)
	if err != nil {
		utils.Dbg(req.ctx, fmt.Sprintf("%s: %s\n", c.id, err.Error()))
		return &Res{
			code: constants.ERROR,
			data: err,
		}
	}

	utils.Dbg(req.ctx, fmt.Sprintf("%s: Done constants.CMD_FETCH\n", c.id))

	return &Res{
		code: constants.SUCCESS,
	}
}

func handle_ajax(c *cursor, req *Req) *Res {
	utils.Dbg(req.ctx, fmt.Sprintf("%s: Handling constants.CMD_FETCH\n", c.id))
	fetchReq, _ := req.data.(FetchReq)
	rows, err := fetchRows(req.ctx, c, fetchReq)
	if err != nil {
		utils.Dbg(req.ctx, fmt.Sprintf("%s: %s\n", c.id, err.Error()))
		return &Res{
			code: constants.ERROR,
			data: err,
		}
	}

	utils.Dbg(req.ctx, fmt.Sprintf("%s: Done constants.CMD_FETCH\n", c.id))

	var code string
	if len(*rows) < fetchReq.n {
		code = constants.EOF
	} else {
		code = constants.SUCCESS
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
	home, err := getHomeDir()
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
		utils.Dbg(ctx, fmt.Sprintf("failed creating file: %s", err))
		return "", nil, err
	}

	return f, csvFile, nil
}

func getHomeDir() (string, error) {
	if runtime.GOOS == "windows" {
		return os.Getenv("USER_HOME_DIR"), nil
	}
	return os.UserHomeDir()
}

func fetchRows_ws(ctx context.Context, c *cursor, fetchReq FetchReq) error {
	if fetchReq.cid != c.id {
		return errors.New(constants.ERR_INVALID_CURSOR_ID)
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
			utils.Dbg(ctx, fmt.Sprintf("failed creating file: %s", err))
			return err
		}

		csvWriter = csv.NewWriter(csvFile)

		if err := csvWriter.Write(cols); err != nil {
			utils.Dbg(ctx, fmt.Sprintf("error writing record to csv: %s", err))
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

		//time.Sleep(500 * time.Millisecond)
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
			utils.Dbg(ctx, fmt.Sprintf("error writing record to csv: %s", err))
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
		return nil, errors.New(constants.ERR_INVALID_CURSOR_ID)
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
