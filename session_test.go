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
	"context"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

const N = 1000

func TestNewSession(t *testing.T) {
	ctx := context.Background()
	sid, err := NewSession(ctx, "mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)
}

func TestExecute(t *testing.T) {
	ctx := context.Background()
	sid, err := NewSession(ctx, "mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	//_, err = Execute(sid, "select * from users")
	//_, err = Execute(ctx, sid, "select sleep (10)")
	n, err := Execute(ctx, sid, "update users set name = 'PK' where id = 1")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}
	t.Logf("%d rows affected", n)
}

func TestFetch(t *testing.T) {
	ctx := context.Background()
	sid, err := NewSession(ctx, "mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	//cid, err := Query(ctx, sid, "select id from `bills-1` limit 10000")
	cid, err := Query(ctx, sid, "select * from `bills-1` order by `serial` desc")
	//cid, err := Query(ctx, sid, "select * from `billss-1`")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	for {
		rows, eof, err := Fetch(ctx, sid, cid, N)
		if err != nil {
			t.Errorf("%s\n", err.Error())
			break
		}

		t.Logf("Received %d rows", len(*rows))
		if eof == true {
			break
		}
	}
}

func TestCancel(t *testing.T) {
	ctx := context.Background()
	sid, err := NewSession(ctx, "mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	cid, err := Query(ctx, sid, "select sleep(10)")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	//test cancellation
	t1 := time.NewTimer(2 * time.Second)
	go func() {
		<-t1.C
		err := Cancel(ctx, sid, cid)
		if err != nil {
			t.Errorf("%s\n", err.Error())
		}
	}()

	for {
		rows, eof, err := Fetch(ctx, sid, cid, N)
		if err == context.Canceled {
			//expected
			t.Logf("%s: Breaking due to context canceled", sid)
			break
		}

		if err != nil {
			t.Errorf("%s: %s", sid, err.Error())
			break
		}

		t.Logf("Received %d rows", len(*rows))
		if eof == true {
			break
		}
	}
}

const TEST_UNUSED_SESSION = 0
const TEST_FETCH = 1
const TEST_CANCEL = 2
const STRESS_TICKER_INTERVAL = 1 * time.Second
const STRESS_STOP_TIME = 300 * time.Second
const CANCEL_AFTER = 2 * time.Second

//create several sessions. Leave some sessions unused
//Fetch data from some sessions
//Cancel a few sessions
//Repeat several times for a long period of time.
func TestStress(t *testing.T) {
	queries := []string{
		"select * from users limit " + strconv.Itoa(N),
		"select * from invoices limit " + strconv.Itoa(N*100),
		"select * from `bills-1` limit " + strconv.Itoa(N/100),
		"select * from `short-book-1` limit " + strconv.Itoa(N-1),
		"select sleep(10)",
		"select sleep(1)",
	}

	ticker := time.NewTicker(STRESS_TICKER_INTERVAL)
	stopTimer := time.NewTimer(STRESS_STOP_TIME)
loop:
	for {
		select {
		case <-stopTimer.C:
			t.Logf("Done testing")
			break loop

		case <-ticker.C:
			startTest(t, queries)
		}
	}
}

func startTest(t *testing.T, queries []string) {
	t.Logf("Starting new test")
	types := []int{TEST_UNUSED_SESSION, TEST_FETCH, TEST_CANCEL}
	n := rand.Intn(len(types))

	switch n {
	case TEST_UNUSED_SESSION:
		testUnused(t, queries)

	case TEST_FETCH:
		testFetch(t, queries)

	case TEST_CANCEL:
		testCancel(t, queries)
	}
}

func testUnused(t *testing.T, queries []string) {
	ctx := context.Background()
	t.Logf("testUnused")
	q := queries[rand.Intn(len(queries))]
	sid, err := NewSession(ctx, "mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
		return
	}

	t.Log(sid)

	_, err = Query(ctx, sid, q)
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}
}

func testFetch(t *testing.T, queries []string) {
	ctx := context.Background()
	t.Logf("testFetch")
	q := queries[rand.Intn(len(queries))]

	sid, err := NewSession(ctx, "mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
		return
	}

	t.Log(sid)

	cid, err := Query(ctx, sid, q)
	if err != nil {
		t.Errorf("%s\n", err.Error())
		return
	}

	for {
		rows, eof, err := Fetch(ctx, sid, cid, N)
		if err != nil {
			t.Errorf("%s: %s", sid, err.Error())
			break
		}

		t.Logf("%s: Received %d rows", sid, len(*rows))
		if eof == true {
			t.Logf("%s: Breaking due to constants.EOF", sid)
			break
		}
	}
}

func testCancel(t *testing.T, queries []string) {
	ctx := context.Background()
	t.Logf("testCancel")
	q := queries[rand.Intn(len(queries))]

	sid, err := NewSession(ctx, "mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
		return
	}

	t.Log(sid)

	cid, err := Query(ctx, sid, q)
	if err != nil {
		t.Errorf("%s: %s\n", sid, err.Error())
		return
	}

	//test cancellation
	t1 := time.NewTimer(CANCEL_AFTER)
	go func() {
		<-t1.C
		err := Cancel(ctx, sid, cid)
		if err != nil {
			t.Errorf("%s: %s\n", sid, err.Error())
		}
	}()

	for {
		rows, eof, err := Fetch(ctx, sid, cid, N)
		if err == context.Canceled {
			//expected
			t.Logf("%s: Breaking due to context canceled", sid)
			break
		}

		if err != nil {
			t.Errorf("%s: %s", sid, err.Error())
			break
		}

		t.Logf("%s: Received %d rows", sid, len(*rows))
		if eof == true {
			t.Logf("%s: Breaking due to constants.EOF", sid)
			break
		}
	}
}

func TestFetchAfterEof(t *testing.T) {
	ctx := context.Background()
	t.Logf("testCancel")
	q := "select * from stores"

	sid, err := NewSession(ctx, "mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
		return
	}

	t.Log(sid)

	cid, err := Query(ctx, sid, q)
	if err != nil {
		t.Errorf("%s: %s\n", sid, err.Error())
		return
	}

	for {
		_, eof, _ := Fetch(ctx, sid, cid, N)

		if eof == true {
			t.Logf("%s: Testing fetch after constants.EOF", sid)
			Fetch(ctx, sid, cid, N)
			break
		}
	}
}
