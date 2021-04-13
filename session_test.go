package main

import (
	"os"
	"testing"
	"time"
)

const N = 2000

func TestNewSession(t *testing.T) {
	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	cid, err := Execute(sid, "select * from users")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(cid)
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
			t.Errorf("%s\n", err.Error())
			break
		}

		t.Logf("Received %d rows", len(*rows))
		if eof == true {
			break
		}
	}
}

//func TestCleanup(t *testing.T) {
//sid, err := NewSession("mysql", "server:dev-server@tcp(127.0.0.1:3306)/test-generico")
//if err != nil {
//t.Errorf("%s\n", err.Error())
//}
//
//t.Log(sid)
//
//_, err := Execute(sid, "select * from invoices")
//if err != nil {
//t.Errorf("%s\n", err.Error())
//}
//}
