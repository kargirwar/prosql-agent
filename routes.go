package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"context"
	"database/sql"
	"net/url"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
)

type Response struct {
	Status    string      `json:"status"`
	Msg       string      `json:"msg"`
	ErrorCode string      `json:"error-code"`
	Data      interface{} `json:"data"`
	Eof       bool        `json:"eof"`
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

	host, present := params["host"]
	if !present || len(host) == 0 {
		e := errors.New("Host not provided")
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

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user[0], pass[0], host[0], port[0], dbName), nil
}

func ping(w http.ResponseWriter, r *http.Request) {
	defer TimeTrack(r.Context(), time.Now())

	dsn, err := getDsn(r)
	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	var pool *sql.DB // Database connection pool.
	pool, err = sql.Open("mysql", dsn)
	if err != nil {
		sendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		sendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}

	sendSuccess(r.Context(), w, nil, false)
}

func login(w http.ResponseWriter, r *http.Request) {
	defer TimeTrack(r.Context(), time.Now())

	dsn, err := getDsn(r)
	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	sid, err := NewSession(r.Context(), "mysql", dsn)
	if err != nil {
		sendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}

	sendSuccess(r.Context(), w, struct {
		SessionId string `json:"session-id"`
	}{sid}, false)
}

func setDb(w http.ResponseWriter, r *http.Request) {
	sid, db, err := getSetDbParams(r)

	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	err = SetDb(sid, db)

	if err != nil {
		sendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}

	sendSuccess(r.Context(), w, nil, false)
}

func execute(w http.ResponseWriter, r *http.Request) {
	defer TimeTrack(r.Context(), time.Now())

	sid, query, err := getExecuteParams(r)

	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	cid, err := Execute(r.Context(), sid, query)

	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	sendSuccess(r.Context(), w, struct {
		CursorId string `json:"cursor-id"`
	}{cid}, false)
}

func fetch(w http.ResponseWriter, r *http.Request) {
	defer TimeTrack(r.Context(), time.Now())

	sid, cid, n, err := getFetchParams(r)

	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	rows, eof, err := Fetch(r.Context(), sid, cid, n)

	if err != nil {
		sendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}

	sendSuccess(r.Context(), w, rows, eof)
}

func getFetchParams(r *http.Request) (string, string, int, error) {
	params := r.URL.Query()

	sid, present := params["session-id"]
	if !present || len(sid) == 0 {
		e := errors.New("Session ID not provided")
		return "", "", -1, e
	}

	cid, present := params["cursor-id"]
	if !present || len(cid) == 0 {
		e := errors.New("Cursor ID not provided")
		return "", "", -1, e
	}

	num, present := params["num-of-rows"]
	if !present || len(num) == 0 {
		e := errors.New("Number of rows not provided")
		return "", "", -1, e
	}

	n, err := strconv.Atoi(num[0])
	if err != nil {
		e := errors.New("Number of rows must be integer")
		return "", "", -1, e
	}
	return sid[0], cid[0], n, nil
}

func getSetDbParams(r *http.Request) (string, string, error) {
	params := r.URL.Query()

	sid, present := params["session-id"]
	if !present || len(sid) == 0 {
		e := errors.New("Session ID not provided")
		return "", "", e
	}

	db, present := params["db"]
	if !present || len(db) == 0 {
		e := errors.New("Db not provided")
		return "", "", e
	}

	return sid[0], db[0], nil
}

func getExecuteParams(r *http.Request) (string, string, error) {
	params := r.URL.Query()

	sid, present := params["session-id"]
	if !present || len(sid) == 0 {
		e := errors.New("Session ID not provided")
		return "", "", e
	}

	query, present := params["query"]
	if !present || len(query) == 0 {
		e := errors.New("Query not provided")
		return "", "", e
	}

	q, err := url.QueryUnescape(query[0])
	if err != nil {
		return "", "", err
	}

	return sid[0], q, nil
}
