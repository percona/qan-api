package models

import (
	"bytes"
	"log"
	"reflect"
	"strings"
	"text/template"
	"time"
)

const maxAmountOfPoints = 60

// Metrics provire instruments to works with metrics
type metrics struct{}

// Metrics instance of metrics model
var Metrics = metrics{}

type metricGroup struct {
	Basic             bool
	PerconaServer     bool `db:"percona_server"`
	PerformanceSchema bool `db:"performance_schema"`
	ServerSummary     bool
	CountField        string
}

type args struct {
	ClassID    uint `db:"class_id"`
	InstanceID uint `db:"instance_id"`
	Begin      time.Time
	End        time.Time
	EndTS      int64 `db:"end_ts"`
	IntervalTS int64 `db:"interval_ts"`
}

const metricGroupQuery = `
SELECT
    IFNULL((SELECT true FROM query_global_metrics
        WHERE instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end)
        AND Query_time_sum IS NOT NULL
        LIMIT 1), false) AS basic,
    IFNULL((SELECT true FROM query_global_metrics
        WHERE instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end)
        AND Rows_affected_sum IS NOT NULL
        LIMIT 1), false) AS percona_server,
    IFNULL((SELECT true FROM query_global_metrics
        WHERE instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end)
        AND Errors_sum IS NOT NULL
        LIMIT 1), false) AS performance_schema;
`

func (m metrics) identifyMetricGroup(instanceID uint, begin, end time.Time) metricGroup {
	currentMetricGroup := metricGroup{}
	currentMetricGroup.CountField = "query_count"
	args := struct {
		InstanceID uint `db:"instance_id"`
		Begin      time.Time
		End        time.Time
	}{
		instanceID,
		begin,
		end,
	}

	if nstmt, err := db.PrepareNamed(metricGroupQuery); err != nil {
		log.Fatalln(err)
	} else if err = nstmt.Get(&currentMetricGroup, args); err != nil {
		log.Fatalln(err)
	}

	return currentMetricGroup
}

type classMetrics struct {
	generalMetrics
	metricsPercentOfTotal
	rateMetrics
	specialMetrics
}

// GetClassMetrics return metrics for given instance and query class
func (m metrics) GetClassMetrics(classID, instanceID uint, begin, end time.Time) (classMetrics, []rateMetrics) {
	currentMetricGroup := m.identifyMetricGroup(instanceID, begin, end)
	currentMetricGroup.CountField = "query_count"

	intervalTime := end.Sub(begin).Minutes()
	amountOfPoints := int64(maxAmountOfPoints)
	if intervalTime < maxAmountOfPoints {
		amountOfPoints = int64(intervalTime)
	}
	endTs := end.Unix()
	intervalTs := int64(end.Sub(begin).Seconds()) / amountOfPoints
	args := args{
		classID,
		instanceID,
		begin,
		end,
		endTs,
		intervalTs,
	}
	// this two lines should be before ServerSummary = true
	generalClassMetrics := m.getMetrics(currentMetricGroup, args)
	sparks := m.getSparklines(currentMetricGroup, args, amountOfPoints)

	// turns metric group to global
	currentMetricGroup.ServerSummary = true
	currentMetricGroup.CountField = "total_query_count"
	generalGlobalMetrics := m.getMetrics(currentMetricGroup, args)

	classMetricsOfTotal := m.computeOfTotal(generalClassMetrics, generalGlobalMetrics)
	aMetrics := m.computeRateMetrics(generalClassMetrics, begin, end)
	sMetrics := m.computeSpecialMetrics(generalClassMetrics)

	classMetrics := classMetrics{
		generalMetrics:        generalClassMetrics,
		metricsPercentOfTotal: classMetricsOfTotal,
		rateMetrics:           aMetrics,
		specialMetrics:        sMetrics,
	}
	return classMetrics, sparks
}

type globalMetrics struct {
	generalMetrics
	rateMetrics
	specialMetrics
}

// GetGlobalMetrics return metrics for given instance
func (m metrics) GetGlobalMetrics(instanceID uint, begin, end time.Time) (globalMetrics, []rateMetrics) {
	currentMetricGroup := m.identifyMetricGroup(instanceID, begin, end)
	currentMetricGroup.ServerSummary = true
	currentMetricGroup.CountField = "total_query_count"

	intervalTime := end.Sub(begin).Minutes()
	endTs := end.Unix()
	amountOfPoints := int64(maxAmountOfPoints)
	if intervalTime < maxAmountOfPoints {
		amountOfPoints = int64(intervalTime)
	}
	intervalTs := int64(end.Sub(begin).Seconds()) / amountOfPoints
	args := args{
		0,
		instanceID,
		begin,
		end,
		endTs,
		intervalTs,
	}

	generalGlobalMetrics := m.getMetrics(currentMetricGroup, args)
	sparks := m.getSparklines(currentMetricGroup, args, amountOfPoints)

	aMetrics := m.computeRateMetrics(generalGlobalMetrics, begin, end)
	sMetrics := m.computeSpecialMetrics(generalGlobalMetrics)
	globalMetrics := globalMetrics{
		generalGlobalMetrics,
		aMetrics,
		sMetrics,
	}
	return globalMetrics, sparks
}

func (m metrics) getMetrics(group metricGroup, args args) generalMetrics {
	var queryClassMetricsBuffer bytes.Buffer
	if tmpl, err := template.New("queryClassMetricsSQL").Parse(queryClassMetricsTemplate); err != nil {
		log.Fatalln(err)
	} else if err = tmpl.Execute(&queryClassMetricsBuffer, group); err != nil {
		log.Fatalln(err)
	}

	queryClassMetricsSQL := queryClassMetricsBuffer.String()
	gMetrics := generalMetrics{}
	if nstmt, err := db.PrepareNamed(queryClassMetricsSQL); err != nil {
		log.Fatalln(err)
	} else if err = nstmt.Get(&gMetrics, args); err != nil {
		log.Fatalln(err)
	}

	return gMetrics
}

func (m metrics) getSparklines(group metricGroup, args args, amountOfPoints int64) []rateMetrics {
	var querySparklinesBuffer bytes.Buffer
	if tmpl, err := template.New("querySparklinesSQL").Parse(querySparklinesTemplate); err != nil {
		log.Fatalln(err)
	} else if err = tmpl.Execute(&querySparklinesBuffer, group); err != nil {
		log.Fatalln(err)
	}

	querySparklinesSQL := querySparklinesBuffer.String()
	var sparksWithGaps []rateMetrics
	if nstmt, err := db.PrepareNamed(querySparklinesSQL); err != nil {
		log.Fatalln(err)
	} else if err = nstmt.Select(&sparksWithGaps, args); err != nil {
		log.Fatalln(err)
	}

	metricLogRaw := make(map[int64]rateMetrics)

	for i := range sparksWithGaps {
		key := sparksWithGaps[i].Ts.Unix()
		metricLogRaw[key] = sparksWithGaps[i]
	}

	// fills up gaps in sparklines by zero values
	var sparks []rateMetrics
	var pointN int64
	for pointN = 0; pointN < amountOfPoints; pointN++ {
		ts := args.EndTS - pointN*args.IntervalTS
		val, ok := metricLogRaw[ts]
		// skip first or last point if they are empty
		if (pointN == 0 || pointN == amountOfPoints-1) && !ok {
			continue
		}
		if !ok {
			val = rateMetrics{Point: pointN, Ts: time.Unix(ts, 0).UTC()}
		}
		sparks = append(sparks, val)
	}
	return sparks
}

func (m metrics) computeOfTotal(classMetrics, globalMetrics generalMetrics) metricsPercentOfTotal {
	mPercentOfTotal := metricsPercentOfTotal{}
	reflectPercentOfTotal := reflect.ValueOf(&mPercentOfTotal).Elem()
	reflectClassMetrics := reflect.ValueOf(&classMetrics).Elem()
	reflectGlobalMetrics := reflect.ValueOf(&globalMetrics).Elem()

	for i := 0; i < reflectPercentOfTotal.NumField(); i++ {
		fieldName := reflectPercentOfTotal.Type().Field(i).Name
		classVal := reflectClassMetrics.FieldByName(strings.TrimSuffix(fieldName, "_of_total")).Float()
		totalVal := reflectGlobalMetrics.FieldByName(strings.TrimSuffix(fieldName, "_of_total")).Float()
		var n float64
		if totalVal > 0 {
			n = classVal / totalVal
		}
		reflectPercentOfTotal.FieldByName(fieldName).SetFloat(n)
	}
	return mPercentOfTotal
}

func (m metrics) computeRateMetrics(gMetrics generalMetrics, begin, end time.Time) rateMetrics {
	duration := end.Sub(begin).Seconds()
	aMetrics := rateMetrics{}
	reflectionAdittionalMetrics := reflect.ValueOf(&aMetrics).Elem()
	reflectionGeneralMetrics := reflect.ValueOf(&gMetrics).Elem()

	for i := 0; i < reflectionAdittionalMetrics.NumField(); i++ {
		fieldName := reflectionAdittionalMetrics.Type().Field(i).Name
		if strings.HasSuffix(fieldName, "_per_sec") {
			generalFieldName := strings.TrimSuffix(fieldName, "_per_sec")
			metricVal := reflectionGeneralMetrics.FieldByName(generalFieldName).Float()

			reflectionAdittionalMetrics.FieldByName(fieldName).SetFloat(metricVal / duration)
		}
	}
	return aMetrics
}

func (m metrics) computeSpecialMetrics(gMetrics generalMetrics) specialMetrics {
	sMetrics := specialMetrics{}
	reflectionSpecialMetrics := reflect.ValueOf(&sMetrics).Elem()
	reflectionGeneralMetrics := reflect.ValueOf(&gMetrics).Elem()

	for i := 0; i < reflectionSpecialMetrics.NumField(); i++ {
		field := reflectionSpecialMetrics.Type().Field(i)
		fieldName := field.Name
		fieldTag := field.Tag.Get("divider")
		generalFieldName := strings.Split(fieldName, "_per_")[0]
		dividend := reflectionGeneralMetrics.FieldByName(generalFieldName).Float()
		divider := reflectionGeneralMetrics.FieldByName(fieldTag).Float()
		if divider == 0 {
			continue
		}
		reflectionSpecialMetrics.FieldByName(fieldName).SetFloat(dividend / divider)
	}
	return sMetrics
}

type specialMetrics struct {
	Lock_time_avg_per_query_time                     float32 `json:",omitempty" divider:"Query_time_avg"`
	InnoDB_rec_lock_wait_avg_per_query_time          float32 `json:",omitempty" divider:"Query_time_avg"`
	InnoDB_IO_r_wait_avg_per_query_time              float32 `json:",omitempty" divider:"Query_time_avg"`
	InnoDB_queue_wait_avg_per_query_time             float32 `json:",omitempty" divider:"Query_time_avg"`
	InnoDB_IO_r_bytes_sum_per_io                     float32 `json:",omitempty" divider:"InnoDB_IO_r_ops_sum"`
	QC_Hit_sum_per_query                             float32 `json:",omitempty" divider:"Query_count"`
	Bytes_sent_sum_per_rows                          float32 `json:",omitempty" divider:"Rows_sent_sum"`
	Rows_examined_sum_per_rows                       float32 `json:",omitempty" divider:"Rows_sent_sum"`
	Filesort_sum_per_query                           float32 `json:",omitempty" divider:"Query_count"`
	Filesort_on_disk_sum_per_query                   float32 `json:",omitempty" divider:"Query_count"`
	Merge_passes_sum_per_external_sort               float32 `json:",omitempty" divider:"Filesort_sum"`
	Full_join_sum_per_query                          float32 `json:",omitempty" divider:"Query_count"`
	Full_scan_sum_per_query                          float32 `json:",omitempty" divider:"Query_count"`
	Tmp_table_sum_per_query                          float32 `json:",omitempty" divider:"Query_count"`
	Tmp_tables_sum_per_query_with_tmp_table          float32 `json:",omitempty" divider:"Tmp_table_sum"`
	Tmp_table_on_disk_sum_per_query                  float32 `json:",omitempty" divider:"Query_count"`
	Tmp_disk_tables_sum_per_query_with_tmp_table     float32 `json:",omitempty" divider:"Tmp_table_on_disk_sum"`
	Tmp_table_sizes_sum_per_query_with_any_tmp_table float32 `json:",omitempty" divider:"Total_tmp_tables_sum"` // = Tmp_table_sum + Tmp_table_on_disk_sum
}

type rateMetrics struct {
	Point                            int64
	Ts                               time.Time
	Query_count_per_sec              float32 `json:",omitempty"`
	Query_time_sum_per_sec           float32 `json:",omitempty"`
	Lock_time_sum_per_sec            float32 `json:",omitempty"` // load
	InnoDB_rec_lock_wait_sum_per_sec float32 `json:",omitempty"` // load
	InnoDB_IO_r_wait_sum_per_sec     float32 `json:",omitempty"` // load
	InnoDB_IO_r_ops_sum_per_sec      float32 `json:",omitempty"`
	InnoDB_IO_r_bytes_sum_per_sec    float32 `json:",omitempty"`
	InnoDB_queue_wait_sum_per_sec    float32 `json:",omitempty"` // load

	QC_Hit_sum_per_sec            float32 `json:",omitempty"`
	Rows_sent_sum_per_sec         float32 `json:",omitempty"`
	Bytes_sent_sum_per_sec        float32 `json:",omitempty"`
	Rows_examined_sum_per_sec     float32 `json:",omitempty"`
	Rows_affected_sum_per_sec     float32 `json:",omitempty"`
	Filesort_sum_per_sec          float32 `json:",omitempty"`
	Filesort_on_disk_sum_per_sec  float32 `json:",omitempty"`
	Merge_passes_sum_per_sec      float32 `json:",omitempty"`
	Full_join_sum_per_sec         float32 `json:",omitempty"`
	Full_scan_sum_per_sec         float32 `json:",omitempty"`
	Tmp_table_sum_per_sec         float32 `json:",omitempty"`
	Tmp_tables_sum_per_sec        float32 `json:",omitempty"`
	Tmp_table_on_disk_sum_per_sec float32 `json:",omitempty"`
	Tmp_disk_tables_sum_per_sec   float32 `json:",omitempty"`
	Tmp_table_sizes_sum_per_sec   float32 `json:",omitempty"`

	/* Perf Schema */

	Errors_sum_per_sec                 float32 `json:",omitempty"`
	Warnings_sum_per_sec               float32 `json:",omitempty"`
	Select_full_range_join_sum_per_sec float32 `json:",omitempty"`
	Select_range_sum_per_sec           float32 `json:",omitempty"`
	Select_range_check_sum_per_sec     float32 `json:",omitempty"`
	Sort_range_sum_per_sec             float32 `json:",omitempty"`
	Sort_rows_sum_per_sec              float32 `json:",omitempty"`
	Sort_scan_sum_per_sec              float32 `json:",omitempty"`
	No_index_used_sum_per_sec          float32 `json:",omitempty"`
	No_good_index_used_sum_per_sec     float32 `json:",omitempty"`
}

type metricsPercentOfTotal struct {
	Query_count_of_total       float32
	Query_time_sum_of_total    float32
	Lock_time_sum_of_total     float32
	Rows_sent_sum_of_total     float32
	Rows_examined_sum_of_total float32
	// 5
	/* Perf Schema or Percona Server */

	Rows_affected_sum_of_total     float32 `json:",omitempty"`
	Bytes_sent_sum_of_total        float32 `json:",omitempty"`
	Tmp_tables_sum_of_total        float32 `json:",omitempty"`
	Tmp_disk_tables_sum_of_total   float32 `json:",omitempty"`
	Tmp_table_sizes_sum_of_total   float32 `json:",omitempty"`
	QC_Hit_sum_of_total            float32 `json:",omitempty"`
	Full_scan_sum_of_total         float32 `json:",omitempty"`
	Full_join_sum_of_total         float32 `json:",omitempty"`
	Tmp_table_sum_of_total         float32 `json:",omitempty"`
	Tmp_table_on_disk_sum_of_total float32 `json:",omitempty"`
	Filesort_sum_of_total          float32 `json:",omitempty"`
	Filesort_on_disk_sum_of_total  float32 `json:",omitempty"`
	Merge_passes_sum_of_total      float32 `json:",omitempty"`
	// 13
	/* Percona Server */

	InnoDB_IO_r_ops_sum_of_total       float32 `json:",omitempty"`
	InnoDB_IO_r_bytes_sum_of_total     float32 `json:",omitempty"`
	InnoDB_IO_r_wait_sum_of_total      float32 `json:",omitempty"`
	InnoDB_rec_lock_wait_sum_of_total  float32 `json:",omitempty"`
	InnoDB_queue_wait_sum_of_total     float32 `json:",omitempty"`
	InnoDB_pages_distinct_sum_of_total float32 `json:",omitempty"`
	// 6
	/* Perf Schema */

	Errors_sum_of_total                 float32 `json:",omitempty"`
	Warnings_sum_of_total               float32 `json:",omitempty"`
	Select_full_range_join_sum_of_total float32 `json:",omitempty"`
	Select_range_sum_of_total           float32 `json:",omitempty"`
	Select_range_check_sum_of_total     float32 `json:",omitempty"`
	Sort_range_sum_of_total             float32 `json:",omitempty"`
	Sort_rows_sum_of_total              float32 `json:",omitempty"`
	Sort_scan_sum_of_total              float32 `json:",omitempty"`
	No_index_used_sum_of_total          float32 `json:",omitempty"`
	No_good_index_used_sum_of_total     float32 `json:",omitempty"`
	// 10
}

// 34

type generalMetrics struct {

	/*  Basic metrics */

	Query_count       float32
	Query_time_sum    float32
	Query_time_min    float32
	Query_time_avg    float32
	Query_time_med    float32
	Query_time_p95    float32
	Query_time_max    float32
	Lock_time_sum     float32
	Lock_time_min     float32
	Lock_time_avg     float32
	Lock_time_med     float32
	Lock_time_p95     float32
	Lock_time_max     float32
	Rows_sent_sum     float32
	Rows_sent_min     float32
	Rows_sent_avg     float32
	Rows_sent_med     float32
	Rows_sent_p95     float32
	Rows_sent_max     float32
	Rows_examined_sum float32
	Rows_examined_min float32
	Rows_examined_avg float32
	Rows_examined_med float32
	Rows_examined_p95 float32
	Rows_examined_max float32
	Bytes_sent_sum    float32
	Bytes_sent_min    float32
	Bytes_sent_avg    float32
	Bytes_sent_med    float32
	Bytes_sent_p95    float32
	Bytes_sent_max    float32

	/* Perf Schema or Percona Server */

	Rows_affected_sum     float32 `json:",omitempty"`
	Rows_affected_min     float32 `json:",omitempty"`
	Rows_affected_avg     float32 `json:",omitempty"`
	Rows_affected_med     float32 `json:",omitempty"`
	Rows_affected_p95     float32 `json:",omitempty"`
	Rows_affected_max     float32 `json:",omitempty"`
	Tmp_tables_sum        float32 `json:",omitempty"`
	Tmp_tables_min        float32 `json:",omitempty"`
	Tmp_tables_avg        float32 `json:",omitempty"`
	Tmp_tables_med        float32 `json:",omitempty"`
	Tmp_tables_p95        float32 `json:",omitempty"`
	Tmp_tables_max        float32 `json:",omitempty"`
	Tmp_disk_tables_sum   float32 `json:",omitempty"`
	Tmp_disk_tables_min   float32 `json:",omitempty"`
	Tmp_disk_tables_avg   float32 `json:",omitempty"`
	Tmp_disk_tables_med   float32 `json:",omitempty"`
	Tmp_disk_tables_p95   float32 `json:",omitempty"`
	Tmp_disk_tables_max   float32 `json:",omitempty"`
	Tmp_table_sizes_sum   float32 `json:",omitempty"`
	Tmp_table_sizes_min   float32 `json:",omitempty"`
	Tmp_table_sizes_avg   float32 `json:",omitempty"`
	Tmp_table_sizes_med   float32 `json:",omitempty"`
	Tmp_table_sizes_p95   float32 `json:",omitempty"`
	Tmp_table_sizes_max   float32 `json:",omitempty"`
	Total_tmp_tables_sum  float32 `json:",omitempty"`
	QC_Hit_sum            float32 `json:",omitempty"`
	Full_scan_sum         float32 `json:",omitempty"`
	Full_join_sum         float32 `json:",omitempty"`
	Tmp_table_sum         float32 `json:",omitempty"`
	Tmp_table_on_disk_sum float32 `json:",omitempty"`
	Filesort_sum          float32 `json:",omitempty"`
	Filesort_on_disk_sum  float32 `json:",omitempty"`
	Merge_passes_sum      float32 `json:",omitempty"`
	Merge_passes_min      float32 `json:",omitempty"`
	Merge_passes_avg      float32 `json:",omitempty"`
	Merge_passes_med      float32 `json:",omitempty"`
	Merge_passes_p95      float32 `json:",omitempty"`
	Merge_passes_max      float32 `json:",omitempty"`

	/* Percona Server */

	InnoDB_IO_r_ops_sum       float32 `json:",omitempty"`
	InnoDB_IO_r_ops_min       float32 `json:",omitempty"`
	InnoDB_IO_r_ops_avg       float32 `json:",omitempty"`
	InnoDB_IO_r_ops_med       float32 `json:",omitempty"`
	InnoDB_IO_r_ops_p95       float32 `json:",omitempty"`
	InnoDB_IO_r_ops_max       float32 `json:",omitempty"`
	InnoDB_IO_r_bytes_sum     float32 `json:",omitempty"`
	InnoDB_IO_r_bytes_min     float32 `json:",omitempty"`
	InnoDB_IO_r_bytes_avg     float32 `json:",omitempty"`
	InnoDB_IO_r_bytes_med     float32 `json:",omitempty"`
	InnoDB_IO_r_bytes_p95     float32 `json:",omitempty"`
	InnoDB_IO_r_bytes_max     float32 `json:",omitempty"`
	InnoDB_IO_r_wait_sum      float32 `json:",omitempty"`
	InnoDB_IO_r_wait_min      float32 `json:",omitempty"`
	InnoDB_IO_r_wait_avg      float32 `json:",omitempty"`
	InnoDB_IO_r_wait_med      float32 `json:",omitempty"`
	InnoDB_IO_r_wait_p95      float32 `json:",omitempty"`
	InnoDB_IO_r_wait_max      float32 `json:",omitempty"`
	InnoDB_rec_lock_wait_sum  float32 `json:",omitempty"`
	InnoDB_rec_lock_wait_min  float32 `json:",omitempty"`
	InnoDB_rec_lock_wait_avg  float32 `json:",omitempty"`
	InnoDB_rec_lock_wait_med  float32 `json:",omitempty"`
	InnoDB_rec_lock_wait_p95  float32 `json:",omitempty"`
	InnoDB_rec_lock_wait_max  float32 `json:",omitempty"`
	InnoDB_queue_wait_sum     float32 `json:",omitempty"`
	InnoDB_queue_wait_min     float32 `json:",omitempty"`
	InnoDB_queue_wait_avg     float32 `json:",omitempty"`
	InnoDB_queue_wait_med     float32 `json:",omitempty"`
	InnoDB_queue_wait_p95     float32 `json:",omitempty"`
	InnoDB_queue_wait_max     float32 `json:",omitempty"`
	InnoDB_pages_distinct_sum float32 `json:",omitempty"`
	InnoDB_pages_distinct_min float32 `json:",omitempty"`
	InnoDB_pages_distinct_avg float32 `json:",omitempty"`
	InnoDB_pages_distinct_med float32 `json:",omitempty"`
	InnoDB_pages_distinct_p95 float32 `json:",omitempty"`
	InnoDB_pages_distinct_max float32 `json:",omitempty"`

	/* Perf Schema */

	Errors_sum                 float32 `json:",omitempty"`
	Warnings_sum               float32 `json:",omitempty"`
	Select_full_range_join_sum float32 `json:",omitempty"`
	Select_range_sum           float32 `json:",omitempty"`
	Select_range_check_sum     float32 `json:",omitempty"`
	Sort_range_sum             float32 `json:",omitempty"`
	Sort_rows_sum              float32 `json:",omitempty"`
	Sort_scan_sum              float32 `json:",omitempty"`
	No_index_used_sum          float32 `json:",omitempty"`
	No_good_index_used_sum     float32 `json:",omitempty"`
}

const queryClassMetricsTemplate = `
SELECT

{{ if .Basic }}
 /*  Basic metrics */

 COALESCE(SUM({{ .CountField }}), 0) AS query_count,
 COALESCE(SUM(Query_time_sum), 0) AS query_time_sum,
 COALESCE(MIN(Query_time_min), 0) AS query_time_min,
 COALESCE(AVG(Query_time_avg), 0) AS query_time_avg,
 COALESCE(AVG(Query_time_med), 0) AS query_time_med,
 COALESCE(AVG(Query_time_p95), 0) AS query_time_p95,
 COALESCE(MAX(Query_time_max), 0) AS query_time_max,
 COALESCE(SUM(Lock_time_sum), 0) AS lock_time_sum,
 COALESCE(MIN(Lock_time_min), 0) AS lock_time_min,
 COALESCE(AVG(Lock_time_avg), 0) AS lock_time_avg,
 COALESCE(AVG(Lock_time_med), 0) AS lock_time_med,
 COALESCE(AVG(Lock_time_p95), 0) AS lock_time_p95,
 COALESCE(MAX(Lock_time_max), 0) AS lock_time_max,
 COALESCE(SUM(Rows_sent_sum), 0) AS rows_sent_sum,
 COALESCE(MIN(Rows_sent_min), 0) AS rows_sent_min,
 COALESCE(AVG(Rows_sent_avg), 0) AS rows_sent_avg,
 COALESCE(AVG(Rows_sent_med), 0) AS rows_sent_med,
 COALESCE(AVG(Rows_sent_p95), 0) AS rows_sent_p95,
 COALESCE(MAX(Rows_sent_max), 0) AS rows_sent_max,
 COALESCE(SUM(Rows_examined_sum), 0) AS rows_examined_sum,
 COALESCE(MIN(Rows_examined_min), 0) AS rows_examined_min,
 COALESCE(AVG(Rows_examined_avg), 0) AS rows_examined_avg,
 COALESCE(AVG(Rows_examined_med), 0) AS rows_examined_med,
 COALESCE(AVG(Rows_examined_p95), 0) AS rows_examined_p95,
 COALESCE(MAX(Rows_examined_max), 0) AS rows_examined_max,
 COALESCE(SUM(Bytes_sent_sum), 0) AS bytes_sent_sum,
 COALESCE(MIN(Bytes_sent_min), 0) AS bytes_sent_min,
 COALESCE(AVG(Bytes_sent_avg), 0) AS bytes_sent_avg,
 COALESCE(AVG(Bytes_sent_med), 0) AS bytes_sent_med,
 COALESCE(AVG(Bytes_sent_p95), 0) AS bytes_sent_p95,
 COALESCE(MAX(Bytes_sent_max), 0) AS bytes_sent_max

{{ end }}

{{ if or .PerconaServer .PerformanceSchema }}
 /* Perf Schema or Percona Server */

 , /* <-- final comma for basic metrics */

 COALESCE(SUM(Rows_affected_sum), 0) AS rows_affected_sum,
 COALESCE(MIN(Rows_affected_min), 0) AS rows_affected_min,
 COALESCE(AVG(Rows_affected_avg), 0) AS rows_affected_avg,
 COALESCE(AVG(Rows_affected_med), 0) AS rows_affected_med,
 COALESCE(AVG(Rows_affected_p95), 0) AS rows_affected_p95,
 COALESCE(MAX(Rows_affected_max), 0) AS rows_affected_max,

 COALESCE(SUM(Full_scan_sum), 0) AS full_scan_sum,
 COALESCE(SUM(Full_join_sum), 0) AS full_join_sum,
 COALESCE(SUM(Tmp_table_sum), 0) AS tmp_table_sum,
 COALESCE(SUM(Tmp_table_on_disk_sum), 0) AS tmp_table_on_disk_sum,

 COALESCE(SUM(Merge_passes_sum), 0) AS merge_passes_sum,
 COALESCE(MIN(Merge_passes_min), 0) AS merge_passes_min,
 COALESCE(AVG(Merge_passes_avg), 0) AS merge_passes_avg,
 COALESCE(AVG(Merge_passes_med), 0) AS merge_passes_med,
 COALESCE(AVG(Merge_passes_p95), 0) AS merge_passes_p95,
 COALESCE(MAX(Merge_passes_max), 0) AS merge_passes_max,

{{ end }}

{{ if .PerconaServer }}
 /* Percona Server */

 COALESCE(SUM(Tmp_tables_sum), 0) AS tmp_tables_sum,
 COALESCE(MIN(Tmp_tables_min), 0) AS tmp_tables_min,
 COALESCE(AVG(Tmp_tables_avg), 0) AS tmp_tables_avg,
 COALESCE(AVG(Tmp_tables_med), 0) AS tmp_tables_med,
 COALESCE(AVG(Tmp_tables_p95), 0) AS tmp_tables_p95,
 COALESCE(MAX(Tmp_tables_max), 0) AS tmp_tables_max,
 COALESCE(SUM(Tmp_disk_tables_sum), 0) AS tmp_disk_tables_sum,
 COALESCE(MIN(Tmp_disk_tables_min), 0) AS tmp_disk_tables_min,
 COALESCE(AVG(Tmp_disk_tables_avg), 0) AS tmp_disk_tables_avg,
 COALESCE(AVG(Tmp_disk_tables_med), 0) AS tmp_disk_tables_med,
 COALESCE(AVG(Tmp_disk_tables_p95), 0) AS tmp_disk_tables_p95,
 COALESCE(MAX(Tmp_disk_tables_max), 0) AS tmp_disk_tables_max,
 COALESCE(SUM(Tmp_table_sizes_sum), 0) AS tmp_table_sizes_sum,
 COALESCE(MIN(Tmp_table_sizes_min), 0) AS tmp_table_sizes_min,
 COALESCE(AVG(Tmp_table_sizes_avg), 0) AS tmp_table_sizes_avg,
 COALESCE(AVG(Tmp_table_sizes_med), 0) AS tmp_table_sizes_med,
 COALESCE(AVG(Tmp_table_sizes_p95), 0) AS tmp_table_sizes_p95,
 COALESCE(MAX(Tmp_table_sizes_max), 0) AS tmp_table_sizes_max,
 COALESCE(SUM(Tmp_table_sum + Tmp_table_on_disk_sum), 0) AS total_tmp_tables_sum,

 COALESCE(SUM(QC_Hit_sum), 0) AS qc_hit_sum,
 COALESCE(SUM(Filesort_sum), 0) AS filesort_sum,

 COALESCE(SUM(Filesort_on_disk_sum), 0) AS filesort_on_disk_sum,
 COALESCE(SUM(InnoDB_IO_r_ops_sum), 0) AS innodb_io_r_ops_sum,
 COALESCE(MIN(InnoDB_IO_r_ops_min), 0) AS innodb_io_r_ops_min,
 COALESCE(AVG(InnoDB_IO_r_ops_avg), 0) AS innodb_io_r_ops_avg,
 COALESCE(AVG(InnoDB_IO_r_ops_med), 0) AS innodb_io_r_ops_med,
 COALESCE(AVG(InnoDB_IO_r_ops_p95), 0) AS innodb_io_r_ops_p95,
 COALESCE(MAX(InnoDB_IO_r_ops_max), 0) AS innodb_io_r_ops_max,
 COALESCE(SUM(InnoDB_IO_r_bytes_sum), 0) AS innodb_io_r_bytes_sum,
 COALESCE(MIN(InnoDB_IO_r_bytes_min), 0) AS innodb_io_r_bytes_min,
 COALESCE(AVG(InnoDB_IO_r_bytes_avg), 0) AS innodb_io_r_bytes_avg,
 COALESCE(AVG(InnoDB_IO_r_bytes_med), 0) AS innodb_io_r_bytes_med,
 COALESCE(AVG(InnoDB_IO_r_bytes_p95), 0) AS innodb_io_r_bytes_p95,
 COALESCE(MAX(InnoDB_IO_r_bytes_max), 0) AS innodb_io_r_bytes_max,
 COALESCE(SUM(InnoDB_IO_r_wait_sum), 0) AS innodb_io_r_wait_sum,
 COALESCE(MIN(InnoDB_IO_r_wait_min), 0) AS innodb_io_r_wait_min,
 COALESCE(AVG(InnoDB_IO_r_wait_avg), 0) AS innodb_io_r_wait_avg,
 COALESCE(AVG(InnoDB_IO_r_wait_med), 0) AS innodb_io_r_wait_med,
 COALESCE(AVG(InnoDB_IO_r_wait_p95), 0) AS innodb_io_r_wait_p95,
 COALESCE(MAX(InnoDB_IO_r_wait_max), 0) AS innodb_io_r_wait_max,
 COALESCE(SUM(InnoDB_rec_lock_wait_sum), 0) AS innodb_rec_lock_wait_sum,
 COALESCE(MIN(InnoDB_rec_lock_wait_min), 0) AS innodb_rec_lock_wait_min,
 COALESCE(AVG(InnoDB_rec_lock_wait_avg), 0) AS innodb_rec_lock_wait_avg,
 COALESCE(AVG(InnoDB_rec_lock_wait_med), 0) AS innodb_rec_lock_wait_med,
 COALESCE(AVG(InnoDB_rec_lock_wait_p95), 0) AS innodb_rec_lock_wait_p95,
 COALESCE(MAX(InnoDB_rec_lock_wait_max), 0) AS innodb_rec_lock_wait_max,
 COALESCE(SUM(InnoDB_queue_wait_sum), 0) AS innodb_queue_wait_sum,
 COALESCE(MIN(InnoDB_queue_wait_min), 0) AS innodb_queue_wait_min,
 COALESCE(AVG(InnoDB_queue_wait_avg), 0) AS innodb_queue_wait_avg,
 COALESCE(AVG(InnoDB_queue_wait_med), 0) AS innodb_queue_wait_med,
 COALESCE(AVG(InnoDB_queue_wait_p95), 0) AS innodb_queue_wait_p95,
 COALESCE(MAX(InnoDB_queue_wait_max), 0) AS innodb_queue_wait_max,
 COALESCE(SUM(InnoDB_pages_distinct_sum), 0) AS innodb_pages_distinct_sum,
 COALESCE(MIN(InnoDB_pages_distinct_min), 0) AS innodb_pages_distinct_min,
 COALESCE(AVG(InnoDB_pages_distinct_avg), 0) AS innodb_pages_distinct_avg,
 COALESCE(AVG(InnoDB_pages_distinct_med), 0) AS innodb_pages_distinct_med,
 COALESCE(AVG(InnoDB_pages_distinct_p95), 0) AS innodb_pages_distinct_p95,
 COALESCE(MAX(InnoDB_pages_distinct_max), 0) AS innodb_pages_distinct_max
{{ end }}

{{ if .PerformanceSchema }}
/* Perf Schema */

	COALESCE(SUM(Errors_sum), 0) AS errors_sum,
	COALESCE(SUM(Warnings_sum), 0) AS warnings_sum,
	COALESCE(SUM(Select_full_range_join_sum), 0) AS select_full_range_join_sum,
	COALESCE(SUM(Select_range_sum), 0) AS select_range_sum,
	COALESCE(SUM(Select_range_check_sum), 0) AS select_range_check_sum,
	COALESCE(SUM(Sort_range_sum), 0) AS sort_range_sum,
	COALESCE(SUM(Sort_rows_sum), 0) AS sort_rows_sum,
	COALESCE(SUM(Sort_scan_sum), 0) AS sort_scan_sum,
	COALESCE(SUM(No_index_used_sum), 0) AS no_index_used_sum,
	COALESCE(SUM(No_good_index_used_sum), 0) AS no_good_index_used_sum
{{ end }}

FROM {{if .ServerSummary }} query_global_metrics {{ else }} query_class_metrics {{ end }}
WHERE {{if not .ServerSummary }} query_class_id = :class_id AND {{ end }}
	 instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end);
`

const querySparklinesTemplate = `
SELECT
    (:end_ts - UNIX_TIMESTAMP(start_ts)) DIV :interval_ts as point,
    FROM_UNIXTIME(:end_ts - (SELECT point) * :interval_ts) AS ts,
	{{ if .Basic }}
	/*  Basic metrics */
        COALESCE(SUM({{ .CountField }}), 0) / :interval_ts AS query_count_per_sec,
        COALESCE(SUM(Query_time_sum), 0) / :interval_ts AS query_time_sum_per_sec,
	COALESCE(SUM(Lock_time_sum), 0) / :interval_ts AS lock_time_sum_per_sec,
	COALESCE(SUM(Rows_sent_sum), 0) / :interval_ts AS rows_sent_sum_per_sec,
	COALESCE(SUM(Rows_examined_sum), 0) / :interval_ts AS rows_examined_sum_per_sec,
	COALESCE(SUM(Bytes_sent_sum), 0) / :interval_ts AS bytes_sent_sum_per_sec
	{{ end }}
	{{ if or .PerconaServer .PerformanceSchema }}
 	/* Perf Schema or Percona Server */
 	, /* <-- final comma for basic metrics */
	COALESCE(SUM(Rows_affected_sum), 0) / :interval_ts AS rows_affected_sum_per_sec,
	COALESCE(SUM(Merge_passes_sum), 0) / :interval_ts AS merge_passes_sum_per_sec,
	COALESCE(SUM(Full_join_sum), 0) / :interval_ts AS full_join_sum_per_sec,
	COALESCE(SUM(Full_scan_sum), 0) / :interval_ts AS full_scan_sum_per_sec,
	COALESCE(SUM(Tmp_table_sum), 0) / :interval_ts AS tmp_table_sum_per_sec,
	COALESCE(SUM(Tmp_table_on_disk_sum), 0) / :interval_ts AS tmp_table_on_disk_sum_per_sec,
    {{ end }}
	{{ if .PerconaServer }}
    /* Percona Server */
	COALESCE(SUM(InnoDB_IO_r_ops_sum), 0) / :interval_ts AS innodb_io_r_ops_sum_per_sec,

        COALESCE(SUM(InnoDB_IO_r_wait_sum), 0) / :interval_ts AS innodb_io_r_wait_sum_per_sec,
	COALESCE(SUM(InnoDB_rec_lock_wait_sum), 0) / :interval_ts AS innodb_rec_lock_wait_sum_per_sec,
	COALESCE(SUM(InnoDB_queue_wait_sum), 0) / :interval_ts AS innodb_queue_wait_sum_per_sec,

	COALESCE(SUM(InnoDB_IO_r_bytes_sum), 0) / :interval_ts AS innodb_io_r_bytes_sum_per_sec,
	COALESCE(SUM(QC_Hit_sum), 0) / :interval_ts AS qc_hit_sum_per_sec,
	COALESCE(SUM(Filesort_sum), 0) / :interval_ts AS filesort_sum_per_sec,
	COALESCE(SUM(Filesort_on_disk_sum), 0) / :interval_ts AS filesort_on_disk_sum_per_sec,
	COALESCE(SUM(Tmp_tables_sum), 0) / :interval_ts AS tmp_tables_sum_per_sec,
	COALESCE(SUM(Tmp_disk_tables_sum), 0) / :interval_ts AS tmp_disk_tables_sum_per_sec,
	COALESCE(SUM(Tmp_table_sizes_sum), 0) / :interval_ts AS tmp_table_sizes_sum_per_sec
	{{ end }}
	{{ if .PerformanceSchema }}
	/* Perf Schema */
	COALESCE(SUM(Errors_sum), 0) / :interval_ts AS errors_sum_per_sec,
	COALESCE(SUM(Warnings_sum), 0) / :interval_ts AS warnings_sum_per_sec,
	COALESCE(SUM(Select_full_range_join_sum), 0) / :interval_ts AS select_full_range_join_sum_per_sec,
	COALESCE(SUM(Select_range_sum), 0) / :interval_ts AS select_range_sum_per_sec,
	COALESCE(SUM(Select_range_check_sum), 0) / :interval_ts AS select_range_check_sum_per_sec,
	COALESCE(SUM(Sort_range_sum), 0) / :interval_ts AS sort_range_sum_per_sec,
	COALESCE(SUM(Sort_rows_sum), 0) / :interval_ts AS sort_rows_sum_per_sec,
	COALESCE(SUM(Sort_scan_sum), 0) / :interval_ts AS sort_scan_sum_per_sec,
	COALESCE(SUM(No_index_used_sum), 0) / :interval_ts AS no_index_used_sum_per_sec,
	COALESCE(SUM(No_good_index_used_sum), 0) / :interval_ts AS no_good_index_used_sum_per_sec

{{ end }}
FROM {{if .ServerSummary }} query_global_metrics {{ else }} query_class_metrics {{ end }}
WHERE {{if not .ServerSummary }} query_class_id = :class_id AND {{ end }}
    instance_id = :instance_id AND (start_ts >= :begin AND start_ts < :end)
GROUP BY point;
`
