/* agent_configs */
SELECT
    agent_instance_id,
    IFNULL(service, ''),
    other_instance_id,
    IFNULL(in_file, ''),
    IFNULL(running, ''),
    updated
FROM agent_configs 
INTO OUTFILE '/var/lib/mysql-files/agent_configs.csv'
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n';

/* agent_log */
SELECT
    IFNULL(instance_id, 0),
    IFNULL(sec, 0),
    IFNULL(nsec, 0),
    IFNULL(`level`, 0),
    IFNULL(service, 0),
    IFNULL(msg, 0)
FROM agent_log 
INTO OUTFILE '/var/lib/mysql-files/agent_log.csv'
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n';

/* instances */
SELECT
    IFNULL(instance_id, 0),
    IFNULL(subsystem_id, 0),
    IFNULL(parent_uuid, ''),
    IFNULL(uuid, ''),
    IFNULL(name, ''),
    IFNULL(dsn, ''),
    IFNULL(distro, ''),
    IFNULL(version, ''),
    created,
    IFNULL(deleted, 0)
FROM instances 
INTO OUTFILE '/var/lib/mysql-files/instances.csv'
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n';


/* query_class_metrics */
SELECT
    DATE(start_ts),
    IFNULL(query_class_id, 0),
    IFNULL(instance_id, 0),
    start_ts,
    end_ts,
    IFNULL(query_count, 0),
    IFNULL(lrq_count, 0),
    IFNULL(Query_time_sum, 0),
    IFNULL(Query_time_min, 0),
    IFNULL(Query_time_max, 0),
    IFNULL(Query_time_avg, 0),
    IFNULL(Query_time_p95, 0),
    IFNULL(Query_time_stddev, 0),
    IFNULL(Query_time_med, 0),
    IFNULL(Lock_time_sum, 0),
    IFNULL(Lock_time_min, 0),
    IFNULL(Lock_time_max, 0),
    IFNULL(Lock_time_avg, 0),
    IFNULL(Lock_time_p95, 0),
    IFNULL(Lock_time_stddev, 0),
    IFNULL(Lock_time_med, 0),
    IFNULL(Rows_sent_sum, 0),
    IFNULL(Rows_sent_min, 0),
    IFNULL(Rows_sent_max, 0),
    IFNULL(Rows_sent_avg, 0),
    IFNULL(Rows_sent_p95, 0),
    IFNULL(Rows_sent_stddev, 0),
    IFNULL(Rows_sent_med, 0),
    IFNULL(Rows_examined_sum, 0),
    IFNULL(Rows_examined_min, 0),
    IFNULL(Rows_examined_max, 0),
    IFNULL(Rows_examined_avg, 0),
    IFNULL(Rows_examined_p95, 0),
    IFNULL(Rows_examined_stddev, 0),
    IFNULL(Rows_examined_med, 0),
    IFNULL(Rows_affected_sum, 0),
    IFNULL(Rows_affected_min, 0),
    IFNULL(Rows_affected_max, 0),
    IFNULL(Rows_affected_avg, 0),
    IFNULL(Rows_affected_p95, 0),
    IFNULL(Rows_affected_stddev, 0),
    IFNULL(Rows_affected_med, 0),
    IFNULL(Rows_read_sum, 0),
    IFNULL(Rows_read_min, 0),
    IFNULL(Rows_read_max, 0),
    IFNULL(Rows_read_avg, 0),
    IFNULL(Rows_read_p95, 0),
    IFNULL(Rows_read_stddev, 0),
    IFNULL(Rows_read_med, 0),
    IFNULL(Merge_passes_sum, 0),
    IFNULL(Merge_passes_min, 0),
    IFNULL(Merge_passes_max, 0),
    IFNULL(Merge_passes_avg, 0),
    IFNULL(Merge_passes_p95, 0),
    IFNULL(Merge_passes_stddev, 0),
    IFNULL(Merge_passes_med, 0),
    IFNULL(InnoDB_IO_r_ops_sum, 0),
    IFNULL(InnoDB_IO_r_ops_min, 0),
    IFNULL(InnoDB_IO_r_ops_max, 0),
    IFNULL(InnoDB_IO_r_ops_avg, 0),
    IFNULL(InnoDB_IO_r_ops_p95, 0),
    IFNULL(InnoDB_IO_r_ops_stddev, 0),
    IFNULL(InnoDB_IO_r_ops_med, 0),
    IFNULL(InnoDB_IO_r_bytes_sum, 0),
    IFNULL(InnoDB_IO_r_bytes_min, 0),
    IFNULL(InnoDB_IO_r_bytes_max, 0),
    IFNULL(InnoDB_IO_r_bytes_avg, 0),
    IFNULL(InnoDB_IO_r_bytes_p95, 0),
    IFNULL(InnoDB_IO_r_bytes_stddev, 0),
    IFNULL(InnoDB_IO_r_bytes_med, 0),
    IFNULL(InnoDB_IO_r_wait_sum, 0),
    IFNULL(InnoDB_IO_r_wait_min, 0),
    IFNULL(InnoDB_IO_r_wait_max, 0),
    IFNULL(InnoDB_IO_r_wait_avg, 0),
    IFNULL(InnoDB_IO_r_wait_p95, 0),
    IFNULL(InnoDB_IO_r_wait_stddev, 0),
    IFNULL(InnoDB_IO_r_wait_med, 0),
    IFNULL(InnoDB_rec_lock_wait_sum, 0),
    IFNULL(InnoDB_rec_lock_wait_min, 0),
    IFNULL(InnoDB_rec_lock_wait_max, 0),
    IFNULL(InnoDB_rec_lock_wait_avg, 0),
    IFNULL(InnoDB_rec_lock_wait_p95, 0),
    IFNULL(InnoDB_rec_lock_wait_stddev , 0),
    IFNULL(InnoDB_rec_lock_wait_med, 0),
    IFNULL(InnoDB_queue_wait_sum, 0),
    IFNULL(InnoDB_queue_wait_min, 0),
    IFNULL(InnoDB_queue_wait_max, 0),
    IFNULL(InnoDB_queue_wait_avg, 0),
    IFNULL(InnoDB_queue_wait_p95, 0),
    IFNULL(InnoDB_queue_wait_stddev, 0),
    IFNULL(InnoDB_queue_wait_med, 0),
    IFNULL(InnoDB_pages_distinct_sum, 0),
    IFNULL(InnoDB_pages_distinct_min, 0),
    IFNULL(InnoDB_pages_distinct_max, 0),
    IFNULL(InnoDB_pages_distinct_avg, 0),
    IFNULL(InnoDB_pages_distinct_p95, 0),
    IFNULL(InnoDB_pages_distinct_stddev, 0),
    IFNULL(InnoDB_pages_distinct_med, 0),
    IFNULL(QC_Hit_sum, 0),
    IFNULL(Full_scan_sum, 0),
    IFNULL(Full_join_sum, 0),
    IFNULL(Tmp_table_sum, 0),
    IFNULL(Tmp_table_on_disk_sum, 0),
    IFNULL(Filesort_sum, 0),
    IFNULL(Filesort_on_disk_sum, 0),
    IFNULL(Query_length_sum, 0),
    IFNULL(Query_length_min, 0),
    IFNULL(Query_length_max, 0),
    IFNULL(Query_length_avg, 0),
    IFNULL(Query_length_p95, 0),
    IFNULL(Query_length_stddev, 0),
    IFNULL(Query_length_med, 0),
    IFNULL(Bytes_sent_sum, 0),
    IFNULL(Bytes_sent_min, 0),
    IFNULL(Bytes_sent_max, 0),
    IFNULL(Bytes_sent_avg, 0),
    IFNULL(Bytes_sent_p95, 0),
    IFNULL(Bytes_sent_stddev, 0),
    IFNULL(Bytes_sent_med, 0),
    IFNULL(Tmp_tables_sum, 0),
    IFNULL(Tmp_tables_min, 0),
    IFNULL(Tmp_tables_max, 0),
    IFNULL(Tmp_tables_avg, 0),
    IFNULL(Tmp_tables_p95, 0),
    IFNULL(Tmp_tables_stddev, 0),
    IFNULL(Tmp_tables_med, 0),
    IFNULL(Tmp_disk_tables_sum, 0),
    IFNULL(Tmp_disk_tables_min, 0),
    IFNULL(Tmp_disk_tables_max, 0),
    IFNULL(Tmp_disk_tables_avg, 0),
    IFNULL(Tmp_disk_tables_p95, 0),
    IFNULL(Tmp_disk_tables_stddev, 0),
    IFNULL(Tmp_disk_tables_med, 0),
    IFNULL(Tmp_table_sizes_sum, 0),
    IFNULL(Tmp_table_sizes_min, 0),
    IFNULL(Tmp_table_sizes_max, 0),
    IFNULL(Tmp_table_sizes_avg, 0),
    IFNULL(Tmp_table_sizes_p95, 0),
    IFNULL(Tmp_table_sizes_stddev, 0),
    IFNULL(Tmp_table_sizes_med, 0),
    IFNULL(Errors_sum, 0),
    IFNULL(Warnings_sum, 0),
    IFNULL(Select_full_range_join_sum, 0),
    IFNULL(Select_range_sum, 0),
    IFNULL(Select_range_check_sum, 0),
    IFNULL(Sort_range_sum, 0),
    IFNULL(Sort_rows_sum, 0),
    IFNULL(Sort_scan_sum, 0),
    IFNULL(No_index_used_sum, 0),
    IFNULL(No_good_index_used_sum, 0)
FROM query_class_metrics
INTO OUTFILE '/var/lib/mysql-files/query_class_metrics.csv'
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n';

/* query_classes */
SELECT
    IFNULL(query_class_id, 0),
    IFNULL(checksum, ''),
    IFNULL(abstract, ''),
    IFNULL(fingerprint, ''),
    IFNULL(tables, ''),
    first_seen,
    last_seen,
    status
FROM query_classes 
INTO OUTFILE '/var/lib/mysql-files/query_classes.csv'
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n';

/* query_examples */
SELECT
    IFNULL(query_class_id, 0),
    IFNULL(instance_id, 0),
    IFNULL(period, 0),
    IFNULL(ts, 0),
    IFNULL(db, 0),
    IFNULL(Query_time, 0),
    IFNULL(query, 0)
FROM query_examples
INTO OUTFILE '/var/lib/mysql-files/query_examples.csv'
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n';
