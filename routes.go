package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"context"
	"database/sql"
	"net/url"
	"strconv"

	"github.com/denisbrodbeck/machineid"
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

func about(w http.ResponseWriter, r *http.Request) {
	defer TimeTrack(r.Context(), time.Now())

	id, err := machineid.ProtectedID(APP_NAME)
	if err != nil {
		id = "not-available"
	}

	Dbg(r.Context(), "device-id:"+id)

	sendSuccess(r.Context(), w, struct {
		ID      string `json:"device-id"`
		Version string `json:"cursor-id"`
	}{id, VERSION}, false)
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

//execute query and send results over ws
func query_ws(w http.ResponseWriter, r *http.Request) {
	ctx := getContext(r)
	defer TimeTrack(ctx, time.Now())
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		Dbg(ctx, fmt.Sprintf("%s", err.Error()))
		return
	}
	defer ws.Close()

	sid, query, err := getQueryParams(r)

	if err != nil {
		sendError_ws(ctx, ws, err, ERR_INVALID_USER_INPUT)
		return
	}

	cid, err := Query(ctx, sid, query)
	err = Fetch_ws(ctx, sid, cid, ws, BATCH_SIZE)

	if err != nil {
		sendError_ws(ctx, ws, err, ERR_INVALID_USER_INPUT)
		return
	}
}

//execute query and return its cursor id for later use
func query(w http.ResponseWriter, r *http.Request) {
	defer TimeTrack(r.Context(), time.Now())

	sid, query, err := getQueryParams(r)

	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	cid, err := Query(r.Context(), sid, query)

	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	sendSuccess(r.Context(), w, struct {
		CursorId string `json:"cursor-id"`
	}{cid}, false)
}

//execute query and return results
func execute(w http.ResponseWriter, r *http.Request) {
	defer TimeTrack(r.Context(), time.Now())

	sid, query, err := getQueryParams(r)

	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	n, err := Execute(r.Context(), sid, query)

	if err != nil {
		sendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	sendSuccess(r.Context(), w, struct {
		RowsAffected int64 `json:"rows-affected"`
	}{n}, false)
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

func getQueryParams(r *http.Request) (string, string, error) {
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
