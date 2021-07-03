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
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/dchest/uniuri"
)

const BASE_URL = "http://localhost:23890"

var client = &http.Client{Timeout: 10 * time.Second}
var creds = map[string]string{
	"user": "server",
	"pass": "dev-server",
	"host": "127.0.0.1",
	"port": "3306",
	"db":   "test-generico",
}

var ctx = context.Background()

const N = 1000

func TestSid(t *testing.T) {
	sid := "VPtBxUGvRy9Eg3M3"
	setdb_test(t, sid, "test-generico")
	cid := execute_test(t, sid, "select * from `authorized-devices`")
	fetch_test(t, sid, cid)
}

func TestCorrupted(t *testing.T) {
	//login and get session id
	sid := login_test(t)
	//get db list

	cid := execute_test(t, sid, "select description from `vouchers` where id = 606")
	rows := fetch_test(t, sid, cid)
	log.Println(rows)
}

func TestClientExecute(t *testing.T) {
	//login and get session id
	sid := login_test(t)
	//get db list

	cid := execute_test(t, sid, "select * from users limit 1000")
	fetch_test(t, sid, cid)
}

func TestClientLogin(t *testing.T) {
	//login and get session id
	sid := login_test(t)
	//get db list
	cid := execute_test(t, sid, "show databases")
	fetch_test(t, sid, cid)

	//get table list
	cid = execute_test(t, sid, "show tables from `test-generico`")
	fetch_test(t, sid, cid)

	//select database
	//setdb_test(t, sid, "test-generico")
	//get data
	cid = execute_test(t, sid, "select * from inventory limit 1000")
	fetch_test(t, sid, cid)
	//t.Log(rows)

	//change database
	//setdb_test(t, sid, "dev3-generico")
	//get data
	cid = execute_test(t, sid, "select * from users limit 1")
	fetch_test(t, sid, cid)
	//t.Log(rows)

	//change database
	//setdb_test(t, sid, "test-generico")
	//get data
	cid = execute_test(t, sid, "select * from `invoice-items-1` limit 1000")
	fetch_test(t, sid, cid)

	//t.Log(rows)
}

func login_test(t *testing.T) string {
	defer TimeTrack(ctx, time.Now())

	r := &Response{}

	err := getJson(getQuery(t, "login", creds), r)
	if err != nil {
		t.Fatalf(err.Error())
	}

	sid := getValue(t, r, "session-id")
	return sid
}

func setdb_test(t *testing.T, sid, db string) {
	defer TimeTrack(ctx, time.Now())

	r := &Response{}
	params := map[string]string{
		"session-id": sid,
		"db":         db,
	}

	err := getJson(getQuery(t, "set-db", params), r)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if r.Status != "ok" {
		t.Fatalf(r.Msg)
	}
}

func execute_test(t *testing.T, sid, query string) string {
	defer TimeTrack(ctx, time.Now())

	r := &Response{}
	params := map[string]string{
		"session-id": sid,
		"query":      query,
	}

	err := getJson(getQuery(t, "execute", params), r)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if r.Status != "ok" {
		t.Fatalf(r.Msg)
	}

	return getValue(t, r, "cursor-id")
}

func fetch_test(t *testing.T, sid, cid string) interface{} {
	defer TimeTrack(ctx, time.Now())

	r := &Response{}
	params := map[string]string{
		"session-id":  sid,
		"cursor-id":   cid,
		"num-of-rows": strconv.Itoa(N),
	}

	err := getJson(getQuery(t, "fetch", params), r)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if r.Status != "ok" {
		t.Errorf(r.Msg)
		return r.Data
	}

	return r.Data
}

func getValue(t *testing.T, r *Response, k string) string {
	m, ok := r.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Unable to parse JSON")
	}
	return m[k].(string)
}

func getQuery(t *testing.T, path string, m map[string]string) string {
	base, err := url.Parse(BASE_URL)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Path params
	base.Path += path

	// Query params
	params := url.Values{}
	for k, v := range m {
		params.Add(k, v)
		base.RawQuery = params.Encode()
	}

	return base.String()
}

func getJson(url string, target interface{}) error {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	r.Header.Add("X-Request-Id", uniuri.New())
	res, err := client.Do(r)
	defer res.Body.Close()

	//body, err := ioutil.ReadAll(res.Body)
	//bodyString := string(body)
	//fmt.Println(bodyString)

	return json.NewDecoder(res.Body).Decode(target)
}
