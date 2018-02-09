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
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/percona/go-mysql/event"
	"github.com/percona/pmm/proto/metrics"
	qp "github.com/percona/pmm/proto/qan"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/service/query"
	"github.com/percona/qan-api/stats"
)

const (
	MAX_ABSTRACT    = 100  // query_classes.abstract
	MAX_FINGERPRINT = 5000 // query_classes.fingerprint
)

type MySQLMetricWriter struct {
	dbm   db.Manager
	ih    instance.DbHandler
	m     *query.Mini
	stats *stats.Stats
	// --
	stmtSelectClassId       *sql.Stmt
	stmtInsertGlobalMetrics *sql.Stmt
	stmtInsertClassMetrics  *sql.Stmt
	stmtInsertQueryExample  *sql.Stmt
	stmtInsertQueryClass    *sql.Stmt
	stmtUpdateQueryClass    *sql.Stmt
}

func NewMySQLMetricWriter(
	dbm db.Manager,
	ih instance.DbHandler,
	m *query.Mini,
	stats *stats.Stats,
) *MySQLMetricWriter {
	h := &MySQLMetricWriter{
		dbm:   dbm,
		ih:    ih,
		m:     m,
		stats: stats,
		// --
	}
	return h
}

func (h *MySQLMetricWriter) Write(report qp.Report) error {
	var err error

	instanceId, in, err := h.ih.Get(report.UUID)
	if err != nil {
		return fmt.Errorf("cannot get instance of %s: %s", report.UUID, err)
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

	// Internal metrics
	h.stats.SetComponent("db")
	t := time.Now()

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

		id, err := h.getClassId(class.Id)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("WARNING: cannot get query class ID, skipping: %s: %#v: %s", err, class, trace)
			continue
		}

		if id != 0 {
			// Existing class, update it.  These update aren't fatal, but they shouldn't fail.
			var tables string
			var err error
			if in.Subsystem == instance.SubsystemNameMySQL {
				_, tables, err = h.getQueryAndTables(class)
				if err != nil {
					log.Printf("WARNING: cannot parse query to update: %s", err)
				}
			}
			if err := h.updateQueryClass(id, lastSeen, tables); err != nil {
				log.Printf("WARNING: cannot update query class, skipping: %s: %#v: %s", err, class, trace)
				continue
			}
		} else {
			// New class, create it.
			id, err = h.newClass(instanceId, in.Subsystem, class, lastSeen)
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
			if err = h.updateQueryExample(instanceId, class, id, lastSeen); err != nil {
				log.Printf("WARNING: cannot update query example: %s: %#v: %s", err, class, trace)
			}
		}

		vals := h.getMetricValues(class.Metrics)
		classVals := []interface{}{
			id,
			instanceId,
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

	h.stats.TimingDuration(h.stats.System("insert-class-metrics"), time.Now().Sub(t), h.stats.SampleRate)

	if classDupes > 0 {
		log.Printf("WARNING: %s duplicate query class metrics: start_ts='%s': %s", classDupes, report.StartTs, trace)
	}

	// //////////////////////////////////////////////////////////////////////
	// Insert global metrics into query_global_metrics.
	// //////////////////////////////////////////////////////////////////////

	// It's important to do this after class metics to avoid a race condition:
	// QAN profile looks first at global metrics, then gets corresponding class
	// metrics. If we insert global metrics first, QAN might might get global
	// metrics then class metrics before we've inserted the class metrics for
	// the global metrics. This makes QAN show data for the time range but no
	// queries.

	vals := h.getMetricValues(report.Global.Metrics)

	// Use NULL for Percona Server rate limit values unless set.
	var (
		globalRateType  interface{} = nil
		globalRateLimit interface{} = nil
	)
	if report.RateLimit > 1 {
		globalRateType = "query" // only thing we support
		globalRateLimit = report.RateLimit
	}

	// Use NULL for slow log values unless set.
	var (
		slowLogFile     interface{} = nil
		slowLogFileSize interface{} = nil
		startOffset     interface{} = nil
		endOffset       interface{} = nil
		stopOffset      interface{} = nil
	)
	if report.SlowLogFile != "" {
		slowLogFile = report.SlowLogFile
		slowLogFileSize = report.SlowLogFileSize
		startOffset = report.StartOffset
		endOffset = report.EndOffset
		stopOffset = report.StopOffset
	}

	globalVals := []interface{}{
		instanceId,
		report.StartTs,
		report.EndTs,
		report.RunTime,
		report.Global.TotalQueries,
		report.Global.UniqueQueries,
		globalRateType,
		globalRateLimit,
		slowLogFile,
		slowLogFileSize,
		startOffset,
		endOffset,
		stopOffset,
	}

	globalVals = append(globalVals, vals...)
	t = time.Now()
	_, err = h.stmtInsertGlobalMetrics.Exec(globalVals...)
	h.stats.TimingDuration(h.stats.System("insert-global-metrics"), time.Now().Sub(t), h.stats.SampleRate)
	if err != nil {
		if mysql.ErrorCode(err) == mysql.ER_DUP_ENTRY {
			log.Printf("WARNING: duplicate global metrics: start_ts='%s': %s", report.StartTs, trace)
		} else {
			return mysql.Error(err, "writeMetrics insertGlobalMetrics")
		}
	}

	return nil
}

func (h *MySQLMetricWriter) getClassId(checksum string) (uint, error) {
	var classId uint
	if err := h.stmtSelectClassId.QueryRow(checksum).Scan(&classId); err != nil {
		return 0, err
	}
	return classId, nil
}

func (h *MySQLMetricWriter) newClass(instanceId uint, subsystem string, class *event.Class, lastSeen string) (uint, error) {
	var queryAbstract, queryQuery string
	var tables interface{}

	switch subsystem {
	case instance.SubsystemNameMySQL:
		t := time.Now()
		var query query.QueryInfo
		var err error
		query, tables, err = h.getQueryAndTables(class)
		if err != nil {
			return 0, err
		}

		h.stats.TimingDuration(h.stats.System("abstract-fingerprint"), time.Now().Sub(t), h.stats.SampleRate)

		// Truncate long fingerprints and abstracts to avoid MySQL warning 1265:
		// Data truncated for column 'abstract'
		if len(query.Query) > MAX_FINGERPRINT {
			query.Query = query.Query[0:MAX_FINGERPRINT-3] + "..."
		}
		if len(query.Abstract) > MAX_ABSTRACT {
			query.Abstract = query.Abstract[0:MAX_ABSTRACT-3] + "..."
		}
		queryAbstract = query.Abstract
		queryQuery = query.Query
	case instance.SubsystemNameMongo:
		queryAbstract = class.Fingerprint
		queryQuery = class.Fingerprint
	}

	// Create the query class which is internally identified by its query_class_id.
	// The query checksum is the class is identified externally (in a QAN report).
	// Since this is the first time we've seen the query, firstSeen=lastSeen.
	t := time.Now()
	res, err := h.stmtInsertQueryClass.Exec(class.Id, queryAbstract, queryQuery, tables, lastSeen, lastSeen)

	h.stats.TimingDuration(h.stats.System("insert-query-class"), time.Now().Sub(t), h.stats.SampleRate)
	if err != nil {
		if mysql.ErrorCode(err) == mysql.ER_DUP_ENTRY {
			// Duplicate entry; someone else inserted the same server
			// (or caller didn't check first).  Return its server_id.
			return h.getClassId(class.Id)
		} else {
			// Other error, let caller handle.
			return 0, mysql.Error(err, "newClass INSERT query_classes")
		}
	}
	classId, err := res.LastInsertId()
	if err != nil {
		return 0, mysql.Error(err, "newClass res.LastInsertId")
	}

	return uint(classId), nil // success
}

func (h *MySQLMetricWriter) getQueryAndTables(class *event.Class) (query.QueryInfo, string, error) {
	var schema, tables string
	var queryInfo query.QueryInfo
	// Default schema to add to the tables if there is no schema in the query like:
	// SELECT a, b, c FROM table
	if class.Example != nil {
		schema = class.Example.Db
	}
	if len(class.Fingerprint) < 2 {
		return queryInfo, "", fmt.Errorf("empty fingerprint")
	}

	class.Fingerprint = strings.TrimSpace(class.Fingerprint)
	// If we have a query example, that's better to parse than a fingerprint
	queryExample := class.Fingerprint
	if class.Example != nil && class.Example.Query != "" {
		queryExample = class.Example.Query
	}
	query, err := h.m.Parse(queryExample, schema)
	if err != nil {
		return queryInfo, "", err
	}

	if len(query.Tables) > 0 {
		bytes, _ := json.Marshal(query.Tables)
		tables = string(bytes)
	}
	// We still want to store the fingerprint in the database
	// even if an example is available
	query.Query = class.Fingerprint
	return query, tables, nil
}

func (h *MySQLMetricWriter) updateQueryClass(queryClassId uint, lastSeen, tables string) error {
	t := time.Now()
	_, err := h.stmtUpdateQueryClass.Exec(lastSeen, lastSeen, tables, queryClassId)
	h.stats.TimingDuration(h.stats.System("update-query-class"), time.Now().Sub(t), h.stats.SampleRate)
	return mysql.Error(err, "updateQueryClass UPDATE query_classes")
}

func (h *MySQLMetricWriter) updateQueryExample(instanceId uint, class *event.Class, classId uint, lastSeen string) error {
	// INSERT ON DUPLICATE KEY UPDATE
	t := time.Now()
	_, err := h.stmtInsertQueryExample.Exec(instanceId, classId, lastSeen, lastSeen, class.Example.Db, class.Example.QueryTime, class.Example.Query)
	h.stats.TimingDuration(h.stats.System("update-query-example"), time.Now().Sub(t), h.stats.SampleRate)
	return mysql.Error(err, "updateQueryExample INSERT query_examples")
}

func (h *MySQLMetricWriter) getMetricValues(e *event.Metrics) []interface{} {
	t := time.Now()
	defer func() {
		h.stats.TimingDuration(h.stats.System("get-metric-values"), time.Now().Sub(t), h.stats.SampleRate)
	}()

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
						val = stats.Min
					case "max":
						val = stats.Max
					case "avg":
						val = stats.Avg
					case "p95":
						val = stats.P95
					case "med":
						val = stats.Med
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
					val = stats.Min
				case "max":
					val = stats.Max
				case "avg":
					val = stats.Avg
				case "p95":
					val = stats.P95
				case "med":
					val = stats.Med
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
	t := time.Now()
	defer func() {
		h.stats.TimingDuration(h.stats.System("prepare-stmts"), time.Now().Sub(t), h.stats.SampleRate)
	}()

	var err error

	// SELECT
	h.stmtSelectClassId, err = h.dbm.DB().Prepare(
		"SELECT query_class_id" +
			" FROM query_classes" +
			" WHERE checksum = ?")
	if err != nil {
		panic("Failed to prepare stmtSelectClassId:" + err.Error())
	}

	// INSERT
	h.stmtInsertGlobalMetrics, err = h.dbm.DB().Prepare(insertGlobalMetrics)
	if err != nil {
		panic("Failed to prepare stmtInsertGlobalMetrics: " + err.Error())
	}

	h.stmtInsertClassMetrics, err = h.dbm.DB().Prepare(insertClassMetrics)
	if err != nil {
		panic("Failed to prepare stmtInsertClassMetrics: " + err.Error())
	}

	h.stmtInsertQueryExample, err = h.dbm.DB().Prepare(
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
	h.stmtInsertQueryClass, err = h.dbm.DB().Prepare(
		"INSERT INTO query_classes" +
			" (checksum, abstract, fingerprint, tables, first_seen, last_seen)" +
			" VALUES (?, ?, ?, ?, COALESCE(?, NOW()), ?)")
	if err != nil {
		panic("Failed to prepare stmtInsertQueryClass: " + err.Error())
	}

	// UPDATE
	h.stmtUpdateQueryClass, err = h.dbm.DB().Prepare(
		"UPDATE query_classes" +
			" SET first_seen = LEAST(first_seen, ?), " +
			" last_seen = GREATEST(last_seen, ?), " +
			" tables = ? " +
			" WHERE query_class_id = ?")
	if err != nil {
		panic("Failed to prepare stmtUpdateQueryClass: " + err.Error())
	}
}

func (h *MySQLMetricWriter) closeStatements() {
	h.stmtSelectClassId.Close()
	h.stmtInsertGlobalMetrics.Close()
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
