package main

import (
	"fmt"
	"net/http"
    "time"

	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

var pool *sql.DB // Database connection pool.

func ping(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Access-Control-Allow-Origin", "https://dev.prosql.io")
    w.Header().Set("Content-Type", "application/json")

    if r.Method == http.MethodOptions {
        return
    }

    dsn := "server:dev-server@tcp(127.0.0.1:3306)/test-generico"
    pool, err := sql.Open("mysql", dsn)
    if err != nil {
        // This will not be a connection error, but a DSN parse error or
        // another initialization error.
        fmt.Fprintf(w, `{"status":"error", "msg": "invalid dsn"}`)
    }
    defer pool.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    if err := pool.PingContext(ctx); err != nil {
        fmt.Fprintf(w, `{"status":"error", "msg": "unable to connect"}`)
    }

    fmt.Fprintf(w, `{"status":"ok"}`)
}
