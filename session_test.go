package main

import (
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

const N = 1000

func TestNewSession(t *testing.T) {
	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)
}

func TestExecute(t *testing.T) {
	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	_, err = Execute(sid, "select * from users")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}
}

func TestFetch(t *testing.T) {
	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	cid, err := Execute(sid, "select * from users")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	for {
		rows, eof, err := Fetch(sid, cid, N)
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
	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	cid, err := Execute(sid, "select * from invoices")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	//test cancellation
	t1 := time.NewTimer(10 * time.Second)
	go func() {
		<-t1.C
		err := Cancel(sid, cid)
		if err != nil {
			t.Errorf("%s\n", err.Error())
		}
	}()

	for {
		rows, eof, err := Fetch(sid, cid, N)
		if err != nil {
			if err.Error() == ERR_INVALID_CURSOR_ID {
				break
			}
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
const STRESS_STOP_TIME = 30 * time.Second

//create several sessions. Leave some sessions unused
//Fetch data from some sessions
//Cancel a few sessions
//Repeat several times for a long period of time.
func TestStress(t *testing.T) {
	n := strconv.Itoa(N)
	n10 := strconv.Itoa(N * 10)
	n100 := strconv.Itoa(N * 10)

	queries := []string{
		"select * from users limit " + n,
		"select * from invoices limit " + n100,
		"select * from `bills-1` limit " + n100,
		"select * from `short-book-1` limit " + n10,
	}

	ticker := time.NewTicker(STRESS_TICKER_INTERVAL)
	stopTimer := time.NewTimer(STRESS_STOP_TIME)
loop:
	for {
		select {
		case <-stopTimer.C:
			break loop

		case <-ticker.C:
			startTest(t, queries)
		}
	}
}

func startTest(t *testing.T, queries []string) {
	t.Errorf("Starting new test")
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
	q := queries[rand.Intn(len(queries))]
	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	_, err = Execute(sid, q)
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}
}

func testFetch(t *testing.T, queries []string) {
	q := queries[rand.Intn(len(queries))]

	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	cid, err := Execute(sid, q)
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	for {
		rows, eof, err := Fetch(sid, cid, N)
		if err != nil {
			if err.Error() == ERR_INVALID_CURSOR_ID {
				t.Logf("%s: Breaking due to ERR_INVALID_CURSOR_ID", sid)
				break
			}
		}

		t.Logf("%s: Received %d rows", sid, len(*rows))
		if eof == true {
			t.Logf("%s: Breaking due to EOF", sid)
			break
		}
	}
}

func testCancel(t *testing.T, queries []string) {
	q := queries[rand.Intn(len(queries))]

	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	cid, err := Execute(sid, q)
	if err != nil {
		t.Errorf("%s: %s\n", sid, err.Error())
	}

	//test cancellation
	t1 := time.NewTimer(2 * time.Second)
	go func() {
		<-t1.C
		err := Cancel(sid, cid)
		if err != nil {
			t.Errorf("%s: %s\n", sid, err.Error())
		}
	}()

	for {
		rows, eof, err := Fetch(sid, cid, N)
		if err != nil {
			if err.Error() == ERR_INVALID_CURSOR_ID {
				t.Logf("%s: Breaking due to ERR_INVALID_CURSOR_ID", sid)
				break
			}
		}

		t.Logf("%s: Received %d rows", sid, len(*rows))
		if eof == true {
			t.Logf("%s: Breaking due to EOF", sid)
			break
		}
	}
}
