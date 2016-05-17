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

package metrics

import (
	"errors"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/percona/pmm/proto/metrics"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/qan-api/stats"
)

var basicMetrics []string      // four universal metrics
var psMetrics []string         // basic + Percona Server
var perfSchemaMetrics []string // some basic + Performance Schema

var (
	ErrNoMetrics = errors.New("no metrics exist")
)

func init() {
	basicMetrics = []string{}
	psMetrics = []string{}
	for _, metric := range metrics.Query {
		// Universal metrics
		if (metric.Flags & metrics.UNIVERSAL) != 0 {
			for _, stat := range metrics.StatNames {
				if stat == "p5" {
					continue // no 5th percentile for query metrics
				}
				basicMetrics = append(basicMetrics, metrics.AggregateFunction(metric.Name, stat, "query_count"))
				psMetrics = append(psMetrics, metrics.AggregateFunction(metric.Name, stat, "query_count"))
			}
		}

		// Percona Server metrics
		if (metric.Flags & metrics.PERCONA_SERVER) != 0 {
			if (metric.Flags & metrics.COUNTER) != 0 {
				// Counter metrics have only sum.
				psMetrics = append(psMetrics, metrics.AggregateFunction(metric.Name, "sum", "query_count"))
			} else {
				for _, stat := range metrics.StatNames {
					if stat == "p5" {
						continue // no 5th percentile for query metrics
					}
					psMetrics = append(psMetrics, metrics.AggregateFunction(metric.Name, stat, "query_count"))
				}
			}
		}
	}

	perfSchemaMetrics = []string{}
	for _, metric := range metrics.Query {
		if ((metric.Flags & metrics.UNIVERSAL) == 0) && ((metric.Flags & metrics.PERF_SCHEMA) == 0) {
			continue
		}
		if metric.Name == "Quey_time" {
			perfSchemaMetrics = append(perfSchemaMetrics, metrics.AggregateFunction(metric.Name, "sum", "query_count"))
			perfSchemaMetrics = append(perfSchemaMetrics, metrics.AggregateFunction(metric.Name, "min", "query_count"))
			perfSchemaMetrics = append(perfSchemaMetrics, metrics.AggregateFunction(metric.Name, "avg", "query_count"))
			perfSchemaMetrics = append(perfSchemaMetrics, metrics.AggregateFunction(metric.Name, "max", "query_count"))
		} else {
			perfSchemaMetrics = append(perfSchemaMetrics, metrics.AggregateFunction(metric.Name, "sum", "query_count"))
		}
	}
}

type QueryMetricsHandler struct {
	dbm   db.Manager
	stats *stats.Stats
}

func NewQueryMetricsHandler(dbm db.Manager, stats *stats.Stats) *QueryMetricsHandler {
	h := &QueryMetricsHandler{
		dbm:   dbm,
		stats: stats,
	}
	return h
}

func (h *QueryMetricsHandler) Get(instanceId, classId uint, begin, end time.Time) (map[string]metrics.Stats, error) {
	// First determine which group of query metrics exist, if any: basic (the
	// four universal in all distros and versions), Percona Server, or
	// Performance Schema.

	basic, ps, perfSchema, err := h.checkMetricGroups(instanceId, begin, end)
	if err != nil {
		return nil, err
	}

	// The basic metrics are universal, so if there are none, then no data was
	// collected (or stored yet) for the given query and time range.
	if !basic {
		return nil, ErrNoMetrics
	}

	// Perf Schema metrics are a smaller and less consistent subset of metrics,
	// so we handle them specially.
	if perfSchema {
		return h.getPerfSchema(instanceId, classId, begin, end)
	}

	// The Percona Server metrics are a superset of the four universal slow
	// log metrics. The list is long so we handle them in a separate func.
	if ps {
		return h.getPerconaServer(instanceId, classId, begin, end)
	}

	// We have only the four universal slow log metrics.
	var cnt uint64
	queryTime := metrics.Stats{}
	lockTime := metrics.Stats{}
	rowsSent := metrics.Stats{}
	rowsExamined := metrics.Stats{}

	// todo: handle no results, cnt will be null
	query := "SELECT SUM(query_count), " + strings.Join(basicMetrics, ", ") +
		" FROM query_class_metrics" +
		" WHERE query_class_id = ? AND instance_id = ? AND (start_ts >= ? AND start_ts < ?)"

	err = h.dbm.DB().QueryRow(query, classId, instanceId, begin, end).Scan(
		&cnt,
		&queryTime.Sum,
		&queryTime.Min,
		&queryTime.Avg,
		&queryTime.Med,
		&queryTime.P95,
		&queryTime.Max,
		&lockTime.Sum,
		&lockTime.Min,
		&lockTime.Avg,
		&lockTime.Med,
		&lockTime.P95,
		&lockTime.Max,
		&rowsSent.Sum,
		&rowsSent.Min,
		&rowsSent.Avg,
		&rowsSent.Med,
		&rowsSent.P95,
		&rowsSent.Max,
		&rowsExamined.Sum,
		&rowsExamined.Min,
		&rowsExamined.Avg,
		&rowsExamined.Med,
		&rowsExamined.P95,
		&rowsExamined.Max,
	)
	if err != nil {
		return nil, err
	}

	queryTime.Cnt = cnt
	lockTime.Cnt = cnt
	rowsSent.Cnt = cnt
	rowsExamined.Cnt = cnt

	s := map[string]metrics.Stats{
		"Query_time":    queryTime,
		"Lock_time":     lockTime,
		"Rows_sent":     rowsSent,
		"Rows_examined": rowsExamined,
	}

	return s, nil
}

func (h *QueryMetricsHandler) Summary(instanceId uint, begin, end time.Time) (map[string]metrics.Stats, error) {
	// First determine which group of query metrics exist, if any: basic (the
	// four universal in all distros and versions), Percona Server, or
	// Performance Schema.

	basic, ps, perfSchema, err := h.checkMetricGroups(instanceId, begin, end)
	if err != nil {
		return nil, err
	}

	// The basic metrics are universal, so if there are none, then no data was
	// collected (or stored yet) for the given query and time range.
	if !basic {
		return nil, ErrNoMetrics
	}

	// Perf Schema metrics are a smaller and less consistent subset of metrics,
	// so we handle them specially.
	if perfSchema {
		return h.getPerfSchemaSummary(instanceId, begin, end)
	}

	// The Percona Server metrics are a superset of the four universal slow
	// log metrics. The list is long so we handle them in a separate func.
	if ps {
		return h.getPerconaServerSummary(instanceId, begin, end)
	}

	// We have only the four universal slow log metrics.
	var cnt uint64
	queryTime := metrics.Stats{}
	lockTime := metrics.Stats{}
	rowsSent := metrics.Stats{}
	rowsExamined := metrics.Stats{}

	// todo: handle no results, cnt will be null
	query := "SELECT SUM(query_count), " + strings.Join(basicMetrics, ", ") +
		" FROM query_class_metrics" +
		" WHERE instance_id = ? AND (start_ts >= ? AND start_ts < ?)"

	err = h.dbm.DB().QueryRow(query, instanceId, begin, end).Scan(
		&cnt,
		&queryTime.Sum,
		&queryTime.Min,
		&queryTime.Avg,
		&queryTime.Med,
		&queryTime.P95,
		&queryTime.Max,
		&lockTime.Sum,
		&lockTime.Min,
		&lockTime.Avg,
		&lockTime.Med,
		&lockTime.P95,
		&lockTime.Max,
		&rowsSent.Sum,
		&rowsSent.Min,
		&rowsSent.Avg,
		&rowsSent.Med,
		&rowsSent.P95,
		&rowsSent.Max,
		&rowsExamined.Sum,
		&rowsExamined.Min,
		&rowsExamined.Avg,
		&rowsExamined.Med,
		&rowsExamined.P95,
		&rowsExamined.Max,
	)
	if err != nil {
		return nil, err
	}

	queryTime.Cnt = cnt
	lockTime.Cnt = cnt
	rowsSent.Cnt = cnt
	rowsExamined.Cnt = cnt

	s := map[string]metrics.Stats{
		"Query_time":    queryTime,
		"Lock_time":     lockTime,
		"Rows_sent":     rowsSent,
		"Rows_examined": rowsExamined,
	}

	return s, nil
}

func (h *QueryMetricsHandler) checkMetricGroups(instanceId uint, begin, end time.Time) (bool, bool, bool, error) {
	q := "(SELECT 1 FROM query_global_metrics" +
		" WHERE instance_id = ? AND (start_ts >= ? AND start_ts < ?)" +
		" AND Query_time_sum IS NOT NULL" +
		" LIMIT 1)" +
		" UNION" +
		" (SELECT 2 FROM query_global_metrics" +
		" WHERE instance_id = ? AND (start_ts >= ? AND start_ts < ?)" +
		" AND Rows_affected_sum IS NOT NULL" +
		" LIMIT 1)" +
		" UNION" +
		" (SELECT 3 FROM query_global_metrics" +
		" WHERE instance_id = ? AND (start_ts >= ? AND start_ts < ?)" +
		" AND Errors_sum IS NOT NULL" +
		" LIMIT 1)"
	rows, err := h.dbm.DB().Query(q, instanceId, begin, end, instanceId, begin, end, instanceId, begin, end)
	if err != nil {
		return false, false, false, mysql.Error(err, "checkMetricGroups: SELECT query_global_metrics")
	}
	defer rows.Close()

	var basic, ps, perfSchema bool
	for rows.Next() {
		var n int
		err := rows.Scan(&n)
		if err != nil {
			return false, false, false, mysql.Error(err, "checkMetricGroups: SELECT query_global_metrics")
		}
		switch n {
		case 1:
			basic = true
		case 2:
			ps = true
		case 3:
			perfSchema = true
		default:
			panic("checkMetricGroups query selected an invalid number")
		}
	}
	return basic, ps, perfSchema, nil
}

func (h *QueryMetricsHandler) getPerfSchema(instanceId, classId uint, begin, end time.Time) (map[string]metrics.Stats, error) {
	return nil, nil
}

func (h *QueryMetricsHandler) getPerfSchemaSummary(instanceId uint, begin, end time.Time) (map[string]metrics.Stats, error) {
	return nil, nil
}

func (h *QueryMetricsHandler) getPerconaServer(instanceId, classId uint, begin, end time.Time) (map[string]metrics.Stats, error) {
	var cnt uint64
	query_time := metrics.Stats{}
	lock_time := metrics.Stats{}
	rows_sent := metrics.Stats{}
	rows_examined := metrics.Stats{}
	rows_affected := metrics.Stats{}
	bytes_sent := metrics.Stats{}
	tmp_tables := metrics.Stats{}
	tmp_disk_tables := metrics.Stats{}
	tmp_table_sizes := metrics.Stats{}
	qc_hit := metrics.Stats{}
	full_scan := metrics.Stats{}
	full_join := metrics.Stats{}
	tmp_table := metrics.Stats{}
	tmp_table_on_disk := metrics.Stats{}
	filesort := metrics.Stats{}
	filesort_on_disk := metrics.Stats{}
	merge_passes := metrics.Stats{}
	innodb_io_r_ops := metrics.Stats{}
	innodb_io_r_bytes := metrics.Stats{}
	innodb_io_r_wait := metrics.Stats{}
	innodb_rec_lock_wait := metrics.Stats{}
	innodb_queue_wait := metrics.Stats{}
	innodb_pages_distinct := metrics.Stats{}

	// todo: handle no results, cnt will be null
	q := "SELECT SUM(query_count), " + strings.Join(psMetrics, ", ") +
		" FROM query_class_metrics" +
		" WHERE query_class_id = ? AND instance_id = ? AND (start_ts >= ? AND start_ts < ?)"
	err := h.dbm.DB().QueryRow(q, classId, instanceId, begin, end).Scan(
		&cnt,
		&query_time.Sum,
		&query_time.Min,
		&query_time.Avg,
		&query_time.Med,
		&query_time.P95,
		&query_time.Max,
		&lock_time.Sum,
		&lock_time.Min,
		&lock_time.Avg,
		&lock_time.Med,
		&lock_time.P95,
		&lock_time.Max,
		&rows_sent.Sum,
		&rows_sent.Min,
		&rows_sent.Avg,
		&rows_sent.Med,
		&rows_sent.P95,
		&rows_sent.Max,
		&rows_examined.Sum,
		&rows_examined.Min,
		&rows_examined.Avg,
		&rows_examined.Med,
		&rows_examined.P95,
		&rows_examined.Max,
		&rows_affected.Sum,
		&rows_affected.Min,
		&rows_affected.Avg,
		&rows_affected.Med,
		&rows_affected.P95,
		&rows_affected.Max,
		&bytes_sent.Sum,
		&bytes_sent.Min,
		&bytes_sent.Avg,
		&bytes_sent.Med,
		&bytes_sent.P95,
		&bytes_sent.Max,
		&tmp_tables.Sum,
		&tmp_tables.Min,
		&tmp_tables.Avg,
		&tmp_tables.Med,
		&tmp_tables.P95,
		&tmp_tables.Max,
		&tmp_disk_tables.Sum,
		&tmp_disk_tables.Min,
		&tmp_disk_tables.Avg,
		&tmp_disk_tables.Med,
		&tmp_disk_tables.P95,
		&tmp_disk_tables.Max,
		&tmp_table_sizes.Sum,
		&tmp_table_sizes.Min,
		&tmp_table_sizes.Avg,
		&tmp_table_sizes.Med,
		&tmp_table_sizes.P95,
		&tmp_table_sizes.Max,
		&qc_hit.Sum,
		&full_scan.Sum,
		&full_join.Sum,
		&tmp_table.Sum,
		&tmp_table_on_disk.Sum,
		&filesort.Sum,
		&filesort_on_disk.Sum,
		&merge_passes.Sum,
		&merge_passes.Min,
		&merge_passes.Avg,
		&merge_passes.Med,
		&merge_passes.P95,
		&merge_passes.Max,
		&innodb_io_r_ops.Sum,
		&innodb_io_r_ops.Min,
		&innodb_io_r_ops.Avg,
		&innodb_io_r_ops.Med,
		&innodb_io_r_ops.P95,
		&innodb_io_r_ops.Max,
		&innodb_io_r_bytes.Sum,
		&innodb_io_r_bytes.Min,
		&innodb_io_r_bytes.Avg,
		&innodb_io_r_bytes.Med,
		&innodb_io_r_bytes.P95,
		&innodb_io_r_bytes.Max,
		&innodb_io_r_wait.Sum,
		&innodb_io_r_wait.Min,
		&innodb_io_r_wait.Avg,
		&innodb_io_r_wait.Med,
		&innodb_io_r_wait.P95,
		&innodb_io_r_wait.Max,
		&innodb_rec_lock_wait.Sum,
		&innodb_rec_lock_wait.Min,
		&innodb_rec_lock_wait.Avg,
		&innodb_rec_lock_wait.Med,
		&innodb_rec_lock_wait.P95,
		&innodb_rec_lock_wait.Max,
		&innodb_queue_wait.Sum,
		&innodb_queue_wait.Min,
		&innodb_queue_wait.Avg,
		&innodb_queue_wait.Med,
		&innodb_queue_wait.P95,
		&innodb_queue_wait.Max,
		&innodb_pages_distinct.Sum,
		&innodb_pages_distinct.Min,
		&innodb_pages_distinct.Avg,
		&innodb_pages_distinct.Med,
		&innodb_pages_distinct.P95,
		&innodb_pages_distinct.Max,
	)
	if err != nil {
		return nil, err
	}

	// We always have the four universal metircs.
	query_time.Cnt = cnt
	lock_time.Cnt = cnt
	rows_sent.Cnt = cnt
	rows_examined.Cnt = cnt
	s := map[string]metrics.Stats{
		"Query_time":    query_time,
		"Lock_time":     lock_time,
		"Rows_sent":     rows_sent,
		"Rows_examined": rows_examined,
	}

	// Include Percona Server metrics that aren't NULL.
	if rows_affected.Sum.Valid {
		rows_affected.Cnt = cnt
		s["Rows_affected"] = rows_affected
	}
	if bytes_sent.Sum.Valid {
		bytes_sent.Cnt = cnt
		s["Bytes_sent"] = bytes_sent
	}
	if tmp_tables.Sum.Valid {
		tmp_tables.Cnt = cnt
		s["Tmp_tables"] = tmp_tables
	}
	if tmp_disk_tables.Sum.Valid {
		tmp_disk_tables.Cnt = cnt
		s["Tmp_disk_tables"] = tmp_disk_tables
	}
	if tmp_table_sizes.Sum.Valid {
		tmp_table_sizes.Cnt = cnt
		s["Tmp_table_sizes"] = tmp_table_sizes
	}
	if qc_hit.Sum.Valid {
		qc_hit.Cnt = cnt
		s["QC_Hit"] = qc_hit
	}
	if full_scan.Sum.Valid {
		full_scan.Cnt = cnt
		s["Full_scan"] = full_scan
	}
	if full_join.Sum.Valid {
		full_join.Cnt = cnt
		s["Full_join"] = full_join
	}
	if tmp_table.Sum.Valid {
		tmp_table.Cnt = cnt
		s["Tmp_table"] = tmp_table
	}
	if tmp_table_on_disk.Sum.Valid {
		tmp_table_on_disk.Cnt = cnt
		s["Tmp_table_on_disk"] = tmp_table_on_disk
	}
	if filesort.Sum.Valid {
		filesort.Cnt = cnt
		s["Filesort"] = filesort
	}
	if filesort_on_disk.Sum.Valid {
		filesort_on_disk.Cnt = cnt
		s["Filesort_on_disk"] = filesort_on_disk
	}
	if merge_passes.Sum.Valid {
		merge_passes.Cnt = cnt
		s["Merge_passes"] = merge_passes
	}
	if innodb_io_r_ops.Sum.Valid {
		innodb_io_r_ops.Cnt = cnt
		s["InnoDB_IO_r_ops"] = innodb_io_r_ops
	}
	if innodb_io_r_bytes.Sum.Valid {
		innodb_io_r_bytes.Cnt = cnt
		s["InnoDB_IO_r_bytes"] = innodb_io_r_bytes
	}
	if innodb_io_r_wait.Sum.Valid {
		innodb_io_r_wait.Cnt = cnt
		s["InnoDB_IO_r_wait"] = innodb_io_r_wait
	}
	if innodb_rec_lock_wait.Sum.Valid {
		innodb_rec_lock_wait.Cnt = cnt
		s["InnoDB_rec_lock_wait"] = innodb_rec_lock_wait
	}
	if innodb_queue_wait.Sum.Valid {
		innodb_queue_wait.Cnt = cnt
		s["InnoDB_queue_wait"] = innodb_queue_wait
	}
	if innodb_pages_distinct.Sum.Valid {
		innodb_pages_distinct.Cnt = cnt
		s["InnoDB_pages_distinct"] = innodb_pages_distinct
	}

	return s, nil
}

func (h *QueryMetricsHandler) getPerconaServerSummary(instanceId uint, begin, end time.Time) (map[string]metrics.Stats, error) {
	var cnt uint64
	query_time := metrics.Stats{}
	lock_time := metrics.Stats{}
	rows_sent := metrics.Stats{}
	rows_examined := metrics.Stats{}
	rows_affected := metrics.Stats{}
	bytes_sent := metrics.Stats{}
	tmp_tables := metrics.Stats{}
	tmp_disk_tables := metrics.Stats{}
	tmp_table_sizes := metrics.Stats{}
	qc_hit := metrics.Stats{}
	full_scan := metrics.Stats{}
	full_join := metrics.Stats{}
	tmp_table := metrics.Stats{}
	tmp_table_on_disk := metrics.Stats{}
	filesort := metrics.Stats{}
	filesort_on_disk := metrics.Stats{}
	merge_passes := metrics.Stats{}
	innodb_io_r_ops := metrics.Stats{}
	innodb_io_r_bytes := metrics.Stats{}
	innodb_io_r_wait := metrics.Stats{}
	innodb_rec_lock_wait := metrics.Stats{}
	innodb_queue_wait := metrics.Stats{}
	innodb_pages_distinct := metrics.Stats{}

	// todo: handle no results, cnt will be null
	q := "SELECT SUM(query_count), " + strings.Join(psMetrics, ", ") +
		" FROM query_class_metrics" +
		" WHERE instance_id = ? AND (start_ts >= ? AND start_ts < ?)"
	err := h.dbm.DB().QueryRow(q, instanceId, begin, end).Scan(
		&cnt,
		&query_time.Sum,
		&query_time.Min,
		&query_time.Avg,
		&query_time.Med,
		&query_time.P95,
		&query_time.Max,
		&lock_time.Sum,
		&lock_time.Min,
		&lock_time.Avg,
		&lock_time.Med,
		&lock_time.P95,
		&lock_time.Max,
		&rows_sent.Sum,
		&rows_sent.Min,
		&rows_sent.Avg,
		&rows_sent.Med,
		&rows_sent.P95,
		&rows_sent.Max,
		&rows_examined.Sum,
		&rows_examined.Min,
		&rows_examined.Avg,
		&rows_examined.Med,
		&rows_examined.P95,
		&rows_examined.Max,
		&rows_affected.Sum,
		&rows_affected.Min,
		&rows_affected.Avg,
		&rows_affected.Med,
		&rows_affected.P95,
		&rows_affected.Max,
		&bytes_sent.Sum,
		&bytes_sent.Min,
		&bytes_sent.Avg,
		&bytes_sent.Med,
		&bytes_sent.P95,
		&bytes_sent.Max,
		&tmp_tables.Sum,
		&tmp_tables.Min,
		&tmp_tables.Avg,
		&tmp_tables.Med,
		&tmp_tables.P95,
		&tmp_tables.Max,
		&tmp_disk_tables.Sum,
		&tmp_disk_tables.Min,
		&tmp_disk_tables.Avg,
		&tmp_disk_tables.Med,
		&tmp_disk_tables.P95,
		&tmp_disk_tables.Max,
		&tmp_table_sizes.Sum,
		&tmp_table_sizes.Min,
		&tmp_table_sizes.Avg,
		&tmp_table_sizes.Med,
		&tmp_table_sizes.P95,
		&tmp_table_sizes.Max,
		&qc_hit.Sum,
		&full_scan.Sum,
		&full_join.Sum,
		&tmp_table.Sum,
		&tmp_table_on_disk.Sum,
		&filesort.Sum,
		&filesort_on_disk.Sum,
		&merge_passes.Sum,
		&merge_passes.Min,
		&merge_passes.Avg,
		&merge_passes.Med,
		&merge_passes.P95,
		&merge_passes.Max,
		&innodb_io_r_ops.Sum,
		&innodb_io_r_ops.Min,
		&innodb_io_r_ops.Avg,
		&innodb_io_r_ops.Med,
		&innodb_io_r_ops.P95,
		&innodb_io_r_ops.Max,
		&innodb_io_r_bytes.Sum,
		&innodb_io_r_bytes.Min,
		&innodb_io_r_bytes.Avg,
		&innodb_io_r_bytes.Med,
		&innodb_io_r_bytes.P95,
		&innodb_io_r_bytes.Max,
		&innodb_io_r_wait.Sum,
		&innodb_io_r_wait.Min,
		&innodb_io_r_wait.Avg,
		&innodb_io_r_wait.Med,
		&innodb_io_r_wait.P95,
		&innodb_io_r_wait.Max,
		&innodb_rec_lock_wait.Sum,
		&innodb_rec_lock_wait.Min,
		&innodb_rec_lock_wait.Avg,
		&innodb_rec_lock_wait.Med,
		&innodb_rec_lock_wait.P95,
		&innodb_rec_lock_wait.Max,
		&innodb_queue_wait.Sum,
		&innodb_queue_wait.Min,
		&innodb_queue_wait.Avg,
		&innodb_queue_wait.Med,
		&innodb_queue_wait.P95,
		&innodb_queue_wait.Max,
		&innodb_pages_distinct.Sum,
		&innodb_pages_distinct.Min,
		&innodb_pages_distinct.Avg,
		&innodb_pages_distinct.Med,
		&innodb_pages_distinct.P95,
		&innodb_pages_distinct.Max,
	)
	if err != nil {
		return nil, err
	}

	// We always have the four universal metircs.
	query_time.Cnt = cnt
	lock_time.Cnt = cnt
	rows_sent.Cnt = cnt
	rows_examined.Cnt = cnt
	s := map[string]metrics.Stats{
		"Query_time":    query_time,
		"Lock_time":     lock_time,
		"Rows_sent":     rows_sent,
		"Rows_examined": rows_examined,
	}

	// Include Percona Server metrics that aren't NULL.
	if rows_affected.Sum.Valid {
		rows_affected.Cnt = cnt
		s["Rows_affected"] = rows_affected
	}
	if bytes_sent.Sum.Valid {
		bytes_sent.Cnt = cnt
		s["Bytes_sent"] = bytes_sent
	}
	if tmp_tables.Sum.Valid {
		tmp_tables.Cnt = cnt
		s["Tmp_tables"] = tmp_tables
	}
	if tmp_disk_tables.Sum.Valid {
		tmp_disk_tables.Cnt = cnt
		s["Tmp_disk_tables"] = tmp_disk_tables
	}
	if tmp_table_sizes.Sum.Valid {
		tmp_table_sizes.Cnt = cnt
		s["Tmp_table_sizes"] = tmp_table_sizes
	}
	if qc_hit.Sum.Valid {
		qc_hit.Cnt = cnt
		s["QC_Hit"] = qc_hit
	}
	if full_scan.Sum.Valid {
		full_scan.Cnt = cnt
		s["Full_scan"] = full_scan
	}
	if full_join.Sum.Valid {
		full_join.Cnt = cnt
		s["Full_join"] = full_join
	}
	if tmp_table.Sum.Valid {
		tmp_table.Cnt = cnt
		s["Tmp_table"] = tmp_table
	}
	if tmp_table_on_disk.Sum.Valid {
		tmp_table_on_disk.Cnt = cnt
		s["Tmp_table_on_disk"] = tmp_table_on_disk
	}
	if filesort.Sum.Valid {
		filesort.Cnt = cnt
		s["Filesort"] = filesort
	}
	if filesort_on_disk.Sum.Valid {
		filesort_on_disk.Cnt = cnt
		s["Filesort_on_disk"] = filesort_on_disk
	}
	if merge_passes.Sum.Valid {
		merge_passes.Cnt = cnt
		s["Merge_passes"] = merge_passes
	}
	if innodb_io_r_ops.Sum.Valid {
		innodb_io_r_ops.Cnt = cnt
		s["InnoDB_IO_r_ops"] = innodb_io_r_ops
	}
	if innodb_io_r_bytes.Sum.Valid {
		innodb_io_r_bytes.Cnt = cnt
		s["InnoDB_IO_r_bytes"] = innodb_io_r_bytes
	}
	if innodb_io_r_wait.Sum.Valid {
		innodb_io_r_wait.Cnt = cnt
		s["InnoDB_IO_r_wait"] = innodb_io_r_wait
	}
	if innodb_rec_lock_wait.Sum.Valid {
		innodb_rec_lock_wait.Cnt = cnt
		s["InnoDB_rec_lock_wait"] = innodb_rec_lock_wait
	}
	if innodb_queue_wait.Sum.Valid {
		innodb_queue_wait.Cnt = cnt
		s["InnoDB_queue_wait"] = innodb_queue_wait
	}
	if innodb_pages_distinct.Sum.Valid {
		innodb_pages_distinct.Cnt = cnt
		s["InnoDB_pages_distinct"] = innodb_pages_distinct
	}

	return s, nil
}
