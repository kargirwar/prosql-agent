package main

import (
	"log"
	"testing"
)

const N = 1000
const NUM_OF_SESSIONS = 2

func TestSetDB(t *testing.T) {
	dsn := parseDSN()

	for _, d := range dsn {
		log.Println(d)
		sid, err := NewSession("mysql", d)
		if err != nil {
			t.Errorf("%s\n", err.Error())
		}

		t.Log(sid)

		cid, err := Execute(sid, "select name from users limit 1")
		if err != nil {
			t.Errorf("%s\n", err.Error())
		}

		rows, _, err := Fetch(sid, cid, N)
		if err != nil {
			t.Errorf("%s\n", err.Error())
		}

		log.Println(rows)
	}
}
