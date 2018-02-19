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

package query

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/go-sql-driver/mysql"
	queryProto "github.com/percona/pmm/proto/query"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/qan-api/app/shared"
	queryService "github.com/percona/qan-api/service/query"
	"github.com/percona/qan-api/stats"
)

func GetClassId(db *sql.DB, checksum string) (uint, error) {
	if checksum == "" {
		return 0, nil
	}
	var classId uint
	err := db.QueryRow("SELECT query_class_id FROM query_classes WHERE checksum = ?", checksum).Scan(&classId)
	if err != nil {
		return 0, mysql.Error(err, "GetClassId: SELECT query_classes")
	}
	return classId, nil
}

type MySQLHandler struct {
	dbm   db.Manager
	stats *stats.Stats
}

func NewMySQLHandler(dbm db.Manager, stats *stats.Stats) *MySQLHandler {
	h := &MySQLHandler{
		dbm:   dbm,
		stats: stats,
	}
	return h
}

func (h *MySQLHandler) Get(ids []string) (map[string]queryProto.Query, error) {
	q := "SELECT checksum, COALESCE(abstract, ''), fingerprint, COALESCE(tables, ''), first_seen, last_seen, status" +
		" FROM query_classes" +
		" WHERE checksum IN (" + shared.Placeholders(len(ids)) + ")"
	v := shared.GenericStringList(ids)
	rows, err := h.dbm.DB().Query(q, v...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	queries := map[string]queryProto.Query{}
	for rows.Next() {
		query := queryProto.Query{}
		var tablesJSON string
		err := rows.Scan(
			&query.Id,
			&query.Abstract,
			&query.Fingerprint,
			&tablesJSON,
			&query.FirstSeen,
			&query.LastSeen,
			&query.Status,
		)
		if err != nil {
			return nil, err
		}
		if tablesJSON != "" {
			var tables []queryProto.Table
			if err := json.Unmarshal([]byte(tablesJSON), &tables); err != nil {
				return nil, err
			}
			query.Tables = tables
		}
		queries[query.Id] = query
	}

	return queries, nil
}

func (h *MySQLHandler) Examples(classId, instanceId uint) ([]queryProto.Example, error) {
	params := []interface{}{classId}
	q := "SELECT c.checksum, i.uuid, e.period, e.ts, e.db, e.Query_time, e.query, e.truncated" +
		" FROM query_examples e" +
		" JOIN query_classes c USING (query_class_id)" +
		" JOIN instances i USING (instance_id)" +
		" WHERE query_class_id = ?"
	if instanceId != 0 {
		q += " AND instance_id = ?"
		params = append(params, instanceId)
	}
	q += " ORDER BY period DESC"

	rows, err := h.dbm.DB().Query(q, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	examples := []queryProto.Example{}
	for rows.Next() {
		e := queryProto.Example{}
		err := rows.Scan(
			&e.QueryId,
			&e.InstanceUUID,
			&e.Period,
			&e.Ts,
			&e.Db,
			&e.QueryTime,
			&e.Query,
			&e.Truncated,
		)
		if err != nil {
			return nil, err
		}
		examples = append(examples, e)
	}

	return examples, nil
}

func (h *MySQLHandler) Example(classId, instanceId uint, period time.Time) (queryProto.Example, error) {
	e := queryProto.Example{}
	q := "SELECT period, ts, db, Query_time, query, truncated" +
		" FROM query_examples" +
		" WHERE query_class_id = ? AND instance_id = ? AND period <= ?" +
		" ORDER BY period DESC" +
		" LIMIT 1"
	err := h.dbm.DB().QueryRow(q, classId, instanceId, period).Scan(&e.Period, &e.Ts, &e.Db, &e.QueryTime, &e.Query, &e.Truncated)
	if err != nil {
		return e, mysql.Error(err, "Example: SELECT query_examples")
	}
	return e, nil
}

func (h *MySQLHandler) UpdateExample(classId, instanceId uint, example queryProto.Example) error {
	// todo: WHERE query_class_id=? AND instance_id=? AND period=?
	r, err := h.dbm.DB().Exec(
		"UPDATE query_examples SET db = ?"+
			" WHERE query_class_id = ? AND instance_id = ? AND period = ?",
		example.Db, classId, instanceId, example.Period,
	)
	if err != nil {
		return mysql.Error(err, "UpdateExample: UPDATE query_examples")
	}
	rowsAffected, err := r.RowsAffected()
	if rowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (h *MySQLHandler) UpdateTables(classId uint, tables []queryProto.Table) error {
	// We store []query.Table as a JSON string because this is SQL, not NoSQL.
	bytes, err := json.Marshal(tables)
	if err != nil {
		return err
	}
	_, err = h.dbm.DB().Exec("UPDATE query_classes SET tables = ? WHERE query_class_id = ?", string(bytes), classId)
	if err != nil {
		return mysql.Error(err, "UpdateTables: UPDATE query_classes")
	}
	return nil
}

func (h *MySQLHandler) Tables(classId uint, m *queryService.Mini) ([]queryProto.Table, bool, error) {
	created := false

	// First try to get the tables. If we're lucky, they've already been parsed
	// and we're done.
	var tablesJSON string
	err := h.dbm.DB().QueryRow("SELECT COALESCE(tables, '') FROM query_classes WHERE query_class_id = ?", classId).Scan(&tablesJSON)
	if err != nil {
		return nil, created, mysql.Error(err, "Tables: SELECT query_classes (tables)")
	}

	// We're lucky: we already have tables.
	if tablesJSON != "" {
		var tables []queryProto.Table
		if err := json.Unmarshal([]byte(tablesJSON), &tables); err != nil {
			return nil, created, err
		}
		return tables, created, nil
	}

	// We're not lucky: this query hasn't been parsed yet, so do it now, if possible.
	var fingerprint string
	err = h.dbm.DB().QueryRow("SELECT fingerprint FROM query_classes WHERE query_class_id = ?", classId).Scan(&fingerprint)
	if err != nil {
		return nil, created, mysql.Error(err, "Tables: SELECT query_classes (fingerprint)")
	}

	// If this returns an error, then youtube/vitess/go/sqltypes/sqlparser
	// doesn't support the query type.
	tableInfo, err := m.Parse(fingerprint, "")
	if err != nil {
		return nil, created, shared.ErrNotImplemented
	}

	// The sqlparser was able to handle the query, so marshal the tables
	// into a string and update the tables column so next time we don't
	// have to parse the query.
	if err := h.UpdateTables(classId, tableInfo.Tables); err != nil {
		return nil, created, err
	}
	created = true

	return tableInfo.Tables, created, nil
}
