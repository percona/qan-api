/*
   Copyright (c) 2016, Percona LLC and/or its affiliates. All rights reserved.

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package test

import (
	"database/sql"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/check.v1"
)

func ExecQueries(db *sql.DB, queries []string, t *check.C) {
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
}

func TableDiff(db *sql.DB, table, orderBy, expectFile string) string {
	// Create temp file but close and rm it because MySQL won't write to an existing file.
	tmpFile, err := ioutil.TempFile("/tmp/", table+"-diff-")
	if err != nil {
		log.Fatal(err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())
	defer os.Remove(tmpFile.Name())

	// Select all rows into temp file.
	_, err = db.Exec("SELECT * FROM " + table + " ORDER BY " + orderBy + " INTO OUTFILE '" + tmpFile.Name() + "'")
	if err != nil {
		log.Fatal(err)
	}

	// diff expected and actual ^ rows.
	output, err := exec.Command("diff", "-u", expectFile, tmpFile.Name()).Output()
	updateTestData := os.Getenv("UPDATE_TEST_DATA")
	if updateTestData != "" && len(output) > 0 {
		cmd := exec.Command("cp", "-f", tmpFile.Name(), expectFile)
		if err := cmd.Run(); err != nil {
			log.Printf("UPDATE_TEST_DATA failed: %s", err)
		} else {
			log.Println("Updated", expectFile)
		}
	}
	return string(output)
}

func PrepStmtCount(db *sql.DB) uint {
	var v string
	var n uint
	db.QueryRow("show global status like 'Prepared_stmt_count'").Scan(&v, &n)
	return n
}
