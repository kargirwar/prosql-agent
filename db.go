package main

import (
	"errors"
	"net/http"
	"time"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

type Session struct {
	pool       *sql.DB
	accessTime time.Time
}

var sessions map[string]*Session

func init() {
	sessions = make(map[string]*Session)
}

//At this time planning to have one session pool per tab
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
