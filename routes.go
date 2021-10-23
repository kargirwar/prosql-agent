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
	"github.com/kargirwar/prosql-agent/utils"
)

type QueryParams struct {
	SessionId string
	CursorId  string
	Query     string
	NumOfRows int
	Export    bool
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
	defer utils.TimeTrack(r.Context(), time.Now())

	id, err := machineid.ProtectedID(APP_NAME)
	if err != nil {
		id = "not-available"
	}

	utils.Dbg(r.Context(), "device-id:"+id)

	utils.SendSuccess(r.Context(), w, struct {
		ID      string `json:"device-id"`
		Version string `json:"version"`
		OS      string `json:"os"`
	}{id, VERSION, OS}, false)
}

func ping(w http.ResponseWriter, r *http.Request) {
	defer utils.TimeTrack(r.Context(), time.Now())

	dsn, err := getDsn(r)
	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	var pool *sql.DB // Database connection pool.
	pool, err = sql.Open("mysql", dsn)
	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		utils.SendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}

	utils.SendSuccess(r.Context(), w, nil, false)
}

func login(w http.ResponseWriter, r *http.Request) {
	defer utils.TimeTrack(r.Context(), time.Now())

	dsn, err := getDsn(r)
	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	sid, err := NewSession(r.Context(), "mysql", dsn)
	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}

	utils.SendSuccess(r.Context(), w, struct {
		SessionId string `json:"session-id"`
	}{sid}, false)
}

func cancel(w http.ResponseWriter, r *http.Request) {
	ctx := utils.GetContext(r)
	defer utils.TimeTrack(ctx, time.Now())

	sid, cid, err := getCancelParams(r)

	if err != nil {
		utils.Dbg(ctx, fmt.Sprintf("%s", err.Error()))
		utils.SendError(ctx, w, err, ERR_INVALID_USER_INPUT)
		return
	}

	err = Cancel(ctx, sid, cid)
	if err != nil {
		utils.Dbg(ctx, fmt.Sprintf("%s", err.Error()))
		utils.SendError(ctx, w, err, ERR_INVALID_USER_INPUT)
		return
	}

	utils.SendSuccess(r.Context(), w, nil, false)
}

//execute query and return its cursor id for later use
func query(w http.ResponseWriter, r *http.Request) {
	defer utils.TimeTrack(r.Context(), time.Now())

	sid, query, err := getQueryParams(r)

	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	cid, err := Query(r.Context(), sid, query)

	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	utils.SendSuccess(r.Context(), w, struct {
		CursorId string `json:"cursor-id"`
	}{cid}, false)
}

//execute query and return results
func execute(w http.ResponseWriter, r *http.Request) {
	defer utils.TimeTrack(r.Context(), time.Now())

	sid, query, err := getQueryParams(r)

	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	cid, err := Execute(r.Context(), sid, query)

	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	utils.SendSuccess(r.Context(), w, struct {
		CursorId string `json:"cursor-id"`
	}{cid}, false)
}

func fetch(w http.ResponseWriter, r *http.Request) {
	defer utils.TimeTrack(r.Context(), time.Now())

	sid, cid, n, err := getFetchParams(r)

	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	rows, eof, err := Fetch(r.Context(), sid, cid, n)

	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_DB_ERROR)
		return
	}

	utils.SendSuccess(r.Context(), w, rows, eof)
}

func fetch_ws(w http.ResponseWriter, r *http.Request) {
	ctx := utils.GetContext(r)
	defer utils.TimeTrack(ctx, time.Now())

	ws, err := utils.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		utils.Dbg(ctx, fmt.Sprintf("%s", err.Error()))
		return
	}
	defer ws.Close()

	params, err := getFetchParams_ws(r)

	if err != nil {
		utils.SendError(r.Context(), w, err, ERR_INVALID_USER_INPUT)
		return
	}

	err = Fetch_ws(ctx, params.SessionId, params.CursorId, ws, params.NumOfRows, params.Export)

	if err != nil {
		utils.SendError_ws(ctx, ws, err, ERR_INVALID_USER_INPUT)
		return
	}
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

func getFetchParams_ws(r *http.Request) (*QueryParams, error) {
	var params QueryParams
	input := r.URL.Query()

	sid, present := input["session-id"]
	if !present || len(sid) == 0 {
		e := errors.New("Session ID not provided")
		return nil, e
	}

	params.SessionId = sid[0]

	cid, present := input["cursor-id"]
	if !present || len(sid) == 0 {
		e := errors.New("Cursor ID not provided")
		return nil, e
	}

	params.CursorId = cid[0]

	num, present := input["num-of-rows"]
	if !present || len(num) == 0 {
		e := errors.New("Number of rows not provided")
		return nil, e
	}

	n, err := strconv.Atoi(num[0])
	if err != nil {
		e := errors.New("Number of rows must be integer")
		return nil, e
	}

	params.NumOfRows = n
	//whether to export to file or not
	_, present = input["export"]
	if present {
		params.Export = true
	}

	return &params, nil
}

func getCancelParams(r *http.Request) (string, string, error) {
	params := r.URL.Query()

	sid, present := params["session-id"]
	if !present || len(sid) == 0 {
		e := errors.New("Session ID not provided")
		return "", "", e
	}

	cid, present := params["cursor-id"]
	if !present || len(cid) == 0 {
		e := errors.New("Cursor ID not provided")
		return "", "", e
	}

	return sid[0], cid[0], nil
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
