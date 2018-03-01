CREATE TABLE IF NOT EXISTS `mysql_query_class_metrics` (
    EventDate Date,

    -- dimentions
    digest String,  -- = query_class_id
    digest_text String, -- = fingerprint
    -- query_verb Enum8('OTHER'=0, 'SELECT'=1, 'INSERT'=2, 'UPDATE'=3, 'DELETE'=4, 'REPLACE'=5) DEFAULT 0, /* will be in labels */
    dsn_server String, /* localhost:3306 | socket. Do wee need to omit delfult port? TODO: How much we lost/win with split host and post in separate fields? */
    dsn_schema String, -- What to do if query selects from 2 or more dbs.
    dsn_username String,

    client_host String,
    labels Array(String), -- [label1, label_insert, label_db2]

    agent_uuid String, -- id of agent that sent data
    period_start DateTime,
    period_length UInt8,
    num_queries UInt64,

    -- TODO: do we need only one query example with exta metrics or many? otherwise we need to place this as nested data structure.
    example String,
    example_format Enum8('EXAMPLE'=0, 'DIGEST'=1),
    is_truncated UInt8, -- exemple_size ??
    example_type  Enum8('RANDOM'=0, 'SLOWEST'=1, 'FASTEST'=2, 'WITH_ERROR'=3),
    example_details String, -- JSON with mertics, error, warnings, any other non aggregatable stuff
                            -- {"m_Query_time": 3.5, "m_Lock_time": 5.9 }

    objects String, -- JSON ex.: {"tables": ["tbl1", "tbl2"]} TODO: what else? engine? slowlog|ps?? filter by autocreated labels?

    -- metrics
    m_Query_time_cnt Float32,
    m_Query_time_sum Float32,
    m_Query_time_min Float32,
    m_Query_time_max Float32,
    m_Query_time_p99 Float32,
    m_Query_time_hg Array(Float32), -- TODO: how to aggregate histograms and how fast is this aggregation?

    m_Lock_time_cnt Float32,
    m_Lock_time_sum Float32,
    m_Lock_time_min Float32,
    m_Lock_time_max Float32,
    m_Lock_time_p99 Float32,
    m_Lock_time_hg Array(Float32),

    m_Rows_sent_cnt UInt64,
    m_Rows_sent_sum UInt64,
    m_Rows_sent_min UInt64,
    m_Rows_sent_max UInt64,
    m_Rows_sent_p99 UInt64,
    m_Rows_sent_hg Array(UInt64),

    m_Rows_examined_cnt UInt64,
    m_Rows_examined_sum UInt64,
    m_Rows_examined_min UInt64,
    m_Rows_examined_max UInt64,
    m_Rows_examined_p99 UInt64,
    m_Rows_examined_hg Array(UInt64),

    m_Rows_affected_cnt UInt64,
    m_Rows_affected_sum UInt64,
    m_Rows_affected_min UInt64,
    m_Rows_affected_max UInt64,
    m_Rows_affected_p99 UInt64,
    m_Rows_affected_hg Array(UInt64),

    m_Rows_read_cnt UInt64,
    m_Rows_read_sum UInt64,
    m_Rows_read_min UInt64,
    m_Rows_read_max UInt64,
    m_Rows_read_p99 UInt64,
    m_Rows_read_hg Array(UInt64),

    m_Merge_passes_cnt UInt64,
    m_Merge_passes_sum UInt64,
    m_Merge_passes_min UInt64,
    m_Merge_passes_max UInt64,
    m_Merge_passes_p99 UInt64,
    m_Merge_passes_hg Array(UInt64),

    m_InnoDB_IO_r_ops_cnt UInt64,
    m_InnoDB_IO_r_ops_sum UInt64,
    m_InnoDB_IO_r_ops_min UInt64,
    m_InnoDB_IO_r_ops_max UInt64,
    m_InnoDB_IO_r_ops_p99 UInt64,
    m_InnoDB_IO_r_ops_hg Array(UInt64),

    m_InnoDB_IO_r_bytes_cnt UInt64,
    m_InnoDB_IO_r_bytes_sum UInt64,
    m_InnoDB_IO_r_bytes_min UInt64,
    m_InnoDB_IO_r_bytes_max UInt64,
    m_InnoDB_IO_r_bytes_p99 UInt64,
    m_InnoDB_IO_r_bytes_hg Array(UInt64),

    m_InnoDB_IO_r_wait_cnt Float32,
    m_InnoDB_IO_r_wait_sum Float32,
    m_InnoDB_IO_r_wait_min Float32,
    m_InnoDB_IO_r_wait_max Float32,
    m_InnoDB_IO_r_wait_p99 Float32,
    m_InnoDB_IO_r_wait_hg Array(Float32),

    m_InnoDB_rec_lock_wait_cnt Float32,
    m_InnoDB_rec_lock_wait_sum Float32,
    m_InnoDB_rec_lock_wait_min Float32,
    m_InnoDB_rec_lock_wait_max Float32,
    m_InnoDB_rec_lock_wait_p99 Float32,
    m_InnoDB_rec_lock_wait_hg Array(Float32),

    m_InnoDB_queue_wait_cnt Float32,
    m_InnoDB_queue_wait_sum Float32,
    m_InnoDB_queue_wait_min Float32,
    m_InnoDB_queue_wait_max Float32,
    m_InnoDB_queue_wait_p99 Float32,
    m_InnoDB_queue_wait_hg Array(Float32),

    m_InnoDB_pages_distinct_cnt UInt64,
    m_InnoDB_pages_distinct_sum UInt64,
    m_InnoDB_pages_distinct_min UInt64,
    m_InnoDB_pages_distinct_max UInt64,
    m_InnoDB_pages_distinct_p99 UInt64,
    m_InnoDB_pages_distinct_hg Array(UInt64),

    m_Query_length_cnt UInt64,
    m_Query_length_sum UInt64,
    m_Query_length_min UInt64,
    m_Query_length_max UInt64,
    m_Query_length_p99 UInt64,
    m_Query_length_hg Array(UInt64),

    m_Bytes_sent_cnt UInt64,
    m_Bytes_sent_sum UInt64,
    m_Bytes_sent_min UInt64,
    m_Bytes_sent_max UInt64,
    m_Bytes_sent_p99 UInt64,
    m_Bytes_sent_hg Array(UInt64),

    m_Tmp_tables_cnt UInt64,
    m_Tmp_tables_sum UInt64,
    m_Tmp_tables_min UInt64,
    m_Tmp_tables_max UInt64,
    m_Tmp_tables_p99 UInt64,
    m_Tmp_tables_hg Array(UInt64),

    m_Tmp_disk_tables_cnt UInt64,
    m_Tmp_disk_tables_sum UInt64,
    m_Tmp_disk_tables_min UInt64,
    m_Tmp_disk_tables_max UInt64,
    m_Tmp_disk_tables_p99 UInt64,
    m_Tmp_disk_tables_hg Array(UInt64),

    m_Tmp_table_sizes_cnt UInt64,
    m_Tmp_table_sizes_sum UInt64,
    m_Tmp_table_sizes_min UInt64,
    m_Tmp_table_sizes_max UInt64,
    m_Tmp_table_sizes_p99 UInt64,
    m_Tmp_table_sizes_hg Array(UInt64),

    -- TODO: do we need *_cnt, *_min, *_max, *_p99, *_hg - for next metrics?
    m_QC_Hit_sum UInt64,
    m_Full_scan_sum UInt64,
    m_Full_join_sum UInt64,
    m_Tmp_table_sum UInt64,
    m_Tmp_table_on_disk_sum UInt64,
    m_Filesort_sum UInt64,
    m_Filesort_on_disk_sum UInt64,
    m_Select_full_range_join_sum UInt64,
    m_Select_range_sum UInt64,
    m_Select_range_check_sum UInt64,
    m_Sort_range_sum UInt64,
    m_Sort_rows_sum UInt64,
    m_Sort_scan_sum UInt64,
    m_No_index_used_sum UInt64,
    m_No_good_index_used_sum UInt64,

    -- Errors & Warnings
    num_query_with_warnings UInt64,
    /* simply array of strings or JSON, if we do not need to aggregate this fields? */
    Warnings Nested
    (
        code String,
        count UInt64
    ),
    num_query_with_errors UInt64,
    /* Same Q as in Warning */
    Errors Nested
    (
        code String,
        count UInt64
    )
     /* what else here? */
) ENGINE = MergeTree(EventDate, (digest, dsn_server, dsn_username, dsn_schema, client_host, period_start), 8192);
