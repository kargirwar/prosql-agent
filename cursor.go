package main

import (
	"fmt"
	"log"
	"time"

	"context"
	"database/sql"
	"errors"
	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
	"sync"
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
	c.cancel = cancel

	return &c, nil
}

//goroutine to handle a single cursor
func cursorHandler(c *cursor) {
	log.Printf("Starting cursorHandler for %s\n", c.id)
	defer c.rows.Close()

	for {
		select {
		case req := <-c.in:
			c.accessTime = time.Now()
			res := handleCursorRequest(c, req)
			c.out <- res
			if res.code == ERROR || res.code == EOF || res.code == CLEANUP_DONE {
				//Whatever the error we should exit
				log.Printf("Shutting down cursorHandler for %s\n", c.id)
				break
			}
		}
	}
}

func handleCursorRequest(c *cursor, req *Req) *Res {
	switch req.code {
	case CMD_FETCH:
		log.Printf("Handling CMD_FETCH")
		fetchReq, _ := req.data.(FetchReq)
		rows, err := fetchRows(c, fetchReq)
		if err != nil {
			return &Res{
				code: ERROR,
				data: err,
			}
		}

		log.Printf("Done CMD_FETCH")

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

	case CMD_CLEANUP:
		return &Res{
			code: CLEANUP_DONE,
		}

	default:
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

	var allrows [][]string

	columnNames, err := c.rows.Columns()
	if err != nil {
		return nil, err
	}

	rc := NewStringStringScan(columnNames)

	n := 0
	for c.rows.Next() {
		err := rc.Update(c.rows)
		if err != nil {
			return nil, err
		}

		cv := rc.Get()
		var m = make([]string, len(cv))
		copy(m, cv)
		allrows = append(allrows, m)

		n++
		if n == fetchReq.n {
			break
		}
	}

	if c.rows.Err() != nil {
		return nil, c.rows.Err()
	}

	return &allrows, nil
}

/**
  using a string slice
*/
type stringStringScan struct {
	// cp are the column pointers
	cp []interface{}
	// row contains the final result
	row      []string
	colCount int
	colNames []string
}

func NewStringStringScan(columnNames []string) *stringStringScan {
	lenCN := len(columnNames)
	s := &stringStringScan{
		cp:       make([]interface{}, lenCN),
		row:      make([]string, lenCN*2),
		colCount: lenCN,
		colNames: columnNames,
	}
	j := 0
	for i := 0; i < lenCN; i++ {
		s.cp[i] = new(sql.RawBytes)
		s.row[j] = s.colNames[i]
		j = j + 2
	}
	return s
}

func (s *stringStringScan) Update(rows *sql.Rows) error {
	if err := rows.Scan(s.cp...); err != nil {
		return err
	}
	j := 0
	for i := 0; i < s.colCount; i++ {
		if rb, ok := s.cp[i].(*sql.RawBytes); ok {
			s.row[j+1] = string(*rb)
			*rb = nil // reset pointer to discard current value to avoid a bug
		} else {
			return fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, s.colNames[i])
		}
		j = j + 2
	}
	return nil
}

func (s *stringStringScan) Get() []string {
	return s.row
}
