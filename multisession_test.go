package main

import (
	"os"
	"strconv"
	"testing"
)

const N = 1000
const NUM_OF_SESSIONS = 2

func TestMultipleSessions(t *testing.T) {
	for i := 1; i <= NUM_OF_SESSIONS; i++ {
		dsn := "DSN" + strconv.Itoa(i)
		sid, err := NewSession("mysql", os.Getenv(dsn))
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

			t.Logf("%s Received %d rows", sid, len(*rows))
			if eof == true {
				break
			}
		}
	}
}
