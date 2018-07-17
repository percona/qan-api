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

	"github.com/percona/qan-api/app/db/mysql"
)

// report provide methods to works with query report
type report struct{}

// Report instance of report model
var Report = report{}

// QueryRank - represents a row of query profile
type QueryRank struct {
	Rank        uint    // compared to global, same as Profile.Ranks index
	Percentage  float64 // of global value
	ID          string  `json:"Id"` // hex checksum
	Abstract    string  // e.g. SELECT tbl
	Fingerprint string  // e.g. SELECT tbl
	QPS         float64 // ResponseTime.Cnt / Profile.TotalTime
	Load        float64 // Query_time_sum / (Profile.End - Profile.Begin)
	FirstSeen   time.Time
	Log         []QueryLog
	Stats       Stats // this query's Profile.Metric stats
}

// QueryLog - a point of sparkline
type QueryLog struct {
	Point        uint      `db:"Point"`
	StartTs      time.Time `db:"Start_ts" json:"Start_ts"`
	NoData       bool
	QueryCount   float32 `db:"Query_count" json:"Query_count"`
	QueryLoad    float32 `db:"Query_load" json:"Query_load"`
	QueryTimeAvg float32 `db:"Query_time_avg" json:"Query_time_avg"`
}

// RankBy - a createria of calculating queries with lowest performance.
type RankBy struct {
	Metric string // default: Query_time
	Stat   string // default: sum
	Limit  uint   // default: 10
}

// Profile - container for query profile
type Profile struct {
	InstanceID   string      `json:"InstanceId"` // UUID of MySQL instance
	Begin        time.Time   // time range [Begin, End)
	End          time.Time   // time range [Being, End)
	TotalTime    uint        // total seconds in time range minus gaps (missing periods)
	TotalQueries uint        // total unique class queries in time range
	RankBy       RankBy      // criteria for ranking queries compared to global
	Query        []QueryRank // 0=global, 1..N=queries
}

// Stats - perquery statistics
type Stats struct {
	Cnt uint64  `db:"query_count"`
	Sum float64 `db:"query_time_sum"`
	Min float64 `db:"query_time_min"`
	P5  float64
	Avg float64 `db:"query_time_avg"`
	Med float64 `db:"query_time_med"`
	P95 float64 `db:"query_time_p95"`
	Max float64 `db:"query_time_max"`
}

// get data for spark-lines at query profile
const sparkLinesQueryClass = `
	SELECT (:end_ts - UNIX_TIMESTAMP(start_ts)) DIV :interval_ts AS Point,
	FROM_UNIXTIME(:end_ts - (SELECT point) * :interval_ts) AS Start_ts,
	COALESCE(SUM(query_count), 0) AS Query_count,
	COALESCE(SUM(Query_time_sum)/:interval_ts, 0) AS Query_load,
	COALESCE(AVG(Query_time_avg), 0) AS Query_time_avg
	FROM query_class_metrics
	WHERE query_class_id = :query_class_id AND instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end) GROUP BY point;
	`

const sparkLinesQueryGlobal = `
	SELECT (:end_ts - UNIX_TIMESTAMP(start_ts)) DIV :interval_ts AS Point,
	FROM_UNIXTIME(:end_ts - (SELECT point) * :interval_ts) AS Start_ts,
	COALESCE(SUM(total_query_count), 0) AS Query_count,
	COALESCE(SUM(Query_time_sum)/:interval_ts, 0) AS Query_load,
	COALESCE(AVG(Query_time_avg), 0) AS Query_time_avg
	FROM query_global_metrics
	WHERE instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end) GROUP BY point;
	`

// get data for spark-lines at query profile
func (r report) SparklineData(endTs int64, intervalTs int64, queryClassID uint, instanceID uint, begin, end time.Time) []QueryLog {

	queryLogArrRaw := make(map[int64]QueryLog)
	queryLogArr := []QueryLog{}

	args := struct {
		EndTS        int64     `db:"end_ts"`
		IntervalTS   int64     `db:"interval_ts"`
		QueryClassID uint      `db:"query_class_id"`
		InstanceID   uint      `db:"instance_id"`
		Begin        time.Time `db:"begin"`
		End          time.Time `db:"end"`
	}{endTs, intervalTs, queryClassID, instanceID, begin, end}

	query := sparkLinesQueryClass
	// if for sparklines for total
	if queryClassID == 0 {
		query = sparkLinesQueryGlobal
	}

	ql := []QueryLog{}
	nstmtQuery, err := db.PrepareNamed(query)
	if err != nil {
		log.Fatalln(err)
	}
	defer nstmtQuery.Close()
	err = nstmtQuery.Select(&ql, args)
	if err != nil {
		log.Fatalln(err)
	}

	for _, row := range ql {
		queryLogArrRaw[(row.StartTs).Unix()] = row
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
			val = QueryLog{
				Point:   uint(i),
				StartTs: time.Unix(ts, 0).UTC(),
				NoData:  true,
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
		COALESCE(SUM(total_query_count), 0) AS query_count,
		COALESCE(SUM(Query_time_sum), 0) AS query_time_sum,
		COALESCE(MIN(Query_time_min), 0) AS query_time_min,
		COALESCE(SUM(Query_time_sum)/SUM(total_query_count), 0) AS query_time_avg,
		COALESCE(AVG(Query_time_med), 0) AS query_time_med,
		COALESCE(AVG(Query_time_p95), 0) AS query_time_p95,
		COALESCE(MAX(Query_time_max), 0) AS query_time_max
	FROM query_global_metrics
	WHERE instance_id = :instance_id AND start_ts BETWEEN :begin AND :end
`

const queryReportTemplate = `
	SELECT
		qcm.query_class_id AS query_class_id,
		COALESCE(SUM(qcm.query_count), 0) AS query_count,
		COALESCE(SUM(qcm.Query_time_sum), 0) AS query_time_sum,
		COALESCE(MIN(qcm.Query_time_min), 0) AS query_time_min,
		COALESCE(SUM(qcm.Query_time_sum)/SUM(qcm.query_count), 0) AS query_time_avg,
		COALESCE(AVG(qcm.Query_time_med), 0) AS query_time_med,
		COALESCE(AVG(qcm.Query_time_p95), 0) AS query_time_p95,
		COALESCE(MAX(qcm.Query_time_max), 0) AS query_time_max,
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
	LIMIT :limit OFFSET :offset;
`

func (r report) Profile(instanceID uint, begin, end time.Time, rank RankBy, offset int, search string, firstSeen bool) (Profile, error) {
	args := struct {
		InstanceID uint `db:"instance_id"`
		Begin      time.Time
		End        time.Time
		Limit      uint
		Offset     int
		Keyword    string
		FirstSeen  bool
	}{
		InstanceID: instanceID,
		Begin:      begin,
		End:        end,
		Limit:      rank.Limit,
		Offset:     offset,
		Keyword:    search,
		FirstSeen:  firstSeen,
	}
	p := Profile{
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

	nstmtQueryReportCountUnique, err := db.PrepareNamed(queryReportCountUniqueBuffer.String())
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: db.PrepareNamed: SELECT COUNT(DISTINCT query_class_id)")
	}
	defer nstmtQueryReportCountUnique.Close()
	err = nstmtQueryReportCountUnique.Get(&p.TotalQueries, args)
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: nstmt.Get: SELECT COUNT(DISTINCT query_class_id)")
	}

	totalValues := struct {
		TotalTime uint `db:"total_time"`
		Stats
	}{}
	nstmtQueryReportTotal, err := db.PrepareNamed(queryReportTotal)
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: db.PrepareNamed: queryReportTotal")
	}
	defer nstmtQueryReportTotal.Close()
	err = nstmtQueryReportTotal.Get(&totalValues, args)
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: nstmt.Get: queryReportTotal")
	}

	p.TotalTime = totalValues.TotalTime
	s := totalValues.Stats

	// There's always a row because of the aggregate functions, but if there's
	// no data then COALESCE will cause zero time. In this case, return an empty
	// profile so client knows that there's no problem on our end, there's just
	// no data for the given values.
	if p.TotalTime == 0 {
		return p, nil
	}

	globalSum := s.Sum // to calculate Percentage

	qsize := rank.Limit
	if rank.Limit > p.TotalQueries {
		qsize = p.TotalQueries
	}

	p.Query = make([]QueryRank, int64(qsize)+1)
	p.Query[0].Percentage = 1 // 100%
	p.Query[0].Stats = s
	p.Query[0].QPS = float64(s.Cnt) / intervalTime
	p.Query[0].Load = globalSum / intervalTime
	p.Query[0].Log = r.SparklineData(endTs, intervalTs, 0, instanceID, begin, end)

	// Select query profile
	var queryReportBuffer bytes.Buffer
	if tmpl, err := template.New("queryReportSQL").Parse(queryReportTemplate); err != nil {
		log.Fatalln(err)
	} else if err = tmpl.Execute(&queryReportBuffer, args); err != nil {
		log.Fatalln(err)
	}

	type QueryValue struct {
		QueryClassID uint      `db:"query_class_id"`
		Checksum     string    `db:"checksum"`
		Abstract     string    `db:"abstract"`
		Fingerprint  string    `db:"fingerprint"`
		FirstSeen    time.Time `db:"first_seen"`
		Stats
	}
	queriesValues := []QueryValue{}
	nstmtQueryReport, err := db.PrepareNamed(queryReportBuffer.String())
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: db.PrepareNamed: queryReport")
	}
	defer nstmtQueryReport.Close()
	err = nstmtQueryReport.Select(&queriesValues, args)
	if err != nil {
		return p, mysql.Error(err, "Reporter.Profile: nstmt.Select: queryReport")
	}

	for i, row := range queriesValues {
		i++
		qrank := QueryRank{
			Rank:        uint(i),
			Percentage:  row.Stats.Sum / globalSum,
			ID:          row.Checksum,
			Abstract:    row.Abstract,
			Fingerprint: row.Fingerprint,
			FirstSeen:   row.FirstSeen,
			QPS:         float64(row.Stats.Cnt) / intervalTime,
			Load:        row.Stats.Sum / intervalTime,
			Stats:       row.Stats,
		}
		qrank.Log = r.SparklineData(endTs, intervalTs, row.QueryClassID, instanceID, begin, end)
		p.Query[i] = qrank
	}
	return p, nil
}
