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
	"encoding/json"
	"errors"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/percona/pmm/proto"
)

func WaitRecv(recvChan chan []byte, v interface{}) error {
	select {
	case data := <-recvChan:
		return json.Unmarshal(data, v)
	case <-time.After(1 * time.Second):
		return errors.New("Timeout when receiving data")
	}
	return nil
}

func WaitResp(recvChan chan []byte) *proto.Response {
	resp := &proto.Response{}
	if err := WaitRecv(recvChan, resp); err != nil {
		log.Println(err)
	}
	return resp
}

// n = 0  -> true only when table is empty
// n >= 1 -> true when current number of rows is greater or equal n
func WaitDbRow(db *sql.DB, table string, where string, n int) bool {
	sql := "SELECT COUNT(*) FROM " + table
	if where != "" {
		sql = sql + " WHERE " + where
	}

	timeout := time.After(3 * time.Second)
	for {
		time.Sleep(250 * time.Millisecond)

		var cnt int
		err := db.QueryRow(sql).Scan(&cnt)
		if err != nil {
			log.Fatal(err)
			return false
		}

		if (n == 0 && cnt == 0) || (n >= 1 && cnt >= n) {
			return true
		}

		// Return if timeout.
		select {
		case <-timeout:
			return false
		default:
		}
	}
	return false
}
