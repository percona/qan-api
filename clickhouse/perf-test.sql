/********* TEST select Query Profile **********/
/* MySQL */
SELECT query_class_id, SUM(query_count), SUM(Query_time_sum), MIN(Query_time_min), SUM(Query_time_sum)/SUM(query_count), AVG(Query_time_med), AVG(Query_time_p95), MAX(Query_time_max)
FROM query_class_metrics
WHERE instance_id = 18 AND (start_ts >= '2016-11-27 00:00:00' AND start_ts < '2016-11-28 00:00:00')
GROUP BY query_class_id
ORDER BY SUM(Query_time_sum)
DESC LIMIT 10 OFFSET 0;

/* ClickHouse */
SELECT query_class_id, SUM(query_count), SUM(Query_time_sum), MIN(Query_time_min), SUM(Query_time_sum)/SUM(query_count), AVG(Query_time_med), AVG(Query_time_p95), MAX(Query_time_max)
FROM query_class_metrics
WHERE instance_id = 18 AND (start_ts >= '2016-11-27 00:00:00' AND start_ts < '2016-11-28 00:00:00')
GROUP BY query_class_id
ORDER BY SUM(Query_time_sum)
DESC LIMIT 0, 10;


/********* TEST select sparklines for metrics **********/
/* MySQL */
SELECT
    (1480888800 - UNIX_TIMESTAMP(start_ts)) DIV 11520 as point,
    FROM_UNIXTIME(1480888800 - (SELECT point) * 11520) AS ts,

        /*  Basic metrics */
        COALESCE(SUM(query_count), 0) / 11520 AS query_count_per_sec,
        COALESCE(SUM(Query_time_sum), 0) / 11520 AS query_time_sum_per_sec,
        COALESCE(SUM(Lock_time_sum), 0) / 11520 AS lock_time_sum_per_sec,
        COALESCE(SUM(Rows_sent_sum), 0) / 11520 AS rows_sent_sum_per_sec,
        COALESCE(SUM(Rows_examined_sum), 0) / 11520 AS rows_examined_sum_per_sec


        /* Perf Schema or Percona Server */
        , /* <-- final comma for basic metrics */
        COALESCE(SUM(Rows_affected_sum), 0) / 11520 AS rows_affected_sum_per_sec,
        COALESCE(SUM(Merge_passes_sum), 0) / 11520 AS merge_passes_sum_per_sec,
        COALESCE(SUM(Full_join_sum), 0) / 11520 AS full_join_sum_per_sec,
        COALESCE(SUM(Full_scan_sum), 0) / 11520 AS full_scan_sum_per_sec,
        COALESCE(SUM(Tmp_table_sum), 0) / 11520 AS tmp_table_sum_per_sec,
        COALESCE(SUM(Tmp_table_on_disk_sum), 0) / 11520 AS tmp_table_on_disk_sum_per_sec,


    /* Percona Server */
        COALESCE(SUM(Bytes_sent_sum), 0) / 11520 AS bytes_sent_sum_per_sec,
        COALESCE(SUM(InnoDB_IO_r_ops_sum), 0) / 11520 AS innodb_io_r_ops_sum_per_sec,

        COALESCE(SUM(InnoDB_IO_r_wait_sum), 0) / 11520 AS innodb_io_r_wait_sum_per_sec,
        COALESCE(SUM(InnoDB_rec_lock_wait_sum), 0) / 11520 AS innodb_rec_lock_wait_sum_per_sec,
        COALESCE(SUM(InnoDB_queue_wait_sum), 0) / 11520 AS innodb_queue_wait_sum_per_sec,

        COALESCE(SUM(InnoDB_IO_r_bytes_sum), 0) / 11520 AS innodb_io_r_bytes_sum_per_sec,
        COALESCE(SUM(QC_Hit_sum), 0) / 11520 AS qc_hit_sum_per_sec,
        COALESCE(SUM(Filesort_sum), 0) / 11520 AS filesort_sum_per_sec,
        COALESCE(SUM(Filesort_on_disk_sum), 0) / 11520 AS filesort_on_disk_sum_per_sec,
        COALESCE(SUM(Tmp_tables_sum), 0) / 11520 AS tmp_tables_sum_per_sec,
        COALESCE(SUM(Tmp_disk_tables_sum), 0) / 11520 AS tmp_disk_tables_sum_per_sec,
        COALESCE(SUM(Tmp_table_sizes_sum), 0) / 11520 AS tmp_table_sizes_sum_per_sec
FROM  query_class_metrics
WHERE  query_class_id = 64 AND
    instance_id = 18 AND (start_ts >= '2016-11-27 00:00:00' AND start_ts < '2016-12-05 00:00:00')
GROUP BY point;

/* ClickHouse */
SELECT
    intDiv((1480888800 - toRelativeSecondNum(start_ts)), 11520) as point,
    toDateTime(1480888800 - point * 11520) AS ts,

        /*  Basic metrics */
        SUM(query_count) / 11520 AS query_count_per_sec,
        SUM(Query_time_sum) / 11520 AS query_time_sum_per_sec,
        SUM(Lock_time_sum) / 11520 AS lock_time_sum_per_sec,
        SUM(Rows_sent_sum) / 11520 AS rows_sent_sum_per_sec,
        SUM(Rows_examined_sum) / 11520 AS rows_examined_sum_per_sec


        /* Perf Schema or Percona Server */
        , /* <-- final comma for basic metrics */
        SUM(Rows_affected_sum) / 11520 AS rows_affected_sum_per_sec,
        SUM(Merge_passes_sum) / 11520 AS merge_passes_sum_per_sec,
        SUM(Full_join_sum) / 11520 AS full_join_sum_per_sec,
        SUM(Full_scan_sum) / 11520 AS full_scan_sum_per_sec,
        SUM(Tmp_table_sum) / 11520 AS tmp_table_sum_per_sec,
        SUM(Tmp_table_on_disk_sum) / 11520 AS tmp_table_on_disk_sum_per_sec,


    /* Percona Server */
        SUM(Bytes_sent_sum) / 11520 AS bytes_sent_sum_per_sec,
        SUM(InnoDB_IO_r_ops_sum) / 11520 AS innodb_io_r_ops_sum_per_sec,

        SUM(InnoDB_IO_r_wait_sum) / 11520 AS innodb_io_r_wait_sum_per_sec,
        SUM(InnoDB_rec_lock_wait_sum) / 11520 AS innodb_rec_lock_wait_sum_per_sec,
        SUM(InnoDB_queue_wait_sum) / 11520 AS innodb_queue_wait_sum_per_sec,

        SUM(InnoDB_IO_r_bytes_sum) / 11520 AS innodb_io_r_bytes_sum_per_sec,
        SUM(QC_Hit_sum) / 11520 AS qc_hit_sum_per_sec,
        SUM(Filesort_sum) / 11520 AS filesort_sum_per_sec,
        SUM(Filesort_on_disk_sum) / 11520 AS filesort_on_disk_sum_per_sec,
        SUM(Tmp_tables_sum) / 11520 AS tmp_tables_sum_per_sec,
        SUM(Tmp_disk_tables_sum) / 11520 AS tmp_disk_tables_sum_per_sec,
        SUM(Tmp_table_sizes_sum) / 11520 AS tmp_table_sizes_sum_per_sec
FROM  query_class_metrics
WHERE  query_class_id = 64 AND
    instance_id = 18 AND (start_ts >= '2016-11-27 00:00:00' AND start_ts < '2016-12-05 00:00:00')
GROUP BY point;
