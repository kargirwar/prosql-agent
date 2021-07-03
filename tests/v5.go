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
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	//Connect to database and check for errors
	db, err := sql.Open("mysql", "server:dev-server@tcp(localhost:3306)/test-generico")
	if err != nil {
		log.Println(err)
	}

	//rows, err := db.Query("SELECT id, name, `store-id` FROM users where `store-id` is null limit 5")
	rows, err := db.Query("SELECT * from users limit 5")
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	cols, err := rows.Columns() // Remember to check err afterwards
	if err != nil {
		log.Fatal(err)
	}

	vals := make([]interface{}, len(cols))
	var results [][]string

	for rows.Next() {

		for i := range cols {
			vals[i] = &vals[i]
		}

		err = rows.Scan(vals...)
		// Now you can check each element of vals for nil-ness,
		if err != nil {
			log.Fatal(err)
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

			r = append(r, v)

			fmt.Printf("%s %s\n", c, v)
		}

		results = append(results, r)
	}

	fmt.Println(results)
}
