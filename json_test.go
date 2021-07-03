/* Copyright (C) 2021 Pankaj Kargirwar <kargirwar@gmail.com>

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
