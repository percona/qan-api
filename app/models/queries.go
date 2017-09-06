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
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	"github.com/percona/qan-api/app/shared"
	queryService "github.com/percona/qan-api/service/query"
	"github.com/revel/revel"
)

// Query - represents query class
type Query struct {
	ID          string `json:"Id" db:"checksum"` // 9C8DEE410FA0E0C8
	Abstract    string `db:"abstract"`           // SELECT tbl1
	Fingerprint string `db:"fingerprint"`        // select col from tbl1 where id=?
	Tables      []queryService.Table
	FirstSeen   *time.Time `db:"first_seen"`
	LastSeen    *time.Time `db:"last_seen"`
	Status      string     `db:"status"`
}

type QueryPlain struct {
	ID          string    `json:"Id" db:"checksum"` // 9C8DEE410FA0E0C8
	Abstract    string    `db:"abstract"`           // SELECT tbl1
	Fingerprint string    `db:"fingerprint"`        // select col from tbl1 where id=?
	TablesJSON  string    `db:"tables"`
	FirstSeen   time.Time `db:"first_seen"`
	LastSeen    time.Time `db:"last_seen"`
	Status      string    `db:"status"`
}

// Example - query example of query class
type Example struct {
	QueryID      string    `json:"QueryId" db:"uuid"` // Query.Id
	InstanceUUID string    `db:"checksum"`            // Instance.UUID
	Period       time.Time `db:"period"`
	Ts           time.Time `db:"ts"`
	Db           string    `db:"db"`
	QueryTime    float64   `db:"Query_time"`
	Query        string    `db:"query"`
}

// QueryManager contains methods to work with Query classes and query examples
type QueryManager struct {
	conns *ConnectionsPool
}

// NewQueryManager returns QueryManager with db connections pool.
func NewQueryManager(conns interface{}) QueryManager {
	connsPool := conns.(*ConnectionsPool)
	return QueryManager{connsPool}
}

// GetClassID - get query classes identifier.
func (queryMgr *QueryManager) GetClassID(checksum string) (uint, error) {
	if checksum == "" {
		return 0, nil
	}
	var classID uint
	const query = `
		SELECT query_class_id FROM query_classes WHERE checksum = ?
	`
	err := queryMgr.conns.SQLite.Get(&classID, query, checksum)
	if err != nil {
		return 0, err
	}
	return classID, nil
}

// Get - select query classes for given checksums
func (queryMgr *QueryManager) Get(checksums []string) (map[string]Query, error) {
	const queryQueryClassesTemplate = `
		SELECT checksum, abstract, fingerprint, tables, first_seen, last_seen, status
		FROM query_classes
		WHERE checksum IN ({{ range $index, $value := .}}{{if $index}}, {{end}}'{{$value}}'{{end}})
	`
	tmpl, err := template.New("queryQueryClassesTemplate").Parse(queryQueryClassesTemplate)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare queryQueryClassesTemplate (%v)", err)
	}

	var queryQueryClassesBuffer bytes.Buffer
	err = tmpl.Execute(&queryQueryClassesBuffer, checksums)
	if err != nil {
		return nil, fmt.Errorf("Cannot execute queryQueryClassesBuffer (%v)", err)
	}
	queries := []QueryPlain{}
	err = queryMgr.conns.SQLite.Select(&queries, queryQueryClassesBuffer.String())
	if err != nil {
		revel.ERROR.Printf("queryQueryClassesBuffer.String() eror: %v ", err)
	}

	queriesMap := map[string]Query{}

	for _, query := range queries {
		var tables []queryService.Table
		if query.TablesJSON != "" {
			_ = json.Unmarshal([]byte(query.TablesJSON), &tables)
		}
		queriesMap[query.ID] = Query{
			ID:          query.ID,
			Abstract:    query.Abstract,
			Fingerprint: query.Fingerprint,
			Tables:      tables,
			FirstSeen:   query.FirstSeen,
			LastSeen:    query.LastSeen,
			Status:      query.Status,
		}
	}

	return queriesMap, nil
}

// Examples - select query examples for given query class and instance
func (queryMgr *QueryManager) Examples(classID, instanceID uint) ([]Example, error) {

	const queryExampleTemplate = `
		SELECT c.checksum, i.uuid, e.period, e.ts, e.db, e.Query_time, e.query
		FROM query_examples e
		JOIN query_classes c USING (query_class_id)
		JOIN instances i USING (instance_id)
		WHERE query_class_id = ? {{if .}} AND instance_id = {{.}} {{end}}
		ORDER BY period DESC
	`

	tmpl, err := template.New("queryExampleTemplate").Parse(queryExampleTemplate)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare queryExampleTemplate (%v)", err)
	}

	var queryExampleBuffer bytes.Buffer
	err = tmpl.Execute(&queryExampleBuffer, classID)
	if err != nil {
		return nil, fmt.Errorf("Cannot execute queryQueryClassesBuffer (%v)", err)
	}

	examples := []Example{}
	err = queryMgr.conns.SQLite.Select(&examples, queryExampleBuffer.String(), instanceID)
	if err != nil {
		return nil, fmt.Errorf("Cannot select queryQueryClassesBuffer (%v)", err)
	}

	return examples, nil
}

// Example - get query example for given period
func (queryMgr *QueryManager) Example(classID, instanceID uint, period time.Time) (*Example, error) {
	const query = `
		SELECT period, ts, db, Query_time, query
		FROM query_examples
		WHERE query_class_id = ? AND instance_id = ? AND period <= ?
		ORDER BY period DESC
		LIMIT 1
	`
	example := Example{}
	err := queryMgr.conns.SQLite.Get(&example, query, classID, instanceID, period)
	if err != nil {
		return nil, shared.ErrNotFound
	}
	return &example, nil
}

// UpdateExample - update query example for given query class
func (queryMgr *QueryManager) UpdateExample(classID, instanceID uint, example Example) error {
	const query = `
		UPDATE query_examples SET db = ?
		WHERE query_class_id = ? AND instance_id = ? AND period = ?
	`
	result, err := queryMgr.conns.SQLite.Exec(query, example.Db, classID, instanceID, example.Period)
	if err != nil {
		return fmt.Errorf("UpdateExample: UPDATE query_examples: %v", err)
	}
	rowsAffected, err := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("UpdateExample: query not found: %v", err)
	}
	return nil
}

// UpdateTables - updates tables that were used for query class
// previous comment was:
//  - We store []Table as a JSON string because this is SQL, not NoSQL.
// in clickhouse will use json field type.
func (queryMgr *QueryManager) UpdateTables(classID uint, tables []queryService.Table) error {
	const query = `
		UPDATE query_classes SET tables = ? WHERE query_class_id = ?
	`
	bytes, err := json.Marshal(tables)
	if err != nil {
		return err
	}
	_, err = queryMgr.conns.SQLite.Exec(query, string(bytes), classID)
	if err != nil {
		return fmt.Errorf("Cannot update tables for query example: %v", err)
	}
	return nil
}

// Tables - get tables for given query class
func (queryMgr *QueryManager) Tables(classID uint, m *queryService.Mini) ([]queryService.Table, bool, error) {
	created := false

	// First try to get the tables. If we're lucky, they've already been parsed
	// and we're done.
	var tablesJSON string
	const queryTables = `
		SELECT tables FROM query_classes WHERE query_class_id = ?
	`
	err := queryMgr.conns.SQLite.Get(&tablesJSON, queryTables, classID)
	if err != nil {
		return nil, created, fmt.Errorf("Tables: SELECT query_classes (tables): %v", err)
	}

	// We're lucky: we already have tables.
	if tablesJSON != "" {
		var tables []queryService.Table
		err = json.Unmarshal([]byte(tablesJSON), &tables)
		if err != nil {
			return nil, created, err
		}
		return tables, created, nil
	}

	// We're not lucky: this query hasn't been parsed yet, so do it now, if possible.
	var fingerprint string
	const queryFingerprint = `
		SELECT fingerprint FROM query_classes WHERE query_class_id = ?
	`
	err = queryMgr.conns.SQLite.Get(&fingerprint, queryFingerprint, classID)
	if err != nil {
		return nil, created, fmt.Errorf("Tables: SELECT query_classes (fingerprint): %v", err)
	}

	// If this returns an error, then youtube/vitess/go/sqltypes/sqlparser
	// doesn't support the query type.
	tableInfo, err := m.Parse(fingerprint, "")
	if err != nil {
		return nil, created, fmt.Errorf("Not implemented: %v", err)
	}

	// The sqlparser was able to handle the query, so marshal the tables
	// into a string and update the tables column so next time we don't
	// have to parse the query.
	err = queryMgr.UpdateTables(classID, tableInfo.Tables)
	if err != nil {
		return nil, created, fmt.Errorf("Tables: Update tables: %v", err)
	}
	created = true

	return tableInfo.Tables, created, nil
}
