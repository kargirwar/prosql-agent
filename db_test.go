package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var billsFields = []string{
	"id",
	"serial",
	"zippin-serial",
	"file-path",
	"url",
	"promo-code-id",
	"store-id",
	"patient-id",
	"delivery-address",
	"documents-id",
	"patient-name",
	"doctor-id",
	"additional-discount",
	"promo-discount",
	"total",
	"advance",
	"net-payable",
	"payment-collected",
	"change-value",
	"payment-method",
	"transaction-status",
	"cheque-number",
	"dunzo-order-id",
	"zomato-order-id",
	"amazon-pay-order-id",
	"redeemed-points",
	"generic-loyalty-ratio",
	"ethical-loyalty-ratio",
	"created-by",
	"created-at",
	"updated-at",
	"payment-updated-by",
}

func TestFetch(t *testing.T) {
	var pool *sql.DB
	pool, err := sql.Open("mysql", "server:dev-server@tcp(127.0.0.1:3306)/test-generico")
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		t.Errorf("%s\n", err.Error())
	}

	ch := make(chan []string)

	go func() {
		for {
			r := <-ch
			if r[0] == "done" {
				break
			}
		}
	}()

	for j := 0; j < 1; j++ {
		start := time.Now()
		fetchRows(t, ctx, pool, "select * from `bills-1` limit 500", ch)
		end := time.Now()
		t.Logf("%d took %s\n", j, end.Sub(start))
	}

	ch <- []string{"done"}

	//var q string
	//for j := 1; j < len(billsFields); j++ {
	////generate select query extracting j columns
	//q = "select "
	//
	//var i int
	//for i = 0; i < (j - 1); i++ {
	//q += "`" + billsFields[i] + "`" + ","
	//}
	//
	//q += "`" + billsFields[i] + "`" + " from `bills-1` limit 1000"
	//
	//s := time.Now()
	//fetchRows(t, ctx, pool, q)
	//e := time.Now()
	//t.Logf("%d took %s\n", j, e.Sub(s))
	//}
	//
	//s := time.Now()
	//fetchRows(t, ctx, pool, "select * from `bills-1` limit 1000")
	//e := time.Now()
	//t.Logf("%d took %s\n", 200, e.Sub(s))

}

func fetchRows(t *testing.T, ctx context.Context, pool *sql.DB, q string, ch chan []string) {
	s := time.Now()
	rows, err := pool.QueryContext(ctx, q)
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}
	e := time.Now()

	qTime := e.Sub(s)

	s = time.Now()
	cols, err := rows.Columns()
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}
	e = time.Now()
	cTime := e.Sub(s)

	s = time.Now()
	var sTime time.Duration
	var aTime time.Duration
	var times []time.Time

	vals := make([]interface{}, len(cols))

	for rows.Next() {
		times = append(times, time.Now())
		s = time.Now()
		for i := range cols {
			vals[i] = &vals[i]
		}
		e = time.Now()
		aTime += e.Sub(s)

		s = time.Now()
		err = rows.Scan(vals...)
		if err != nil {
			t.Errorf("%s\n", err.Error())
		}
		e = time.Now()
		sTime += e.Sub(s)

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

			r = append(r, v)
		}

		ch <- r
	}

	e = time.Now()
	lTime := e.Sub(s)

	if rows.Err() != nil {
		t.Errorf("%s\n", err.Error())
	}

	s = time.Now()
	rows.Close()
	e = time.Now()
	clTime := e.Sub(s)

	t.Logf("q %s c %s a %s s %s l %s cl %s\n", qTime, cTime, aTime, sTime, lTime, clTime)

	for i := 1; i < len(times); i++ {
		t.Logf("n: %s\n", times[i].Sub(times[i-1]))
	}
}
