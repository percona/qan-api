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

package qan

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/percona/go-mysql/event"
	"github.com/percona/pmm/proto/metrics"
	qp "github.com/percona/pmm/proto/qan"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/service/query"
)

const (
	maxAbstract    = 100  // query_classes.abstract
	maxFingerprint = 5000 // query_classes.fingerprint
)

type MySQLMetricWriter struct {
	conns *models.ConnectionsPool
	m     *query.Mini
	// --
	stmtInsertClassMetrics *sql.Stmt
	stmtInsertQueryExample *sql.Stmt
	stmtInsertQueryClass   *sql.Stmt
	stmtUpdateQueryClass   *sql.Stmt
}

func NewMySQLMetricWriter(conns interface{}, m *query.Mini) *MySQLMetricWriter {
	connsPool := conns.(*models.ConnectionsPool)
	return &MySQLMetricWriter{conns: connsPool, m: m}
}

func (h *MySQLMetricWriter) Write(report qp.Report) error {
	var err error
	instanceMgr := models.NewInstanceManager(h.conns)
	instanceID, in, err := instanceMgr.Get(report.UUID)
	if err != nil {
		return fmt.Errorf("qan.mysql.go.Write: models.InstanceManager.Get: %v", err)
	}

	if report.Global == nil {
		return fmt.Errorf("missing report.Global")
	}

	if report.Global.Metrics == nil {
		return fmt.Errorf("missing report.Global.Metrics")
	}

	trace := fmt.Sprintf("MySQL %s", report.UUID)

	h.prepareStatements()
	defer h.closeStatements()

	// Default last_seen if no example query ts.
	reportStartTs := report.StartTs.Format(shared.MYSQL_DATETIME_LAYOUT)

	// Metrics from slow log vs. from Performance Schema differ. For example,
	// from slow log we get all Lock_time stats: sum, min, max, avg, p95, med.
	// Perf Schema only has Log_time_sum. In this case, we want to store NULL
	// for the other stats, not zero. We can't detect this from input because
	// Go will default those stats to zero because event.TimeStats.Min is float
	// not interface.
	fromSlowLog := true
	if report.SlowLogFile == "" {
		fromSlowLog = false // from perf schema
	}

	// //////////////////////////////////////////////////////////////////////
	// Insert class metrics into query_class_metrics
	// //////////////////////////////////////////////////////////////////////
	classDupes := 0
	for _, class := range report.Class {
		lastSeen := ""
		if class.Example != nil {
			lastSeen = class.Example.Ts
		}
		if lastSeen == "" {
			lastSeen = reportStartTs
		}

		queryMgr := models.NewQueryManager(h.conns)
		id, err := queryMgr.GetClassID(class.Id)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("WARNING: cannot get query class ID, skipping: %s: %#v: %s", err, class, trace)
			continue
		}

		if id != 0 {
			// Existing class, update it.  These update aren't fatal, but they shouldn't fail.
			if err := h.updateQueryClass(id, lastSeen); err != nil {
				log.Printf("WARNING: cannot update query class, skipping: %s: %#v: %s", err, class, trace)
				continue
			}
		} else {
			// New class, create it.
			id, err = h.newClass(instanceID, in.Subsystem, class, lastSeen)
			if err != nil {
				log.Printf("WARNING: cannot create new query class, skipping: %s: %#v: %s", err, class, trace)
				continue
			}
		}

		// Update query example if this example has a greater Query_time.
		// The "!= nil" is for agent >= v1.0.11 which use *event.Class.Example,
		// but agent <= v1.0.10 don't use a pointer so the struct is always
		// present, so "class.Example.Query != """ filters out empty examples.
		if class.Example != nil && class.Example.Query != "" {
			if err = h.updateQueryExample(instanceID, class, id, lastSeen); err != nil {
				log.Printf("WARNING: cannot update query example: %s: %#v: %s", err, class, trace)
			}
		}

		vals := h.getMetricValues(class.Metrics, fromSlowLog)
		classVals := []interface{}{
			id,
			instanceID,
			report.StartTs,
			report.EndTs,
			class.TotalQueries,
			0, // todo: `lrq_count`,
		}
		classVals = append(classVals, vals...)

		// INSERT query_class_metrics
		_, err = h.stmtInsertClassMetrics.Exec(classVals...)
		if err != nil {
			if mysql.ErrorCode(err) == mysql.ER_DUP_ENTRY {
				classDupes++
				// warn below
			} else {
				log.Printf("WARNING: cannot insert query class metrics: %s: %#v: %s", err, class, trace)
			}
		}
	}

	if classDupes > 0 {
		log.Printf("WARNING: %s duplicate query class metrics: start_ts='%s': %s", classDupes, report.StartTs, trace)
	}

	return nil
}

func (h *MySQLMetricWriter) newClass(instanceID uint, subsystem string, class *event.Class, lastSeen string) (uint, error) {
	var queryAbstract, queryQuery string
	var tables interface{}

	switch subsystem {
	case "mysql":
		// In theory the shortest valid SQL statment should be at least 2 chars
		// (I'm not aware of any 1 char statments), so if the fingerprint is shorter
		// than 2 then it's invalid, effectively empty, probably due to a slow log
		// parser bug.
		if len(class.Fingerprint) < 2 {
			return 0, fmt.Errorf("empty fingerprint")
		} else if class.Fingerprint[len(class.Fingerprint)-1] == '\n' {
			// https://jira.percona.com/browse/PCT-826
			class.Fingerprint = strings.TrimSuffix(class.Fingerprint, "\n")
			log.Printf("WARNING: fingerprint had newline: %s: instance_id=%d", class.Fingerprint, instanceID)
		}

		// Distill the query (select c from t where id=? -> SELECT t) and extract
		// its tables if possible. query.Query = fingerprint (cleaned up).
		defaultDb := ""
		if class.Example != nil {
			defaultDb = class.Example.Db
		}

		query, err := h.m.Parse(class.Fingerprint, defaultDb)
		if err != nil {
			return 0, err
		}
		if len(query.Tables) > 0 {
			bytes, _ := json.Marshal(query.Tables)
			tables = string(bytes)
		}

		// Truncate long fingerprints and abstracts to avoid MySQL warning 1265:
		// Data truncated for column 'abstract'
		if len(query.Query) > maxFingerprint {
			query.Query = query.Query[0:maxFingerprint-3] + "..."
		}
		if len(query.Abstract) > maxAbstract {
			query.Abstract = query.Abstract[0:maxAbstract-3] + "..."
		}
		queryAbstract = query.Abstract
		queryQuery = query.Query
	case "mongo":
		queryAbstract = class.Fingerprint
		queryQuery = class.Fingerprint
	}

	// Create the query class which is internally identified by its query_class_id.
	// The query checksum is the class is identified externally (in a QAN report).
	// Since this is the first time we've seen the query, firstSeen=lastSeen.
	res, err := h.stmtInsertQueryClass.Exec(class.Id, queryAbstract, queryQuery, tables, lastSeen, lastSeen)

	if err != nil {
		if mysql.ErrorCode(err) == mysql.ER_DUP_ENTRY {
			// Duplicate entry; someone else inserted the same server
			// (or caller didn't check first).  Return its server_id.
			queryMgr := models.NewQueryManager(h.conns)
			return queryMgr.GetClassID(class.Id)
		}
		// Other error, let caller handle.
		return 0, mysql.Error(err, "newClass INSERT query_classes")
	}
	classId, err := res.LastInsertId()
	if err != nil {
		return 0, mysql.Error(err, "newClass res.LastInsertId")
	}

	return uint(classId), nil // success
}

func (h *MySQLMetricWriter) updateQueryClass(queryClassId uint, lastSeen string) error {
	_, err := h.stmtUpdateQueryClass.Exec(lastSeen, lastSeen, queryClassId)
	return mysql.Error(err, "updateQueryClass UPDATE query_classes")
}

func (h *MySQLMetricWriter) updateQueryExample(instanceID uint, class *event.Class, classId uint, lastSeen string) error {
	// INSERT ON DUPLICATE KEY UPDATE
	_, err := h.stmtInsertQueryExample.Exec(instanceID, classId, lastSeen, lastSeen, class.Example.Db, class.Example.QueryTime, class.Example.Query)
	return mysql.Error(err, "updateQueryExample INSERT query_examples")
}

func (h *MySQLMetricWriter) getMetricValues(e *event.Metrics, fromSlowLog bool) []interface{} {
	// The "if fromSlowLog" conditionals here prevent storing zero because
	// metrics from Perf Schema don't have most stats, usually just _sum.
	// Since zero is a valid value (e.g. Rows_examined_min=0) we need to
	// clearly distinguish between real zero values and vals not reported.
	vals := make([]interface{}, len(metricColumns))
	i := 0
	for _, m := range metrics.Query {

		// Counter/bools
		if (m.Flags & metrics.COUNTER) != 0 {
			stats, haveMetric := e.BoolMetrics[m.Name]
			if haveMetric {
				vals[i] = stats.Sum
			}
			i++
			continue
		}

		// Microsecond/time
		if (m.Flags & metrics.MICROSECOND) != 0 {
			stats, haveMetric := e.TimeMetrics[m.Name]
			for _, stat := range metrics.StatNames {
				if stat == "p5" {
					continue
				}
				var val interface{} = nil
				if haveMetric {
					switch stat {
					case "sum":
						val = stats.Sum
					case "min":
						if fromSlowLog || m.Name == "Query_time" {
							val = stats.Min
						}
					case "max":
						if fromSlowLog || m.Name == "Query_time" {
							val = stats.Max
						}
					case "avg":
						if fromSlowLog || m.Name == "Query_time" {
							val = stats.Avg
						}
					case "p95":
						if fromSlowLog {
							val = stats.P95
						}
					case "med":
						if fromSlowLog {
							val = stats.Med
						}
					default:
						log.Printf("ERROR: unknown stat: %s %s\n", m.Name, stat)
					}
				}
				vals[i] = val
				i++
			}
			continue
		}

		// Metric isn't microsecond or bool/counter, so it must be numbers, like Rows_sent.
		stats, haveMetric := e.NumberMetrics[m.Name]
		for _, stat := range metrics.StatNames {
			if stat == "p5" {
				continue
			}
			var val interface{} = nil
			if haveMetric {
				switch stat {
				case "sum":
					val = stats.Sum
				case "min":
					if fromSlowLog || m.Name == "Query_time" {
						val = stats.Min
					}
				case "max":
					if fromSlowLog || m.Name == "Query_time" {
						val = stats.Max
					}
				case "avg":
					if fromSlowLog || m.Name == "Query_time" {
						val = stats.Avg
					}
				case "p95":
					if fromSlowLog {
						val = stats.P95
					}
				case "med":
					if fromSlowLog {
						val = stats.Med
					}
				default:
					log.Printf("ERROR: unknown stat: %s %s\n", m.Name, stat)
				}
			}
			vals[i] = val
			i++
		}
	}

	return vals
}

func (h *MySQLMetricWriter) prepareStatements() {
	var err error

	// INSERT

	h.stmtInsertClassMetrics, err = h.conns.SQLite.Prepare(insertClassMetrics)
	if err != nil {
		panic("Failed to prepare stmtInsertClassMetrics: " + err.Error())
	}

	h.stmtInsertQueryExample, err = h.conns.SQLite.Prepare(
		"INSERT INTO query_examples" +
			" (instance_id, query_class_id, period, ts, db, Query_time, query)" +
			" VALUES (?, ?, DATE(?), ?, ?, ?, ?)" +
			" ON DUPLICATE KEY UPDATE" +
			" query=IF(VALUES(Query_time) > COALESCE(Query_time, 0), VALUES(query), query)," +
			" ts=IF(VALUES(Query_time) > COALESCE(Query_time, 0), VALUES(ts), ts)," +
			" Query_time=IF(VALUES(Query_time) > COALESCE(Query_time, 0), VALUES(Query_time), Query_time)," +
			" db=IF(VALUES(Query_time) > COALESCE(Query_time, 0), VALUES(db), db)")
	if err != nil {
		panic("Failed to prepare stmtInsertQueryExample: " + err.Error())
	}

	/* Why use LEAST and GREATEST and update first_seen?
	   Because of the asynchronous nature of agents communication, we can receive
	   the same query from 2 different agents but it isn't madatory that the first
	   one we receive, is the older one. There could have been a network error on
	   the agent having the oldest data
	*/
	h.stmtInsertQueryClass, err = h.conns.SQLite.Prepare(
		"INSERT INTO query_classes" +
			" (checksum, abstract, fingerprint, tables, first_seen, last_seen)" +
			" VALUES (?, ?, ?, ?, COALESCE(?, NOW()), ?)")
	if err != nil {
		panic("Failed to prepare stmtInsertQueryClass: " + err.Error())
	}

	// UPDATE
	h.stmtUpdateQueryClass, err = h.conns.SQLite.Prepare(
		"UPDATE query_classes" +
			" SET first_seen = LEAST(first_seen, ?), " +
			" last_seen = GREATEST(last_seen, ?)" +
			" WHERE query_class_id = ?")
	if err != nil {
		panic("Failed to prepare stmtUpdateQueryClass: " + err.Error())
	}
}

func (h *MySQLMetricWriter) closeStatements() {
	h.stmtInsertClassMetrics.Close()
	h.stmtInsertQueryExample.Close()
	h.stmtInsertQueryClass.Close()
	h.stmtUpdateQueryClass.Close()
}

// --------------------------------------------------------------------------

var metricColumns []string
var insertGlobalMetrics string
var insertClassMetrics string

func init() {
	nCounters := 0
	for _, m := range metrics.Query {
		if (m.Flags & metrics.COUNTER) == 0 {
			nCounters++
		}
	}
	n := ((len(metrics.Query) - nCounters) * (len(metrics.StatNames) - 1)) + nCounters
	metricColumns = make([]string, n)

	i := 0
	for _, m := range metrics.Query {
		if (m.Flags & metrics.COUNTER) == 0 {
			for _, stat := range metrics.StatNames {
				if stat != "p5" {
					metricColumns[i] = m.Name + "_" + stat
					i++
				}
			}
		} else {
			metricColumns[i] = m.Name + "_sum"
			i++
		}
	}

	insertGlobalMetrics = "INSERT INTO query_global_metrics" +
		" (" + strings.Join(GlobalCols, ",") + "," + strings.Join(metricColumns, ",") + ")" +
		" VALUES (" + shared.Placeholders(len(GlobalCols)+len(metricColumns)) + ")"

	insertClassMetrics = "INSERT INTO query_class_metrics" +
		" (" + strings.Join(ClassCols, ",") + "," + strings.Join(metricColumns, ",") + ")" +
		" VALUES (" + shared.Placeholders(len(ClassCols)+len(metricColumns)) + ")"
}

var GlobalCols []string = []string{
	`instance_id`,
	`start_ts`,
	`end_ts`,
	`run_time`,
	`total_query_count`,
	`unique_query_count`,
	`rate_type`,
	`rate_limit`,
	`log_file`,
	`log_file_size`,
	`start_offset`,
	`end_offset`,
	`stop_offset`,
}

var ClassCols []string = []string{
	`query_class_id`,
	`instance_id`,
	`start_ts`,
	`end_ts`,
	`query_count`,
	`lrq_count`,
}
