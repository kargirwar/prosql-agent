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
	"log"
	"os"
	"testing"
)

const N = 1000

func TestSetDb(t *testing.T) {
	sid, err := NewSession("mysql", os.Getenv("DSN"))
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	t.Log(sid)

	//get some data from first db
	dbs := []string{"test-generico", "dev3-generico", "pankaj-02-24-generico"}
	for _, db := range dbs {
		err := SetDb(sid, db)
		if err != nil {
			t.Errorf("%s\n", err.Error())
		}

		testQuery(t, sid, "show databases")
		testQuery(t, sid, "show tables")
		testQuery(t, sid, "select * from users limit 1")
	}
}

func testQuery(t *testing.T, sid, query string) {
	cid, err := Execute(sid, query)
	if err != nil {
		t.Errorf("%s\n", err.Error())
	}

	for {
		rows, _, err := Fetch(sid, cid, N)
		if err != nil {
			t.Errorf("%s\n", err.Error())
			break
		}

		log.Println(rows)
		break
	}
}
