package main

import (
	"log"
	"time"

	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
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
}

type cursors struct {
	store  map[string]*cursor
	cmutex sync.Mutex
}

func (pc *cursors) set(cid string, c *cursor) {
	pc.cmutex.Lock()
	defer pc.cmutex.Unlock()

	pc.store[cid] = c
}

func (pc *cursors) get(cid string) (*cursor, error) {
	pc.cmutex.Lock()
	defer pc.cmutex.Unlock()

	c, present := pc.store[cid]
	if !present {
		return nil, errors.New(ERR_INVALID_CURSOR_ID)
	}

	return c, nil
}

func (pc *cursors) getKeys() []string {
	pc.cmutex.Lock()
	defer pc.cmutex.Unlock()

	keys := make([]string, len(pc.store))

	i := 0
	for k := range pc.store {
		keys[i] = k
		i++
	}

	return keys
}

func (pc *cursors) clear(k string) {
	pc.cmutex.Lock()
	defer pc.cmutex.Unlock()

	_, present := pc.store[k]
	if present {
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

func NewCursor(s *session, query string) (*cursor, error) {
	c, err := createCursor(s, query)

	if err != nil {
		return nil, err
	}

	go cursorHandler(c)
	return c, nil
}

func createCursor(s *session, query string) (*cursor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	rows, err := s.pool.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	var c cursor
	c.id = uniuri.New()
	c.in = make(chan *Req)
	c.out = make(chan *Res)
	c.rows = rows
	c.accessTime = time.Now()
	c.ctx = ctx
	c.cancel = cancel

	return &c, nil
}

//goroutine to handle a single cursor
func cursorHandler(c *cursor) {
	log.Printf("Starting cursorHandler for %s\n", c.id)
	defer c.rows.Close()

loop:
	for {
		select {
		case req := <-c.in:
			c.accessTime = time.Now()
			res := handleCursorRequest(c, req)
			c.out <- res
			if res.code == ERROR || res.code == EOF {
				//Whatever the error we should exit
				log.Printf("%s: Shutting down cursorHandler due to %s\n", c.id, res.code)
				break loop
			}

		case <-c.ctx.Done():
			log.Printf("%s: Shutting down cursorHandler due to ctx.Done", c.id)
			break loop
		}
	}
}

func handleCursorRequest(c *cursor, req *Req) *Res {
	switch req.code {
	case CMD_FETCH:
		log.Printf("%s: Handling CMD_FETCH\n", c.id)
		fetchReq, _ := req.data.(FetchReq)
		rows, err := fetchRows(c, fetchReq)
		if err != nil {
			return &Res{
				code: ERROR,
				data: err,
			}
		}

		log.Printf("%s: Done CMD_FETCH\n", c.id)

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

	default:
		log.Printf("%s: Invalid Command\n", c.id)
		return &Res{
			code: ERROR,
			data: errors.New(ERR_INVALID_CURSOR_CMD),
		}
	}
}

func fetchRows(c *cursor, fetchReq FetchReq) (*[][]string, error) {
	if fetchReq.cid != c.id {
		return nil, errors.New(ERR_INVALID_CURSOR_ID)
	}

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

		results = append(results, r)

		n++
		if n == fetchReq.n {
			break
		}
	}

	if c.rows.Err() != nil {
		return nil, c.rows.Err()
	}

	return &results, nil
}
