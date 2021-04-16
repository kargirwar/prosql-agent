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
