//https://gist.github.com/SchumacherFM/69a167bec7dea644a20e
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
	//"encoding/json"
	"database/sql"
	"fmt"
	"log"
	_ "strconv"

	_ "github.com/go-sql-driver/mysql"
)

const (
	TEST_QUERY = `SELECT * FROM users`
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("start")
	var result [][]string

	db, err := sql.Open("mysql", "server:dev-server@tcp(127.0.0.1:3306)/test-generico")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.Println("opened db")

	rows, err := db.Query(TEST_QUERY)
	fck(err)
	defer rows.Close()

	columnNames, err := rows.Columns()
	fck(err)
	log.Println("read columns")

	colTypes, err := rows.ColumnTypes()
	fck(err)
	log.Println("read column types")

	rc := NewStringStringScan(columnNames, colTypes)

	log.Println("reading rows")
	for rows.Next() {
		err := rc.Update(rows)
		fck(err)
		cv := rc.Get()
		var s = make([]string, len(cv))
		copy(s, cv)
		result = append(result, s)
	}
	log.Println("end")
}

/**
  using a string slice
*/
type stringStringScan struct {
	// cp are the column pointers
	cp []interface{}
	// row contains the final result
	row      []string
	colCount int
	colNames []string
	colTypes []*sql.ColumnType
}

func NewStringStringScan(columnNames []string, columnTypes []*sql.ColumnType) *stringStringScan {
	lenCN := len(columnNames)
	s := &stringStringScan{
		cp:       make([]interface{}, lenCN),
		row:      make([]string, lenCN*2),
		colCount: lenCN,
		colNames: columnNames,
		colTypes: columnTypes,
	}

	j := 0
	for i := 0; i < lenCN; i++ {

		scanType := columnTypes[i].ScanType()
		//if scanType == sql.NullBool ||
		//scanType == sql.NullFloat64 ||
		//scanType == sql.NullInt32 ||
		//scanType == sql.NullInt64 ||
		//scanType == sql.NullString ||
		//scanType == sql.NullTime {
		//s.cp[i] = new(scanType)
		//continue
		//}
		switch scanType.String() {
		case "sql.NullBool":
			s.cp[i] = new(sql.NullBool)
			continue

		case "sql.NullFloat64":
			s.cp[i] = new(sql.NullFloat64)
			continue

		case "sql.NullInt32":
			s.cp[i] = new(sql.NullInt32)
			continue

		case "sql.NullInt64":
			s.cp[i] = new(sql.NullInt64)
			continue

		case "sql.NullString":
			s.cp[i] = new(sql.NullString)
			continue

		case "sql.NullTime":
			s.cp[i] = new(sql.NullTime)
			continue
		}

		s.cp[i] = new([]byte)
		s.row[j] = s.colNames[i]
		j = j + 2
	}

	return s
}

func (s *stringStringScan) Update(rows *sql.Rows) error {
	if err := rows.Scan(s.cp...); err != nil {
		return err
	}

	j := 0
	for i := 0; i < s.colCount; i++ {
		if rb, ok := s.cp[i].(*sql.RawBytes); ok {
			s.row[j+1] = string(*rb)
			*rb = nil // reset pointer to discard current value to avoid a bug
		} else {
			return fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, s.colNames[i])
		}
		j = j + 2
	}

	return nil
}

func (s *stringStringScan) Get() []string {
	return s.row
}

func fck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
