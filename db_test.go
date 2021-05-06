package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestExecute(t *testing.T) {
	var pool *sql.DB
	pool, err := sql.Open("mysql", "server:dev-server@tcp(127.0.0.1:3306)/test-generico")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		t.Errorf("%s\n", err.Error())
	}

	i := 0
	for {
		s := time.Now()

		rows, err := pool.QueryContext(ctx, "select * from users")
		if err != nil {
			t.Errorf("%s\n", err.Error())
		}

		e := time.Now()

		t.Log(e.Sub(s))

		rows.Close()

		i++
		if i == 10 {
			break
		}
	}
}

func TestSeq1(t *testing.T) {
	var pool *sql.DB
	pool, err := sql.Open("mysql", "server:dev-server@tcp(127.0.0.1:3306)/")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		t.Errorf("%s\n", err.Error())
	}

	//
	s := time.Now()

	q := "use `test-generico`"
	rows, err := pool.QueryContext(ctx, q)
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	e := time.Now()

	t.Logf("%s took %s", q, e.Sub(s))

	rows.Close()

	//
	s = time.Now()

	q = "select * from users"
	rows, err = pool.QueryContext(ctx, q)
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	e = time.Now()

	t.Logf("%s took %s", q, e.Sub(s))

	rows.Close()
}
