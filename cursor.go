package main

import (
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
	accessTime time.Time
	cancel     context.CancelFunc
	ctx        context.Context
	mutex      sync.Mutex
	err        error
}

func (c *cursor) execute(s *session, query string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	rows, err := s.pool.QueryContext(c.ctx, query)
	if err != nil {
		c.err = err
		return err
	}

	c.rows = rows
	return nil
}

func (c *cursor) setAccessTime() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.accessTime = time.Now()
}

func (c *cursor) getAccessTime() time.Time {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.accessTime
}

func (c *cursor) fetch(numRows int) (*[][]string, bool, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	cols, err := c.rows.Columns()
	if err != nil {
		return nil, false, err
	}

	vals := make([]interface{}, len(cols))
	var results [][]string

	n := 0
	for c.rows.Next() {
		for i := range cols {
			vals[i] = &vals[i]
		}

		err = c.rows.Scan(vals...)
		if err != nil {
			return nil, false, err
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
		if n == numRows {
			break
		}
	}

	if c.rows.Err() != nil {
		return nil, false, c.rows.Err()
	}

	var eof bool
	if len(results) < numRows {
		eof = true
	}

	return &results, eof, nil
}

//==============================================================//
//          cursor structs and methods end
//==============================================================//

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

func NewCursor(pool *sql.DB, db string) (*cursor, error) {
	var c cursor
	ctx, cancel := context.WithCancel(context.Background())
	c.id = uniuri.New()
	c.accessTime = time.Now()
	c.ctx = ctx
	c.cancel = cancel

	_, err := pool.QueryContext(c.ctx, "use "+"`"+db+"`")
	if err != nil {
		return nil, err
	}

	return &c, nil
}
