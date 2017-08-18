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
	"fmt"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql" // registers mysql driver
	"github.com/percona/qan-api/app/db/mysql"
)

type (

	// QueryReportManager contains methods to get query analytics and reports.;
	QueryReportManager struct {
		conns *ConnectionsPool
	}

	// Profile represents query class profile of db instance.
	Profile struct {
		InstanceID   string      `json:"InstanceId"` // UUID of MySQL instance
		Begin        time.Time   // time range [Begin, End)
		End          time.Time   // time range [Being, End)
		TotalTime    uint        // total seconds in time range minus gaps (missing periods)
		TotalQueries uint        // total unique class queries in time range
		RankBy       RankBy      // criteria for ranking queries compared to global
		Query        []QueryRank // 0=global, 1..N=queries
	}

	// RankBy is criteria for ranking queries compared to global
	RankBy struct {
		Metric string // default: Query_time
		Stat   string // default: sum
		Limit  uint   // default: 10
	}

	// QueryLog is sparkline points for query class profile.
	QueryLog struct {
		Point        uint      `json:"Point" db:"point"`
		StartTS      time.Time `json:"Start_ts" db:"point_time"`
		QueryCount   uint      `json:"Query_count" db:"query_count"`
		QueryTimeSum float32   `json:"Query_time_sum" db:"query_time_sum"`
		QueryTimeAvg float32   `json:"Query_time_avg" db:"query_time_avg"`
	}

	// QueryRank is query class representation
	QueryRank struct {
		Rank        uint    // compared to global, same as Profile.Ranks index
		Percentage  float64 // of global value
		ID          string  `json:"Id"` // hex checksum
		Abstract    string  // e.g. SELECT tbl
		Fingerprint string  // e.g. SELECT tbl
		QPS         float64 // ResponseTime.Cnt / Profile.TotalTime
		Load        float64 // Query_time_sum / (Profile.End - Profile.Begin)
		Log         *[]QueryLog
		Stats       Stats // this query's Profile.Metric stats
	}

	// Stats is statistical measurement of values variations
	Stats struct {
		// TotalTime uint64 // TODO: is it used anywhere?
		Cnt float64
		Sum float64
		Min float64
		Avg float64
		Med float64
		P95 float64
		Max float64
	}
)

// NewQueryReportManager returns InstanceManager with db connections pool.
func NewQueryReportManager(conns interface{}) *QueryReportManager {
	connsPool := conns.(*ConnectionsPool)
	return &QueryReportManager{connsPool}
}

// SparklineData get data for spark-lines at query profile
func (qrm *QueryReportManager) SparklineData(endSeconds int64, intervalTS int64, queryClassID uint, instanceID uint, begin, end time.Time) (*[]QueryLog, error) {
	// get data for spark-lines at query profile
	const queryProfileSparklinesTemplate = `
		SELECT
			intDiv((:end_seconds - toRelativeSecondNum(start_ts)), :interval_ts) as point,
			toDateTime(:end_seconds - point * (:interval_ts)) AS point_time,
			SUM(query_count) AS query_count,
			SUM(Query_time_sum) AS query_time_sum,
			AVG(Query_time_avg) AS query_time_avg
		FROM query_class_metrics
		WHERE {{if not .IsTotal }} query_class_id = :query_class_id AND {{ end }}
			instance_id = :instance_id AND (start_ts >= (:begin) AND start_ts < :end) GROUP BY point;
	`

	args := struct {
		QueryClassID uint      `db:"query_class_id"`
		InstanceID   uint      `db:"instance_id"`
		Begin        time.Time `db:"begin"`
		End          time.Time `db:"end"`
		IntervalTS   int64     `db:"interval_ts"`
		EndSeconds   int64     `db:"end_seconds"`
	}{queryClassID, instanceID, begin, end, intervalTS, endSeconds}

	// is it TOTAL or ugular query class in top 10 query profile report.
	typeOfQueryProfile := struct {
		IsTotal bool
	}{queryClassID == 0}
	var querySparklinesBuffer bytes.Buffer

	tmpl, err := template.New("querySparklinesSQL").Parse(queryProfileSparklinesTemplate)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse 'querySparklinesSQL' template (%v)", err)
	}
	err = tmpl.Execute(&querySparklinesBuffer, typeOfQueryProfile)
	if err != nil {
		return nil, fmt.Errorf("Cannot execute 'querySparklinesSQL' template (%v)", err)
	}
	var sparksWithGaps []QueryLog

	nstmt, err := qrm.conns.ClickHouse.PrepareNamed(querySparklinesBuffer.String())
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare named 'querySparklinesBuffer.String()' (%v)", err)
	}
	err = nstmt.Select(&sparksWithGaps, args)
	if err != nil {
		return nil, fmt.Errorf("Cannot select 'querySparklinesSQL' (%v)", err)
	}
	queryLogArrRaw := make(map[int64]QueryLog)

	for i := range sparksWithGaps {
		key := sparksWithGaps[i].StartTS.Unix()
		queryLogArrRaw[key] = sparksWithGaps[i]
	}

	var i int64
	queryLogArr := []QueryLog{}
	for i = 0; i < amountOfPoints; i++ {
		ts := endSeconds - i*intervalTS
		val, ok := queryLogArrRaw[ts]
		if !ok {
			val = QueryLog{
				Point:   uint(i),
				StartTS: time.Unix(ts, 0).UTC(),
			}
		}
		queryLogArr = append(queryLogArr, val)
	}
	return &queryLogArr, nil
}

// Profile selects query classes profile for given db instance.
func (qrm *QueryReportManager) Profile(instanceID uint, begin, end time.Time, rankBy RankBy, offset int, search string) (*Profile, error) {

	profile := Profile{
		// caller sets InstanceId (MySQL instance UUID)
		Begin:  begin,
		End:    end,
		RankBy: rankBy,
	}

	var filterQueryClassIDs []uint
	if search != "" {
		filterQueryClassIDs, err := qrm.filterByFingerprint(instanceID, begin, end, search)
		if err != nil {
			return &profile, err
		}
		if len(*filterQueryClassIDs) == 0 {
			return &profile, mysql.Error(nil, "None of the queries, in selected time range, do not contain this substring.")
		}
	}

	args := struct {
		InstanceID          uint      `db:"instance_id"`
		Begin               time.Time `db:"start_ts"`
		End                 time.Time `db:"end_ts"`
		Offset              int       `db:"offset"`
		Limit               uint      `db:"limit"`
		FilterQueryClassIDs []uint    `db:"query_class_ids"`
	}{instanceID, begin, end, offset, rankBy.Limit, filterQueryClassIDs}

	// select global queries
	const queryCountUniqueTemplate = `
		SELECT COUNT(DISTINCT query_class_id)
		FROM query_class_metrics
		WHERE instance_id = :instance_id AND (start_ts >= (:start_ts) AND start_ts < (:end_ts) )
		{{ if .FilterQueryClassIDs }}
			AND query_class_id IN ({{ range $index, $element := .FilterQueryClassIDs}}{{if $index}}, {{end}}{{$element}}{{end}})
		{{end}}
	`

	tmpl, err := template.New("countUnique").Parse(queryCountUniqueTemplate)
	if err != nil {
		return &profile, err
	}

	var queryCountUniqueBuffer bytes.Buffer
	err = tmpl.Execute(&queryCountUniqueBuffer, args)
	if err != nil {
		return &profile, nil
	}

	nstmt, err := qrm.conns.ClickHouse.PrepareNamed(queryCountUniqueBuffer.String())
	if err != nil {
		return &profile, err
	}
	err = nstmt.Get(&profile.TotalQueries, args)
	if err != nil {
		return &profile, fmt.Errorf("Reporter.Profile: SELECT COUNT(DISTINCT query_class_id) %v", err)
	}

	const queryTotalProfile = `
		SELECT 
			SUM(end_ts - start_ts) AS total_time,
			SUM(query_count) AS cnt,
			SUM(Query_time_sum) AS sum,
			MIN(Query_time_min) AS min,
			SUM(Query_time_sum)/SUM(query_count) AS avg,
			AVG(Query_time_p95) AS p95,
			MAX(Query_time_max) AS max
		FROM query_class_metrics
		WHERE instance_id = :instance_id AND (start_ts >= :start_ts AND start_ts < :end_ts)
	`

	nstmt, err = qrm.conns.ClickHouse.PrepareNamed(queryTotalProfile)
	if err != nil {
		return nil, err
	}

	totalStats := struct {
		TotalTime uint64 `db:"total_time"`
		Stats
	}{Stats: Stats{}}
	err = nstmt.Get(&totalStats, args)
	if err != nil {
		return &profile, fmt.Errorf("Reporter.Profile: SELECT query_global_metrics (%v)", err)
	}

	// There's always a row because of the aggregate functions, but if there's
	// no data then COALESCE will cause zero time. In this case, return an empty
	// profile so client knows that there's no problem on our end, there's just
	// no data for the given values.
	// if totalStats.TotalTime == 0 {
	if totalStats.Cnt == 0 {
		return &profile, nil
	}

	intervalTime := end.Sub(begin).Seconds()
	endSeconds := end.Unix()
	intervalTs := (endSeconds - begin.Unix()) / (amountOfPoints - 1)

	// totalTime := float64(p.TotalTime) // to calculate QPS
	globalSum := totalStats.Sum // to calculate Percentage

	profile.Query = make([]QueryRank, int64(rankBy.Limit)+1)
	profile.Query[0].Percentage = 1 // 100%
	profile.Query[0].Stats = totalStats.Stats
	profile.Query[0].QPS = totalStats.Stats.Cnt / intervalTime
	profile.Query[0].Load = totalStats.Stats.Sum / intervalTime
	profile.Query[0].Log, err = qrm.SparklineData(endSeconds, intervalTs, 0, instanceID, begin, end)
	if err != nil {
		return &profile, fmt.Errorf("Cannot get Sparkline Data for TOTAL (%v)", err)
	}

	const queryClassMetricsProfileTemplate = `
		SELECT 
			query_class_id,
			SUM(query_count) AS cnt,
			SUM(Query_time_sum) AS sum,
			MIN(Query_time_min) AS min,
			SUM(Query_time_sum)/SUM(query_count) AS avg,
			AVG(Query_time_p95) AS p95,
			MAX(Query_time_max) AS max
		FROM query_class_metrics
		WHERE instance_id = (:instance_id) AND (start_ts >= (:start_ts) AND start_ts < (:end_ts) )
			{{ if .FilterQueryClassIDs }}
			AND query_class_id IN ({{ range $index, $element := .FilterQueryClassIDs}}{{if $index}}, {{end}}{{$element}}{{end}})
			{{end}}
		GROUP BY query_class_id
		ORDER BY sum DESC
		LIMIT {{ if .Offset }} {{ .Offset }},{{end}} {{.Limit}}
	`

	tmpl, err = template.New("queryClassMetricsProfileTemplate").Parse(queryClassMetricsProfileTemplate)
	if err != nil {
		return &profile, err
	}

	var queryClassProfileBuffer bytes.Buffer
	err = tmpl.Execute(&queryClassProfileBuffer, args)
	if err != nil {
		return &profile, err
	}

	nstmt, err = qrm.conns.ClickHouse.PrepareNamed(queryClassProfileBuffer.String())
	if err != nil {
		return nil, err
	}
	var queryClassStats []struct {
		QueryClassID uint `db:"query_class_id"`
		Stats
	}
	err = nstmt.Select(&queryClassStats, args)
	if err != nil {
		return nil, err
	}

	var query = map[uint]int{}
	var queryClassIDs []uint

	for rank, stat := range queryClassStats {
		rank++ // start from 1 (0 is TOTAL)
		sparkLine, err := qrm.SparklineData(endSeconds, intervalTs, stat.QueryClassID, instanceID, begin, end)
		if err != nil {
			return nil, fmt.Errorf("Cannot get Sparkline Data for QueryClassID %v (%v)", stat.QueryClassID, err)
		}
		r := QueryRank{
			Rank:       uint(rank),
			Stats:      stat.Stats,
			Percentage: stat.Stats.Sum / globalSum,
			QPS:        stat.Stats.Cnt / intervalTime,
			Load:       stat.Stats.Sum / intervalTime,
			Log:        sparkLine,
		}

		profile.Query[rank] = r
		query[stat.QueryClassID] = rank
		queryClassIDs = append(queryClassIDs, stat.QueryClassID)
	}

	const queryClassIdentifiersTemplate = `
		SELECT query_class_id, checksum, abstract, fingerprint
		FROM query_classes
		WHERE query_class_id IN ({{ range $index, $element := .FilterQueryClassIDs}}{{if $index}}, {{end}}{{$element}}{{end}})
	`

	args.FilterQueryClassIDs = queryClassIDs

	tmpl, err = template.New("queryClassIdentifiersTemplate").Parse(queryClassIdentifiersTemplate)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare queryClassIdentifiersTemplate (%v)", err)
	}

	var queryClassIdentifiersBuffer bytes.Buffer
	err = tmpl.Execute(&queryClassIdentifiersBuffer, args)
	if err != nil {
		return nil, fmt.Errorf("Cannot execute queryClassIdentifiersTemplate (%v)", err)
	}

	var queryClasses []struct {
		QueryClassID uint   `db:"query_class_id"`
		CheckSum     string `db:"checksum"`
		Abstract     string `db:"abstract"`
		FingerPrint  string `db:"fingerprint"`
	}
	err = qrm.conns.SQLite.Select(&queryClasses, queryClassIdentifiersBuffer.String())
	if err != nil {
		return nil, fmt.Errorf("Cannot select for queryClassIdentifiersBuffer (%v)", err)
	}

	for _, queryClass := range queryClasses {
		rank := query[queryClass.QueryClassID]
		profile.Query[rank].ID = queryClass.CheckSum
		profile.Query[rank].Abstract = queryClass.Abstract
		profile.Query[rank].Fingerprint = queryClass.FingerPrint
	}

	return &profile, nil
}

func (qrm *QueryReportManager) filterByFingerprint(instanceID uint, begin, end time.Time, search string) (*[]uint, error) {

	queryClassIDs := []uint{}

	const selectQueryClasses = `
		SELECT query_class_id 
		FROM query_classes
		WHERE checksum = :checksum OR abstract LIKE :abstract OR fingerprint LIKE :fingerprint
		GROUP BY query_class_id;
	`
	searchArg := struct {
		Checksum    string `db:"checksum"`
		Abstract    string `db:"abstract"`
		Fingerprint string `db:"fingerprint"`
	}{search, "%" + search, "%" + search + "%"}

	nstmt, err := qrm.conns.SQLite.PrepareNamed(selectQueryClasses)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare named selectQueryClasses (%v)", err)
	}

	err = nstmt.Select(&queryClassIDs, searchArg)
	if err != nil {
		return nil, fmt.Errorf("Cannot select selectQueryClasses (%v)", err)
	}

	const filterSelectedQueryClassesTemplate = `
		SELECT query_class_id 
		FROM query_class_metrics
		WHERE 
			instance_id = :instance_id
			AND start_ts > :start_ts
			AND end_ts <= :end_ts
			IN ({{ range $index, $element := .QueryClassIDs}}{{if $index}}, {{end}}{{$element}}{{end}})
		GROUP BY query_class_id;
	`
	args := struct {
		InstanceID    uint      `db:"instance_id"`
		Begin         time.Time `db:"start_ts"`
		End           time.Time `db:"end_ts"`
		QueryClassIDs []uint    `db:"query_class_ids"`
	}{instanceID, begin, end, queryClassIDs}

	tmpl, err := template.New("filterSelectedQueryClassesTemplate").Parse(filterSelectedQueryClassesTemplate)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare filterSelectedQueryClassesTemplate (%v)", err)
	}

	var filterSelectedQueryClassesBuffer bytes.Buffer
	err = tmpl.Execute(&filterSelectedQueryClassesBuffer, args)
	if err != nil {
		return nil, fmt.Errorf("Cannot execute filterSelectedQueryClassesTemplate (%v)", err)
	}

	nstmt, err = qrm.conns.ClickHouse.PrepareNamed(filterSelectedQueryClassesBuffer.String())
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare named filterSelectedQueryClasses (%v)", err)
	}

	err = nstmt.Select(&queryClassIDs, args)
	if err != nil {
		return nil, fmt.Errorf("Cannot select named filterSelectedQueryClasses (%v)", err)
	}

	return &queryClassIDs, nil
}
