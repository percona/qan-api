CREATE TABLE IF NOT EXISTS `mongo_query_class_metrics` (
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
                            -- {"m_Query_time": 3.5, "m_Docs_returned": 5.9 }

    objects String, -- JSON ex.: {"tables": ["tbl1", "tbl2"]} TODO: what else? engine? slowlog|ps?? filter by autocreated labels?

    -- metrics
    m_Query_time_cnt Float32,
    m_Query_time_sum Float32,
    m_Query_time_min Float32,
    m_Query_time_max Float32,
    m_Query_time_p99 Float32,
    m_Query_time_hg Array(Float32), -- TODO: how to aggregate histograms and how fast is this aggregation?

    m_Docs_returned_cnt UInt64,
    m_Docs_returned_sum UInt64,
    m_Docs_returned_min UInt64,
    m_Docs_returned_max UInt64,
    m_Docs_returned_p99 UInt64,
    m_Docs_returned_hg Array(UInt64),

    m_Docs_scanned_cnt UInt64,
    m_Docs_scanned_sum UInt64,
    m_Docs_scanned_min UInt64,
    m_Docs_scanned_max UInt64,
    m_Docs_scanned_p99 UInt64,
    m_Docs_scanned_hg Array(UInt64),

    m_Response_length_cnt UInt64,
    m_Response_length_sum UInt64,
    m_Response_length_min UInt64,
    m_Response_length_max UInt64,
    m_Response_length_p99 UInt64,
    m_Response_length_hg Array(UInt64),

    -- TODO: do we need Errors & Warnings for mongo
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
