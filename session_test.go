package main

import (
	"testing"
)

const N = 100

func TestNewSession(t *testing.T) {
	sid, err := NewSession("mysql", "server:dev-server@tcp(127.0.0.1:3306)/test-generico")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	cid, err := Execute(sid, "select * from users")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(cid)

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
