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

package db

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/percona/qan-api/app/shared"
)

type Db struct {
	dsn   *DSN
	files []string
	db    *sql.DB
}

func NewDb(dsn string, schemaDir, testDir string) *Db {
	files := []string{
		filepath.Join(schemaDir, "pmm.sql"),
		filepath.Join(testDir, "schema/basic.sql"),
	}
	return newDb(dsn, files)
}

func newDb(dsn string, files []string) *Db {
	d, err := NewDSN(dsn)
	if err != nil {
		panic("Cannot parse " + dsn + ": " + err.Error())
	}

	sqlDb, err := sql.Open("mysql", dsn+"&foreign_key_checks=0")
	if err != nil {
		panic(err)
	}

	// Avoid using too many connections when there are thousands testDbs created
	sqlDb.SetMaxOpenConns(2)
	sqlDb.SetMaxIdleConns(0)

	db := &Db{
		dsn:   d,
		db:    sqlDb,
		files: files,
	}

	return db
}

func (d *Db) Start() (err error) {
	err = d.DropDb()
	if err != nil {
		return err
	}

	err = d.CreateDb()
	if err != nil {
		return err
	}

	err = d.LoadData()
	if err != nil {
		return err
	}

	return nil
}

func (d *Db) Stop() (err error) {
	defer d.db.Close()
	return d.DropDb()
}

func runCmd(cmd *exec.Cmd) error {
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("%s error:\n%s", cmd.Args[0], string(output))
		return err
	}
	return nil
}

func (d *Db) DropDb() (err error) {
	c := exec.Command(
		"mysql",
		"--local-infile=1",
		"--host="+d.dsn.addr,
		"--port="+d.dsn.port,
		"--user="+d.dsn.user,
		"--password="+d.dsn.passwd,
		"-e", "DROP DATABASE IF EXISTS "+d.dsn.dbname,
	)
	return runCmd(c)
}

func (d *Db) CreateDb() (err error) {
	c := exec.Command(
		"mysql",
		"--local-infile=1",
		"-h"+d.dsn.addr,
		"-P"+d.dsn.port,
		"-u"+d.dsn.user,
		"--password="+d.dsn.passwd,
		"-e", "CREATE DATABASE "+d.dsn.dbname+" COLLATE 'utf8_general_ci'")
	return runCmd(c)
}

func (d *Db) LoadSchema(data []byte) (err error) {
	data = append(data, []byte(d.preSQL())...)

	cmd := exec.Command(
		"mysql",
		"--local-infile=1",
		"-h"+d.dsn.addr,
		"-P"+d.dsn.port,
		"-u"+d.dsn.user,
		"--password="+d.dsn.passwd,
		"-D", d.dsn.dbname,
	)

	cmdStdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			done <- err
		}()
		_, err = cmdStdin.Write(data)
		if err != nil {
			return
		}
		err = cmdStdin.Close()
	}()

	if err := runCmd(cmd); err != nil {
		return err
	}

	<-done

	return nil
}

func (d *Db) LoadData() (err error) {
	for _, file := range d.files {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}
		err = d.LoadSchema(data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Db) Dump(query string) (tableData [][]string, err error) {
	db, err := sql.Open("mysql", d.dsn.dsn)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	count := len(columns)
	tableData = [][]string{}
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	for rows.Next() {
		for i := 0; i < count; i++ {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)
		entry := []string{}
		for i := range columns {
			v := ""
			if values[i] == nil {
				v = "\\N"
			} else if t, ok := values[i].(time.Time); ok {
				v = t.Format(shared.MYSQL_DATETIME_LAYOUT)
			} else {
				v = fmt.Sprintf("%s", values[i])
			}
			entry = append(entry, v)
		}
		tableData = append(tableData, entry)
	}

	return tableData, nil
}

func (d *Db) DumpString(query string) ([][]string, error) {
	rows, err := d.Dump(query)
	if err != nil {
		return [][]string{}, err
	}
	stringData := [][]string{}
	for i := range rows {
		columns := []string{}
		for j := range rows[i] {
			columns = append(columns, fmt.Sprintf("%s", rows[i][j]))
		}
		stringData = append(stringData, columns)
	}

	return stringData, nil
}

func (d *Db) DumpJson(query string) (string, error) {
	tableData, err := d.Dump(query)
	if err != nil {
		return "", err
	}

	jsonData, err := json.MarshalIndent(tableData, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

func (d *Db) TableExpected(filename string) [][]string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	scannerLines := bufio.NewScanner(bytes.NewReader(data))
	output := [][]string{}
	for scannerLines.Scan() {
		fields := strings.Split(scannerLines.Text(), "\t")
		output = append(output, fields)
	}
	if err := scannerLines.Err(); err != nil {
		panic(err)
	}
	return output
}

func (d *Db) TableGot(table, orderBy string) [][]string {
	output, _ := d.DumpString("SELECT * FROM " + table + " ORDER BY " + orderBy)
	return output
}

func (d *Db) TruncateTables(tables []string) error {
	for _, table := range tables {
		_, err := d.db.Exec("TRUNCATE TABLE " + table)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Db) TruncateAllTables(orgDB *sql.DB) error {
	rows, err := orgDB.Query("SHOW TABLES")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		err := rows.Scan(&table)
		if err != nil {
			return err
		}
		orgDB.Exec("TRUNCATE TABLE " + table)
	}
	err = rows.Err()
	return err
}

func (d *Db) TruncateDataTables() error {
	rows, err := d.db.Query("SHOW TABLES LIKE 'query%'")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		err := rows.Scan(&table)
		if err != nil {
			return err
		}
		d.db.Exec("TRUNCATE TABLE " + table)
	}
	return nil
}

func (d *Db) LoadDataInfiles(dir string) error {
	files, err := filepath.Glob(dir + "/*.tab")
	if err != nil {
		return err
	}
	for _, file := range files {
		table := strings.TrimSuffix(filepath.Base(file), ".tab")
		table = d.dsn.dbname + "." + table
		sql := fmt.Sprintf("LOAD DATA LOCAL INFILE '%s' INTO TABLE %s", file, table)
		c := exec.Command(
			"mysql",
			"-h"+d.dsn.addr,
			"-P"+d.dsn.port,
			"-u"+d.dsn.user,
			"--password="+d.dsn.passwd,
			"--local-infile=1",
			"-e", d.preSQL()+sql,
		)
		if err := runCmd(c); err != nil {
			return err
		}
	}
	return nil
}

func (d *Db) preSQL() (sql string) {
	if d.dsn.timeZone != "" {
		sql = fmt.Sprintf("SET time_zone = %s;", d.dsn.timeZone)
	}
	return sql
}

func (d *Db) DB() *sql.DB {
	return d.db
}
