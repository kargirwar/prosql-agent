package main

import (
    "os"
	"fmt"
	"net/http"
    "time"

	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
    "github.com/gorilla/sessions"
)

var pool *sql.DB // Database connection pool.
var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

func ping(w http.ResponseWriter, r *http.Request) {
    dsn := "server:dev-server@tcp(127.0.0.1:3306)/"
    pool, err := sql.Open("mysql", dsn)
    if err != nil {
        // This will not be a connection error, but a DSN parse error or
        // another initialization error.
        fmt.Fprintf(w, `{"status":"error", "msg": "invalid dsn"}`)
        return
    }
    defer pool.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    if err := pool.PingContext(ctx); err != nil {
        fmt.Fprintf(w, `{"status":"error", "msg": "unable to connect"}`)
        return
    }

    fmt.Fprintf(w, `{"status":"ok"}`)
}

func login(w http.ResponseWriter, r *http.Request) {
    // Get a session. We're ignoring the error resulted from decoding an
    // existing session: Get() always returns a session, even if empty.
    session, _ := store.Get(r, "session-name")
    // Set some session values.
    session.Values["foo"] = "bar"
    session.Values[42] = 43
    // Save it before we write to the response/return from the handler.
    err := session.Save(r, w)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

//func checkSession(w http.ResponseWriter, r *http.Request) {
