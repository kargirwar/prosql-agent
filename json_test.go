package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"
)

func TestJson(t *testing.T) {
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

	data := fetchAll(t, ctx, pool)
	fmt.Println(data)

	res := &Response{
		Data: data,
	}

	str, err := json.Marshal(res)
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	fmt.Println(string(str))
}

func fetchAll(t *testing.T, ctx context.Context, pool *sql.DB) [][]string {
	rows, err := pool.QueryContext(ctx, "select description from vouchers where id = 606")
	if err != nil {
		t.Fatalf("%s\n", err.Error())
	}

	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("%s\n", err.Error())
	}

	vals := make([]interface{}, len(cols))
	var results [][]string

	for rows.Next() {
		for i := range cols {
			vals[i] = &vals[i]
		}

		err = rows.Scan(vals...)
		if err != nil {
			t.Fatalf("%s\n", err.Error())
		}

		var r []string
		for i, c := range cols {
			r = append(r, c)
			var v string

			if vals[i] == nil {
				v = "NULL"
			} else {
				b, _ := vals[i].([]byte)
				v = string(b)
			}

			fmt.Printf("byte len %d\n", len(v))
			fmt.Printf("rune len %d\n", utf8.RuneCountInString(v))

			r = append(r, v)
		}

		results = append(results, r)
	}

	if rows.Err() != nil {
		t.Fatalf("%s\n", err.Error())
	}

	return results
}
