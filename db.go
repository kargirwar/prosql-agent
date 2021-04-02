package main

import (
	"encoding/json"
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

type session struct {
	pool      *sql.DB
	ctx       context.Context
	startTime time.Time
}

type Response struct {
	Status string      `json:"status"`
	Msg    string      `json:"msg"`
	Data   interface{} `json:"data"`
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
	if !present || len(db) == 0 {
		dbName = ""
	} else {
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

	fmt.Fprintf(w, `{"status":"ok"}`)
}

func login(w http.ResponseWriter, r *http.Request) {
	//var array = []int{1, 2, 3}
	sessionId := uniuri.New()
	res := &Response{
		Status: "ok",
		Data: struct {
			Session string
		}{sessionId},
	}
	str, _ := json.Marshal(res)

	fmt.Fprintf(w, string(str))
}

func check(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `{"status":"ok"}`)
}
