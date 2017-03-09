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

package models

import (
	"log"

	_ "github.com/go-sql-driver/mysql" // Golang SQL database driver for MySQL
	"github.com/jmoiron/sqlx"
	_ "github.com/kshvakov/clickhouse" // Golang SQL database driver for Yandex ClickHouse

	"github.com/percona/qan-api/config"
)

func init() {
	var err error
	mysqlDSN := config.Get("mysql.dsn")
	mysqlDB, err := sqlx.Connect("mysql", mysqlDSN)

	if err != nil {
		log.Fatalln(err)
	} else {
		log.Println("Connected to MySQL db.")
	}

	// https: //github.com/kshvakov/clickhouse
	clickDSN := config.Get("clickhouse.dsn")
	clickDB, err := sqlx.Open("clickhouse", clickDSN)

	if err != nil {
		log.Fatalln(err)
	} else {
		log.Println("Connected to ClickHouse db.")
	}

	DefaultBase = newBase(mysqlDB, clickDB)
}

// DefaultBase uses to share db connection
var DefaultBase *Base

// Base - uses to share db connection and common stuff
type Base struct {
	mysqlDB *sqlx.DB
	clickDB *sqlx.DB
}

// newBase create new base
func newBase(mysqlDB, clickDB *sqlx.DB) *Base {
	return &Base{mysqlDB, clickDB}
}
