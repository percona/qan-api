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
	"log"
	"text/template"
	"time"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/db/mysql"

	mp "github.com/percona/pmm/proto/metrics"
	qp "github.com/percona/pmm/proto/qan"
)

// report provide methods to works with query report
type report struct{}

// Report instance of report model
var Report = report{}

// get data for spark-lines at query profile
const sparkLinesQueryClass = `
	SELECT (:end_ts - UNIX_TIMESTAMP(start_ts)) DIV :interval_ts AS Point,
	FROM_UNIXTIME(:end_ts - (SELECT point) * :interval_ts) AS Start_ts,
	SUM(query_count) AS Query_count,
	SUM(Query_time_sum)/:interval_ts AS Query_load,
	AVG(Query_time_avg) AS Query_time_avg
	FROM query_class_metrics
	WHERE query_class_id = :query_class_id AND instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end) GROUP BY point;
	`

const sparkLinesQueryGlobal = `
	SELECT (:end_ts - UNIX_TIMESTAMP(start_ts)) DIV :interval_ts AS Point,
	FROM_UNIXTIME(:end_ts - (SELECT point) * :interval_ts) AS Start_ts,
	SUM(total_query_count) AS Query_count,
	SUM(Query_time_sum)/:interval_ts AS Query_load,
	AVG(Query_time_avg) AS Query_time_avg
	FROM query_global_metrics
	WHERE instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end) GROUP BY point;
	`

// get data for spark-lines at query profile
func (r report) SparklineData(endTs int64, intervalTs int64, queryClassId uint, instanceId uint, begin, end time.Time) []qp.QueryLog {

	queryLogArrRaw := make(map[int64]qp.QueryLog)
	queryLogArr := []qp.QueryLog{}

	args := struct {
		EndTS        int64     `db:"end_ts"`
		IntervalTS   int64     `db:"interval_ts"`
		QueryClassID uint      `db:"query_class_id"`
		InstanceID   uint      `db:"instance_id"`
		Begin        time.Time `db:"begin"`
		End          time.Time `db:"end"`
	}{endTs, intervalTs, queryClassId, instanceId, begin, end}

	query := sparkLinesQueryClass
	// if for sparklines for total
	if queryClassId == 0 {
		query = sparkLinesQueryGlobal
	}

	ql := []qp.QueryLog{}
	if nstmt, err := db.PrepareNamed(query); err != nil {
		log.Fatalln(err)
	} else if err = nstmt.Select(&ql, args); err != nil {
		log.Fatalln(err)
	}

	for _, row := range ql {
		queryLogArrRaw[(row.Start_ts).Unix()] = row
	}

	intervalTimeMinutes := end.Sub(begin).Minutes()
	amountOfPoints := int64(maxAmountOfPoints)
	if intervalTimeMinutes < maxAmountOfPoints {
		amountOfPoints = int64(intervalTimeMinutes)
	}
	var i int64
	for i = 0; i < amountOfPoints; i++ {
		ts := endTs - i*intervalTs
		val, ok := queryLogArrRaw[ts]

		// skip first or last point if they are empty
		if (i == 0 || i == amountOfPoints-1) && !ok {
			continue
		}

		if !ok {
			val = qp.QueryLog{
				Point:    uint(i),
				Start_ts: time.Unix(ts, 0).UTC(),
			}
		}
		queryLogArr = append(queryLogArr, val)
	}
	return queryLogArr
}

const queryReportCountUniqueTemplate = `
	SELECT
		COUNT(DISTINCT qcm.query_class_id)
	FROM query_class_metrics AS qcm
	JOIN query_classes AS qc ON qcm.query_class_id = qc.query_class_id
	WHERE qcm.instance_id = :instance_id AND (qcm.start_ts >= :begin AND qcm.start_ts < :end)
		{{ if .FirstSeen }} AND qc.first_seen >= :begin {{ end }}
		{{ if .Keyword }} AND (qc.checksum = ':keyword' OR qc.abstract LIKE '%:keyword' OR qc.fingerprint LIKE '%:keyword') {{ end }};
`

const queryReportTotal = `
	SELECT
		COALESCE(SUM(TIMESTAMPDIFF(SECOND, start_ts, end_ts)), 0) AS total_time,
		COALESCE(SUM(total_query_count), 0) AS total_query_count,
		SUM(Query_time_sum) AS query_time_sum,
		MIN(Query_time_min) AS query_time_min,
		SUM(Query_time_sum)/SUM(total_query_count) AS query_time_avg,
		AVG(Query_time_med) AS query_time_med,
		AVG(Query_time_p95) AS query_time_p95,
		MAX(Query_time_max) AS query_time_max
	FROM query_global_metrics
	WHERE instance_id = :instance_id AND start_ts BETWEEN :begin AND :end
`

const queryReportTemplate = `
	SELECT
		qcm.query_class_id AS query_class_id,
		SUM(qcm.query_count) AS query_count,
		SUM(qcm.Query_time_sum) AS query_time_sum,
		MIN(qcm.Query_time_min) AS query_time_min,
		SUM(qcm.Query_time_sum)/SUM(qcm.query_count) AS query_time_avg,
		AVG(qcm.Query_time_med) AS query_time_med,
		AVG(qcm.Query_time_p95) AS query_time_p95,
		MAX(qcm.Query_time_max) AS query_time_max,
		qc.checksum AS checksum,
		qc.abstract AS abstract,
		qc.fingerprint AS fingerprint,
		qc.first_seen AS first_seen
	FROM query_class_metrics AS qcm
	JOIN query_classes AS qc ON qcm.query_class_id = qc.query_class_id
	WHERE qcm.instance_id = :instance_id AND (qcm.start_ts >= :begin AND qcm.start_ts < :end)
		{{ if .FirstSeen }} AND qc.first_seen >= :begin {{ end }}
		{{ if .Keyword }} AND (qc.checksum = ':keyword' OR qc.abstract LIKE '%:keyword' OR qc.fingerprint LIKE '%:keyword') {{ end }}
	GROUP BY qcm.query_class_id
	ORDER BY SUM(qcm.Query_time_sum) DESC
	LIMIT 10 OFFSET :offset;
`

func (r report) Profile(instanceID uint, begin, end time.Time, rank qp.RankBy, offset int, search string, firstSeen bool) (qp.Profile, error) {
	args := struct {
		InstanceID uint `db:"instance_id"`
		Begin      time.Time
		End        time.Time
		Offset     int
		Keyword    string
		FirstSeen  bool
	}{
		InstanceID: instanceID,
		Begin:      begin,
		End:        end,
		Offset:     offset,
		Keyword:    search,
		FirstSeen:  firstSeen,
	}
	p := qp.Profile{
		// caller sets InstanceId (MySQL instance UUID)
		Begin:  begin,
		End:    end,
		RankBy: rank,
	}

	intervalTime := end.Sub(begin).Seconds()
	intervalTimeMinutes := end.Sub(begin).Minutes()
	endTs := end.Unix()
	amountOfPoints := int64(maxAmountOfPoints)
	if intervalTimeMinutes < maxAmountOfPoints {
		amountOfPoints = int64(intervalTimeMinutes)
	}

	intervalTs := int64(end.Sub(begin).Seconds()) / amountOfPoints

	// get count of all rows - to calculate pagination.
	var queryReportCountUniqueBuffer bytes.Buffer
	if tmpl, err := template.New("queryReportCountUniqueSQL").Parse(queryReportCountUniqueTemplate); err != nil {
		log.Fatalln(err)
	} else if err = tmpl.Execute(&queryReportCountUniqueBuffer, args); err != nil {
		log.Fatalln(err)
	}

	if nstmt, err := db.PrepareNamed(queryReportCountUniqueBuffer.String()); err != nil {
		return p, mysql.Error(err, "Reporter.Profile: db.PrepareNamed: SELECT COUNT(DISTINCT query_class_id)")
	} else if err = nstmt.Get(&p.TotalQueries, args); err != nil {
		return p, mysql.Error(err, "Reporter.Profile: nstmt.Get: SELECT COUNT(DISTINCT query_class_id)")
	}

	s := mp.Stats{}
	totalValues := struct {
		TotalTime       uint              `db:"total_time"`
		TotalQueryCount uint64            `db:"total_query_count"`
		QueryTimeSum    proto.NullFloat64 `db:"query_time_sum"`
		QueryTimeMin    proto.NullFloat64 `db:"query_time_min"`
		QueryTimeAvg    proto.NullFloat64 `db:"query_time_avg"`
		QueryTimeMed    proto.NullFloat64 `db:"query_time_med"`
		QueryTimeP95    proto.NullFloat64 `db:"query_time_p95"`
		QueryTimeMax    proto.NullFloat64 `db:"query_time_max"`
	}{}
	if nstmt, err := db.PrepareNamed(queryReportTotal); err != nil {
		return p, mysql.Error(err, "Reporter.Profile: db.PrepareNamed: queryReportTotal")
	} else if err = nstmt.Get(&totalValues, args); err != nil {
		return p, mysql.Error(err, "Reporter.Profile: nstmt.Get: queryReportTotal")
	}
	p.TotalTime = totalValues.TotalTime
	s.Cnt = totalValues.TotalQueryCount
	s.Sum = totalValues.QueryTimeSum
	s.Min = totalValues.QueryTimeMin
	s.Avg = totalValues.QueryTimeAvg
	s.Med = totalValues.QueryTimeMed
	s.P95 = totalValues.QueryTimeP95
	s.Max = totalValues.QueryTimeMax

	// There's always a row because of the aggregate functions, but if there's
	// no data then COALESCE will cause zero time. In this case, return an empty
	// profile so client knows that there's no problem on our end, there's just
	// no data for the given values.
	if p.TotalTime == 0 {
		return p, nil
	}

	globalSum := s.Sum.Float64 // to calculate Percentage

	p.Query = make([]qp.QueryRank, int64(rank.Limit)+1)
	p.Query[0].Percentage = 1 // 100%
	p.Query[0].Stats = s
	p.Query[0].QPS = float64(s.Cnt) / intervalTime
	p.Query[0].Load = s.Sum.Float64 / intervalTime
	p.Query[0].Log = r.SparklineData(endTs, intervalTs, 0, instanceID, begin, end)

	// Select query profile

	var queryReportBuffer bytes.Buffer
	if tmpl, err := template.New("queryReportSQL").Parse(queryReportTemplate); err != nil {
		log.Fatalln(err)
	} else if err = tmpl.Execute(&queryReportBuffer, args); err != nil {
		log.Fatalln(err)
	}

	type QueryValue struct {
		QueryClassID uint              `db:"query_class_id"`
		QueryCount   uint64            `db:"query_count"`
		QueryTimeSum proto.NullFloat64 `db:"query_time_sum"`
		QueryTimeMin proto.NullFloat64 `db:"query_time_min"`
		QueryTimeAvg proto.NullFloat64 `db:"query_time_avg"`
		QueryTimeMed proto.NullFloat64 `db:"query_time_med"`
		QueryTimeP95 proto.NullFloat64 `db:"query_time_p95"`
		QueryTimeMax proto.NullFloat64 `db:"query_time_max"`
		Checksum     string            `db:"checksum"`
		Abstract     string            `db:"abstract"`
		Fingerprint  string            `db:"fingerprint"`
		FirstSeen    time.Time         `db:"first_seen"`
	}
	queriesValues := []QueryValue{}
	if nstmt, err := db.PrepareNamed(queryReportBuffer.String()); err != nil {
		return p, mysql.Error(err, "Reporter.Profile: db.PrepareNamed: queryReport")
	} else if err = nstmt.Select(&queriesValues, args); err != nil {
		return p, mysql.Error(err, "Reporter.Profile: nstmt.Select: queryReport")
	}

	for i, row := range queriesValues {
		i++
		qrank := qp.QueryRank{
			Rank:        uint(i),
			Percentage:  row.QueryTimeSum.Float64 / globalSum,
			Id:          row.Checksum,
			Abstract:    row.Abstract,
			Fingerprint: row.Fingerprint,
			FirstSeen:   row.FirstSeen,
			QPS:         float64(row.QueryCount) / intervalTime,
			Load:        row.QueryTimeSum.Float64 / intervalTime,
			Stats: mp.Stats{
				Cnt: row.QueryCount,
				Sum: row.QueryTimeSum,
				Min: row.QueryTimeMin,
				Avg: row.QueryTimeAvg,
				Med: row.QueryTimeMed,
				P95: row.QueryTimeP95,
				Max: row.QueryTimeMax,
			},
		}
		qrank.Log = r.SparklineData(endTs, intervalTs, row.QueryClassID, instanceID, begin, end)
		p.Query[i] = qrank
	}
	return p, nil
}
