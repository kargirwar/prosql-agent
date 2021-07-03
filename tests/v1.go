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

	columnTypes, err := rows.ColumnTypes()
	fck(err)
	log.Println("read columnTypes")

	for i, t := range columnTypes {
		log.Printf("%d %v %s\n", i, columnNames[i], t.ScanType().String())
	}

	rc := NewStringStringScan(columnNames)

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
  using a map
*/
type mapStringScan struct {
	// cp are the column pointers
	cp []interface{}
	// row contains the final result
	row      map[string]string
	colCount int
	colNames []string
}

func NewMapStringScan(columnNames []string) *mapStringScan {
	lenCN := len(columnNames)
	s := &mapStringScan{
		cp:       make([]interface{}, lenCN),
		row:      make(map[string]string, lenCN),
		colCount: lenCN,
		colNames: columnNames,
	}
	for i := 0; i < lenCN; i++ {
		s.cp[i] = new(sql.RawBytes)
	}
	return s
}

func (s *mapStringScan) Update(rows *sql.Rows) error {
	if err := rows.Scan(s.cp...); err != nil {
		return err
	}

	for i := 0; i < s.colCount; i++ {
		if rb, ok := s.cp[i].(*sql.RawBytes); ok {
			s.row[s.colNames[i]] = string(*rb)
			*rb = nil // reset pointer to discard current value to avoid a bug
		} else {
			return fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, s.colNames[i])
		}
	}
	return nil
}

func (s *mapStringScan) Get() map[string]string {
	return s.row
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
}

func NewStringStringScan(columnNames []string) *stringStringScan {
	lenCN := len(columnNames)
	s := &stringStringScan{
		cp:       make([]interface{}, lenCN),
		row:      make([]string, lenCN*2),
		colCount: lenCN,
		colNames: columnNames,
	}
	j := 0
	for i := 0; i < lenCN; i++ {
		s.cp[i] = new(sql.RawBytes)
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

// rowMapString was the first implementation but it creates for each row a new
// map and pointers and is considered as slow. see benchmark
func rowMapString(columnNames []string, rows *sql.Rows) (map[string]string, error) {
	lenCN := len(columnNames)
	ret := make(map[string]string, lenCN)

	columnPointers := make([]interface{}, lenCN)
	for i := 0; i < lenCN; i++ {
		columnPointers[i] = new(sql.RawBytes)
	}

	if err := rows.Scan(columnPointers...); err != nil {
		return nil, err
	}

	for i := 0; i < lenCN; i++ {
		if rb, ok := columnPointers[i].(*sql.RawBytes); ok {
			ret[columnNames[i]] = string(*rb)
		} else {
			return nil, fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, columnNames[i])
		}
	}

	return ret, nil
}

func fck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
