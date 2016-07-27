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
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/percona/pmm/proto/metrics"
	qp "github.com/percona/pmm/proto/qan"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/stats"
)

type Reporter struct {
	dbm   db.Manager
	stats *stats.Stats
}

func NewReporter(dbm db.Manager, stats *stats.Stats) *Reporter {
	qr := &Reporter{
		dbm:   dbm,
		stats: stats,
	}
	return qr
}

func (qr *Reporter) Profile(instanceId uint, begin, end time.Time, r qp.RankBy, offset int) (qp.Profile, error) {
	intervalTime := end.Sub(begin).Seconds()

	stats := make([]string, len(metrics.StatNames)-1)
	i := 0
	for _, stat := range metrics.StatNames {
		if stat == "p5" {
			continue
		}
		stats[i] = metrics.AggregateFunction(r.Metric, stat, "total_query_count")
		i++
	}

	countUnique := "SELECT COUNT(DISTINCT query_class_id) " +
		"FROM query_class_metrics WHERE instance_id = ? " +
		"AND (start_ts >= ? AND start_ts < ?);"

	p := qp.Profile{
		// caller sets InstanceId (MySQL instance UUID)
		Begin:  begin,
		End:    end,
		RankBy: r,
	}

	err := qr.dbm.DB().QueryRow(countUnique, instanceId, begin, end).Scan(
		&p.TotalQueries,
	)
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: SELECT COUNT(DISTINCT query_class_id)")
	}

	q := "SELECT COALESCE(SUM(TIMESTAMPDIFF(SECOND, start_ts, end_ts)), 0), COALESCE(SUM(total_query_count), 0), " + strings.Join(stats, ", ") +
		" FROM query_global_metrics" +
		" WHERE instance_id = ? AND (start_ts >= ? AND start_ts < ?)"

	s := metrics.Stats{}
	err = qr.dbm.DB().QueryRow(q, instanceId, begin, end).Scan(
		&p.TotalTime,
		&s.Cnt,
		&s.Sum,
		&s.Min,
		&s.Avg,
		&s.Med,
		&s.P95,
		&s.Max,
	)
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: SELECT query_global_metrics")
	}

	// get data for spark-lines at query profile
	sparkLinesQueryGlobal := "SELECT start_ts, total_query_count, Query_time_sum " +
		" FROM query_global_metrics " +
		" WHERE instance_id = ? AND (start_ts >= ? AND start_ts < ?) LIMIT 180"

	sparkLinesRows, sparkLinesErr := qr.dbm.DB().Query(sparkLinesQueryGlobal, instanceId, begin, end)
	if sparkLinesErr != nil {
		return p, mysql.Error(err, "Reporter.Profile: SELECT query_global_metrics")
	}

	defer sparkLinesRows.Close()
	queryLogArr := []qp.QueryLog{}
	for sparkLinesRows.Next() {
		ql := qp.QueryLog{}
		sparkLinesRows.Scan(
			&ql.Start_ts,
			&ql.Query_count,
			&ql.Query_time_sum,
		)
		queryLogArr = append(queryLogArr, ql)
	}

	// There's always a row because of the aggregate functions, but if there's
	// no data then COALESCE will cause zero time. In this case, return an empty
	// profile so client knows that there's no problem on our end, there's just
	// no data for the given values.
	if p.TotalTime == 0 {
		return p, nil
	}

	totalTime := float64(p.TotalTime) // to calculate QPS
	globalSum := s.Sum.Float64        // to calculate Percentage

	p.Query = make([]qp.QueryRank, int64(r.Limit)+1)
	p.Query[0].Stats = s
	p.Query[0].QPS = float64(s.Cnt) / totalTime
	p.Query[0].Load = s.Sum.Float64 / intervalTime
	p.Query[0].Log = queryLogArr

	i = 0
	for _, stat := range metrics.StatNames {
		if stat == "p5" {
			continue
		}
		stats[i] = metrics.AggregateFunction(r.Metric, stat, "query_count")
		i++
	}
	q = fmt.Sprintf(
		"SELECT query_class_id, SUM(query_count), "+strings.Join(stats, ", ")+
			" FROM query_class_metrics"+
			" WHERE instance_id = ? AND (start_ts >= ? AND start_ts < ?)"+
			" GROUP BY query_class_id"+
			" ORDER BY %s DESC"+
			" LIMIT %d OFFSET %d ",
		metrics.AggregateFunction(r.Metric, r.Stat, "query_count"),
		r.Limit,
		offset,
	)

	rows, err := qr.dbm.DB().Query(q, instanceId, begin, end)
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: SELECT query_class_metrics")
	}
	defer rows.Close()

	// get data for spark-lines at query profile
	sparkLinesQueryClass := "SELECT start_ts, query_count, Query_time_sum " +
		" FROM query_class_metrics " +
		" WHERE query_class_id = ? and instance_id = ? AND (start_ts >= ? AND start_ts < ?) LIMIT 180"

	var queryClassId uint
	query := map[uint]int{}
	queryClassIds := []interface{}{}
	rank := 1
	for rows.Next() {
		r := qp.QueryRank{
			Rank:  uint(rank),
			Stats: metrics.Stats{},
		}
		err := rows.Scan(
			&queryClassId,
			&r.Stats.Cnt,
			&r.Stats.Sum,
			&r.Stats.Min,
			&r.Stats.Avg,
			&r.Stats.Med,
			&r.Stats.P95,
			&r.Stats.Max,
		)
		if err != nil {
			return p, mysql.Error(err, "Reporter.Profile: SELECT query_class_metrics")
		}
		r.Percentage = r.Stats.Sum.Float64 / globalSum
		r.QPS = float64(r.Stats.Cnt) / totalTime
		r.Load = r.Stats.Sum.Float64 / intervalTime

		sparkLinesRows, sparkLinesErr := qr.dbm.DB().Query(sparkLinesQueryClass, queryClassId, instanceId, begin, end)
		if sparkLinesErr != nil {
			return p, mysql.Error(err, "Reporter.Profile: SELECT query_class_metrics")
		}
		defer sparkLinesRows.Close()
		queryLogArr := []qp.QueryLog{}
		for sparkLinesRows.Next() {
			ql := qp.QueryLog{}
			sparkLinesRows.Scan(
				&ql.Start_ts,
				&ql.Query_count,
				&ql.Query_time_sum,
			)
			queryLogArr = append(queryLogArr, ql)
		}

		r.Log = queryLogArr
		p.Query[rank] = r
		query[queryClassId] = rank
		queryClassIds = append(queryClassIds, queryClassId)

		rank++
	}

	// https://jira.percona.com/browse/PPL-109
	if len(queryClassIds) == 0 {
		return p, fmt.Errorf("bug PPL-109: no query classes for selected instance and time range: %d %s %s %s %d",
			instanceId,
			begin,
			end,
			metrics.AggregateFunction(r.Metric, r.Stat, "query_count"),
			r.Limit,
		)
	}

	p.Query = p.Query[0:rank] // remove unused ranks, if any

	q = "SELECT query_class_id, checksum, abstract" +
		" FROM query_classes" +
		" WHERE query_class_id IN (" + shared.Placeholders(len(queryClassIds)) + ")"

	rows, err = qr.dbm.DB().Query(q, queryClassIds...)
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: SELECT query_classes")
	}
	defer rows.Close()

	var checksum, abstract string
	for rows.Next() {
		err := rows.Scan(
			&queryClassId,
			&checksum,
			&abstract,
		)
		if err != nil {
			return p, mysql.Error(err, "Reporter.Profile: SELECT query_classes")
		}
		rank := query[queryClassId]
		p.Query[rank].Id = checksum
		p.Query[rank].Abstract = abstract
	}

	return p, nil
}
