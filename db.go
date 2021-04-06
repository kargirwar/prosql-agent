package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"context"
	"database/sql"
	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
)

type Response struct {
	Status string      `json:"status"`
	Msg    string      `json:"msg"`
	Data   interface{} `json:"data"`
}

type Session struct {
	pool      *sql.DB
	accessTime time.Time
}

var sessions map[string]*Session

func init() {
    sessions = make(map[string]*Session)
}

func getDsn(r *http.Request) (dsn string, err error) {
	params := r.URL.Query()

	user, present := params["user"]
	if !present || len(user) == 0 {
		e := errors.New("User not provided")
		return "", e
	}

	pass, present := params["pass"]
	if !present || len(pass) == 0 {
		e := errors.New("User not provided")
		return "", e
	}

	ip, present := params["ip"]
	if !present || len(ip) == 0 {
		e := errors.New("IP not provided")
		return "", e
	}

	port, present := params["port"]
	if !present || len(port) == 0 {
		e := errors.New("Port not provided")
		return "", e
	}

	var dbName string
	db, present := params["db"]
	if present && len(db) == 1 {
		dbName = db[0]
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user[0], pass[0], ip[0], port[0], dbName), nil
}

func ping(w http.ResponseWriter, r *http.Request) {
	dsn, err := getDsn(r)
	if err != nil {
		sendError(w, err)
		return
	}

	log.Println(dsn)

	var pool *sql.DB // Database connection pool.
	pool, err = sql.Open("mysql", dsn)
	if err != nil {
		sendError(w, err)
		return
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		sendError(w, err)
		return
	}

	sendSuccess(w, nil)
}

func login(w http.ResponseWriter, r *http.Request) {
    dsn, err := getDsn(r)
    if err != nil {
        sendError(w, err)
        return
    }

    sessionId := uniuri.New()
    err = createSessionPool(dsn, sessionId)
    if err != nil {
        sendError(w, err)
        return
    }

    sendSuccess(w, struct {
        Session string `json:"session-id"`
    }{sessionId})
}

func createSessionPool(dsn string, sessionId string) (err error) {
    var s Session
    pool, err := sql.Open("mysql", dsn)

	if err != nil {
		return err
	}

    s.pool = pool
    s.accessTime = time.Now()
    sessions[sessionId] = &s

    return nil
}

func check(w http.ResponseWriter, r *http.Request) {
    session, err := getSession(r)

    if err != nil {
        sendError(w, err)
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    rows, err := session.pool.QueryContext(ctx, "select * from users")
    if err != nil {
        sendError(w, err)
        return
    }

    defer rows.Close()
	var allrows [][]string

    columnNames, err := rows.Columns()
    if err != nil {
        sendError(w, err)
        return
    }

    rc := NewStringStringScan(columnNames)

    for rows.Next() {
        err := rc.Update(rows)
        if err != nil {
            sendError(w, err)
            return
        }

        cv := rc.Get()
        var m = make([]string, len(cv))
        copy(m, cv)
        allrows = append(allrows, m)
    }

    sendSuccess(w, allrows)
}

func getSession(r *http.Request) (s *Session, err error) {
    params := r.URL.Query()

    sid, present := params["session-id"]
    if !present || len(sid) == 0 {
        e := errors.New("Session ID not provided")
        return nil, e
    }

    sessionId := sid[0]
    session, present := sessions[sessionId]

    if !present {
        e := errors.New("Invalid Session ID")
        return nil, e
    }

    session.accessTime = time.Now()
    return session, nil
}

/**
  using a map
*/
type mapStringScan struct {
	// cp are the column pointers
	cp []interface{}
	// row contains the final result
	row      map[string]string
	colCount int
	colNames []string
}

func NewMapStringScan(columnNames []string) *mapStringScan {
	lenCN := len(columnNames)
	s := &mapStringScan{
		cp:       make([]interface{}, lenCN),
		row:      make(map[string]string, lenCN),
		colCount: lenCN,
		colNames: columnNames,
	}
	for i := 0; i < lenCN; i++ {
		s.cp[i] = new(sql.RawBytes)
	}
	return s
}

func (s *mapStringScan) Update(rows *sql.Rows) error {
	if err := rows.Scan(s.cp...); err != nil {
		return err
	}

	for i := 0; i < s.colCount; i++ {
		if rb, ok := s.cp[i].(*sql.RawBytes); ok {
			s.row[s.colNames[i]] = string(*rb)
			*rb = nil // reset pointer to discard current value to avoid a bug
		} else {
			return fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, s.colNames[i])
		}
	}
	return nil
}

func (s *mapStringScan) Get() map[string]string {
	return s.row
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
