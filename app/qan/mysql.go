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

	_ "github.com/go-sql-driver/mysql" // register mysql driver
	"github.com/percona/go-mysql/event"
	"github.com/percona/pmm/proto/metrics"
	qp "github.com/percona/pmm/proto/qan"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/service/query"
	"github.com/revel/revel"
)

const (
	maxAbstract    = 100  // query_classes.abstract
	maxFingerprint = 5000 // query_classes.fingerprint
)

type MySQLMetricWriter struct {
	conns *models.ConnectionsPool
	m     *query.Mini
	// --
	// stmtInsertClassMetrics *sqlx.Stmt
	// stmtInsertQueryExample *sqlx.Stmt
	// stmtInsertQueryClass   *sqlx.Stmt
	// stmtUpdateQueryClass   *sqlx.Stmt
}

const queryInsertClassMetrics = `
	INSERT INTO query_class_metrics
	(EventDate, query_class_id, instance_id, start_ts, end_ts, query_count, lrq_count, Query_time_sum, Query_time_min, Query_time_avg,
	Query_time_med, Query_time_p95, Query_time_max, Lock_time_sum, Lock_time_min, Lock_time_avg, Lock_time_med,
	Lock_time_p95, Lock_time_max, Rows_sent_sum, Rows_sent_min, Rows_sent_avg, Rows_sent_med, Rows_sent_p95, Rows_sent_max,
	Rows_examined_sum, Rows_examined_min, Rows_examined_avg, Rows_examined_med, Rows_examined_p95, Rows_examined_max,
	Rows_affected_sum, Rows_affected_min, Rows_affected_avg, Rows_affected_med, Rows_affected_p95, Rows_affected_max,
	Bytes_sent_sum, Bytes_sent_min, Bytes_sent_avg, Bytes_sent_med, Bytes_sent_p95, Bytes_sent_max, Tmp_tables_sum,
	Tmp_tables_min, Tmp_tables_avg, Tmp_tables_med, Tmp_tables_p95, Tmp_tables_max, Tmp_disk_tables_sum,
	Tmp_disk_tables_min, Tmp_disk_tables_avg, Tmp_disk_tables_med, Tmp_disk_tables_p95, Tmp_disk_tables_max,
	Tmp_table_sizes_sum, Tmp_table_sizes_min, Tmp_table_sizes_avg, Tmp_table_sizes_med, Tmp_table_sizes_p95,
	Tmp_table_sizes_max, QC_Hit_sum, Full_scan_sum, Full_join_sum, Tmp_table_sum, Tmp_table_on_disk_sum, Filesort_sum,
	Filesort_on_disk_sum, Merge_passes_sum, Merge_passes_min, Merge_passes_avg, Merge_passes_med, Merge_passes_p95,
	Merge_passes_max, InnoDB_IO_r_ops_sum, InnoDB_IO_r_ops_min, InnoDB_IO_r_ops_avg, InnoDB_IO_r_ops_med,
	InnoDB_IO_r_ops_p95, InnoDB_IO_r_ops_max, InnoDB_IO_r_bytes_sum, InnoDB_IO_r_bytes_min, InnoDB_IO_r_bytes_avg,
	InnoDB_IO_r_bytes_med, InnoDB_IO_r_bytes_p95, InnoDB_IO_r_bytes_max, InnoDB_IO_r_wait_sum, InnoDB_IO_r_wait_min,
	InnoDB_IO_r_wait_avg, InnoDB_IO_r_wait_med, InnoDB_IO_r_wait_p95, InnoDB_IO_r_wait_max, InnoDB_rec_lock_wait_sum,
	InnoDB_rec_lock_wait_min, InnoDB_rec_lock_wait_avg, InnoDB_rec_lock_wait_med, InnoDB_rec_lock_wait_p95,
	InnoDB_rec_lock_wait_max, InnoDB_queue_wait_sum, InnoDB_queue_wait_min, InnoDB_queue_wait_avg, InnoDB_queue_wait_med,
	InnoDB_queue_wait_p95, InnoDB_queue_wait_max, InnoDB_pages_distinct_sum, InnoDB_pages_distinct_min,
	InnoDB_pages_distinct_avg, InnoDB_pages_distinct_med, InnoDB_pages_distinct_p95, InnoDB_pages_distinct_max,
	Errors_sum, Warnings_sum, Select_full_range_join_sum, Select_range_sum, Select_range_check_sum, Sort_range_sum,
	Sort_rows_sum, Sort_scan_sum, No_index_used_sum, No_good_index_used_sum, Query_length_sum, Query_length_min,
	Query_length_avg, Query_length_med, Query_length_p95, Query_length_max)
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,
				?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,
				?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
`

// const queryInsertClassMetrics = `INSERT INTO query_class_metrics (EventDate, query_class_id, instance_id, start_ts, end_ts, query_count, lrq_count, Query_time_sum, Query_time_min, Query_time_avg, Query_time_med, Query_time_p95, Query_time_max, Lock_time_sum, Lock_time_min, Lock_time_avg, Lock_time_med, Lock_time_p95, Lock_time_max, Rows_sent_sum, Rows_sent_min, Rows_sent_avg, Rows_sent_med, Rows_sent_p95, Rows_sent_max, Rows_examined_sum, Rows_examined_min, Rows_examined_avg, Rows_examined_med, Rows_examined_p95, Rows_examined_max, Rows_affected_sum, Rows_affected_min, Rows_affected_avg, Rows_affected_med, Rows_affected_p95, Rows_affected_max, Bytes_sent_sum, Bytes_sent_min, Bytes_sent_avg, Bytes_sent_med, Bytes_sent_p95, Bytes_sent_max, Tmp_tables_sum, Tmp_tables_min, Tmp_tables_avg, Tmp_tables_med, Tmp_tables_p95, Tmp_tables_max, Tmp_disk_tables_sum, Tmp_disk_tables_min, Tmp_disk_tables_avg, Tmp_disk_tables_med, Tmp_disk_tables_p95, Tmp_disk_tables_max, Tmp_table_sizes_sum, Tmp_table_sizes_min, Tmp_table_sizes_avg, Tmp_table_sizes_med, Tmp_table_sizes_p95, Tmp_table_sizes_max, QC_Hit_sum, Full_scan_sum, Full_join_sum, Tmp_table_sum, Tmp_table_on_disk_sum, Filesort_sum, Filesort_on_disk_sum, Merge_passes_sum, Merge_passes_min, Merge_passes_avg, Merge_passes_med, Merge_passes_p95, Merge_passes_max, InnoDB_IO_r_ops_sum, InnoDB_IO_r_ops_min, InnoDB_IO_r_ops_avg, InnoDB_IO_r_ops_med, InnoDB_IO_r_ops_p95, InnoDB_IO_r_ops_max, InnoDB_IO_r_bytes_sum, InnoDB_IO_r_bytes_min, InnoDB_IO_r_bytes_avg, InnoDB_IO_r_bytes_med, InnoDB_IO_r_bytes_p95, InnoDB_IO_r_bytes_max, InnoDB_IO_r_wait_sum, InnoDB_IO_r_wait_min, InnoDB_IO_r_wait_avg, InnoDB_IO_r_wait_med, InnoDB_IO_r_wait_p95, InnoDB_IO_r_wait_max, InnoDB_rec_lock_wait_sum, InnoDB_rec_lock_wait_min, InnoDB_rec_lock_wait_avg, InnoDB_rec_lock_wait_med, InnoDB_rec_lock_wait_p95, InnoDB_rec_lock_wait_max, InnoDB_queue_wait_sum, InnoDB_queue_wait_min, InnoDB_queue_wait_avg, InnoDB_queue_wait_med, InnoDB_queue_wait_p95, InnoDB_queue_wait_max, InnoDB_pages_distinct_sum, InnoDB_pages_distinct_min, InnoDB_pages_distinct_avg, InnoDB_pages_distinct_med, InnoDB_pages_distinct_p95, InnoDB_pages_distinct_max, Errors_sum, Warnings_sum, Select_full_range_join_sum, Select_range_sum, Select_range_check_sum, Sort_range_sum, Sort_rows_sum, Sort_scan_sum, No_index_used_sum, No_good_index_used_sum, Query_length_sum, Query_length_min, Query_length_avg, Query_length_med, Query_length_p95, Query_length_max) VALUES (today(), 7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7, 7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7);`

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

	// h.prepareStatements()
	// defer h.closeStatements()

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
			// if err := h.updateQueryClass(id, lastSeen); err != nil {
			// 	log.Printf("WARNING: cannot update query class, skipping: %s: %#v: %s", err, class, trace)
			// 	continue
			// }
			const queryUpdateQueryClass = `
			UPDATE query_classes 
				SET 
					first_seen = (CASE WHEN first_seen < ? THEN first_seen ELSE ? END),
					last_seen = (CASE WHEN last_seen > ? THEN last_seen ELSE ? END),
					tables=IF(tables='', ?, tables)
				WHERE query_class_id = ?;

			`
			_, err = h.conns.SQLite.Exec(queryUpdateQueryClass, lastSeen, lastSeen, lastSeen, lastSeen, id)
			if err != nil {
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
			// if err = h.updateQueryExample(instanceID, class, id, lastSeen); err != nil {
			// 	log.Printf("WARNING: cannot update query example: %s: %#v: %s", err, class, trace)
			// }

			// TODO:
			const queryInsertQueryExample = `
				REPLACE INTO query_examples (instance_id, query_class_id, period, ts, db, Query_time, query)
				VALUES (?, ?, DATE(?), ?, ?, ?, ?)
			`
			_, err = h.conns.SQLite.Exec(queryInsertQueryExample, instanceID, id, lastSeen, lastSeen, class.Example.Db, class.Example.QueryTime, class.Example.Query)
			if err != nil {
				log.Printf("WARNING: cannot update query example: %s: %#v: %s", err, class, trace)
			}

		}

		vals := h.getMetricValues(class.Metrics, fromSlowLog)
		classVals := []interface{}{
			time.Now().Format("2006-01-02"),
			id,
			instanceID,
			report.StartTs,
			report.EndTs,
			class.TotalQueries,
			0, // todo: `lrq_count`,
		}
		classVals = append(classVals, vals...)

		// INSERT query_class_metrics
		// _, err = h.stmtInsertClassMetrics.Exec(classVals...)

		tx, err := h.conns.ClickHouse.Begin()
		defer func() {
			if err != nil {
				tx.Rollback()
				return
			}
			tx.Commit()
		}()

		if err != nil {
			revel.ERROR.Printf("Cannot begin transaction for clickhouse: %v", err)
		}

		queryClassVals := []interface{}{}
		for _, v := range classVals {
			if v == nil {
				queryClassVals = append(queryClassVals, 0)
			} else {
				queryClassVals = append(queryClassVals, v)
			}
		}

		_, err = tx.Exec(queryInsertClassMetrics, queryClassVals...)
		// _, err = tx.Exec(queryInsertClassMetrics)

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
	const queryInsertQueryClass = `
		INSERT INTO query_classes
		(checksum, abstract, fingerprint, tables, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE 
		tables=IF(tables='', ?, tables)
	`
	res, err := h.conns.SQLite.Exec(queryInsertQueryClass, class.Id, queryAbstract, queryQuery, tables, lastSeen, lastSeen, tables)
	// res, err := h.stmtInsertQueryClass.Exec(class.Id, queryAbstract, queryQuery, tables, lastSeen, lastSeen)

	if err != nil {
		revel.ERROR.Printf("============== queryInsertQueryClass: %v", err)

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

// func (h *MySQLMetricWriter) updateQueryClass(queryClassId uint, lastSeen string) error {
// 	_, err := h.stmtUpdateQueryClass.Exec(lastSeen, lastSeen, queryClassId)
// 	return mysql.Error(err, "updateQueryClass UPDATE query_classes")
// }

// func (h *MySQLMetricWriter) updateQueryExample(instanceID uint, class *event.Class, classId uint, lastSeen string) error {
// 	// INSERT ON DUPLICATE KEY UPDATE
// 	_, err := h.stmtInsertQueryExample.Exec(instanceID, classId, lastSeen, lastSeen, class.Example.Db, class.Example.QueryTime, class.Example.Query)
// 	return mysql.Error(err, "updateQueryExample INSERT query_examples")
// }

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
				// var val interface{} = nil
				val := float64(0)
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
			// var val interface{} = nil
			val := uint64(0)
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

// func (h *MySQLMetricWriter) prepareStatements() {
// var err error

// INSERT

// h.stmtInsertClassMetrics, err = h.conns.ClickHouse.Prepare(insertClassMetrics)
// h.stmtInsertClassMetrics, err = h.conns.ClickHouse.Preparex(queryClassMetrics)
// if err != nil {
// 	panic("Failed to prepare stmtInsertClassMetrics: " + err.Error())
// }

// h.stmtInsertQueryExample, err = h.conns.SQLite.Preparex(
// 	`REPLACE INTO query_examples (instance_id, query_class_id, period, ts, db, Query_time, query)
// 		VALUES (?, ?, DATE(?), ?, ?, ?, ?)
// 	`)
// if err != nil {
// 	panic("Failed to prepare stmtInsertQueryExample: " + err.Error())
// }

/* Why use LEAST and GREATEST and update first_seen?
   Because of the asynchronous nature of agents communication, we can receive
   the same query from 2 different agents but it isn't madatory that the first
   one we receive, is the older one. There could have been a network error on
   the agent having the oldest data
*/
// h.stmtInsertQueryClass, err = h.conns.SQLite.Preparex(
// 	`INSERT INTO query_classes
// 		(checksum, abstract, fingerprint, tables, first_seen, last_seen)
// 		 VALUES (?, ?, ?, ?, IFNULL(?, NOW()), ?)
// 	`)
// if err != nil {
// 	panic("Failed to prepare stmtInsertQueryClass: " + err.Error())
// }

// UPDATE
// h.stmtUpdateQueryClass, err = h.conns.SQLite.Preparex(
// 	`UPDATE query_classes
// 		SET first_seen = LEAST(first_seen, ?),
// 		last_seen = GREATEST(last_seen, ?)
// 		WHERE query_class_id = ?
// 	`)
// if err != nil {
// 	panic("Failed to prepare stmtUpdateQueryClass: " + err.Error())
// }
// }

// func (h *MySQLMetricWriter) closeStatements() {
// 	h.stmtInsertClassMetrics.Close()
// 	h.stmtInsertQueryExample.Close()
// 	h.stmtInsertQueryClass.Close()
// 	h.stmtUpdateQueryClass.Close()
// }

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
