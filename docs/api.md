FORMAT: X-1A

# Percona Datastore API

#### Metadata

+ All data uses content type `application/json`
+ Standard [HTTP status codes](http://httpstatus.es/) are used, except where noted
+ Except for the agent, link relations are not returned with resources

#### Limitations

+ The API does little or not validation; user beware and be correct!

# Group Agents

Agents are special types of instances which represent installed (and deleted) instances of [Percona Agent](https://docs.google.com/a/percona.com/document/d/1rSMWyC-ykcwJmklbiwwJmZl03Zjosz9EQVDCzU2Jnnk/edit?usp=sharing).

## Agent [/agents]

+ Model

    ```js
    {
        UUID:       "521740123bae11e5a38e3aca4a148664",
        ParentUUID: "c0093fe29b9de9fa7387aaff091101ac",
        Hostname:   "tor.fipar.net",
        Version:    "1.0.0",
        Created:    "2008-01-01T00:00:00Z",
        Deleted:    "0000-00-00T00:00:00Z",
    }
    ```

### POST /agents
Create an agent. The `ParentUUID` is the UUID of the OS instance where the agent is running. `Name` is usually the server's hostname.

+ Request
    ```js
    {
        ParentUUID: "c0093fe29b9de9fa7387aaff091101ac",
        Name:       "tor.fipar.net",
        Version:    "1.0.0"
    }
    ```

+ Response 201
    + Headers
        Location: /agents/{uuid}

### GET /agents
List all agents. The agent has been logically deleted if `Deleted` is not zero.

+ Response 200

    ```js
    [
        {
            UUID:       "521740123bae11e5a38e3aca4a148664",
            ParentUUID: "c0093fe29b9de9fa7387aaff091101ac",
            Hostname:   "tor.fipar.net",
            Version:    "1.0.0",
            Created:    "2008-01-01T00:00:00Z",
            Deleted:    "0000-00-00T00:00:00Z"
        }
    ]
    ```

### GET /agents/{uuid}
Get an agent by UUID.

+ Response 200

    [Agent][]

### PUT /agents/{uuid}/cmd
Send a command to an agent. The request body is always a `Cmd` and the response is always a `Reply`. First, the `Cmd`:

```js
{
    Id        "b4135ac6-3d1b-11e5-a38e-3aca4a148664",
    Ts        "2015-01-01T00:00:00Z",
    User      "",
    AgentUUID "d00743103bae11e5a38e3cd04a148664"
    Service   "agent",
    Cmd       "Stop",
    Data:     "<base64 encoded data, if any>"
}
```

The fields are:

+ `Id`: command ID set by API; _do not_ set this! It's only show here for completeness.
+ `Ts`: optional timestamp when command was sent
+ `User`: optional user who sent the command
+ `AgentUUID`: required UUID of the agent to which the command is sent
+ `Service`: required name of the agent internal service to which the command is sent (see below)
+ `Cmd`: required command name, case-sensitive
+ `Data`: variable command data if `Cmd` requires it, _always base64 encoded in Cmd and Reply_ (JSON structure shown to demonstrate the data structure before encoding)

The most important and only strictly required fields are `AgentUUID`, `Service`, `Cmd`. Most commands require `Data`, which is base64 encoded. Command data depends on the `Service` and `Cmd`. The agent internal services and their commands are:

+ `agent` (the main agent process)
    + Abort (panic with stack trace)
    + Restart
    + Stop (normal shutdown)
    + Status (for all services)
    + Update (do self-update)
    + StartService (start one of the following services)
    + StopService (stop one of the following services)
    + GetConfig
    + GetAllConfigs
    + SetConfig
    + Version
+ `log` (online and file logger)
    + SetConfig
    + GetConfig
    + Reconnect
+ `data` (spooler and sender)
    + SetConfig
    + GetConfig
+ `qan` (Query Analytics tool)
    + StartService (start running tool on a MySQL instance)
    + StopService (stop running tool on a MySQL instance)
    + GetConfig
+ `instance` (instance repo)
    + UpdateInstance
    + GetInfo (get MySQL info)
+ `mrms` (MySQL Restart Monitoring Service)
    + (None currently)
+ `query`
    + Explain (EXPLAIN a query)
    + TableInfo (get table def, status, and indexes)

In response, the agent always returns a `Reply`:

```js
{
    Id:    "b4135ac6-3d1b-11e5-a38e-3aca4a148664",
    Cmd:   "Restart",
    Error: "",
    Data:  "<base64 encoded data, if any>"
}
```

`Error` is the important field: if empty, the command was successful, else it contains an error message.

+ Request

    Restart
    ```js
    {
        AgentUUID: "d00743103bae11e5a38e3cd04a148664",
        Service:   "agent",
        Cmd:       "Restart"
    }
    ```

    EXPLAIN
    ```js
    {
        AgentUUID: "d00743103bae11e5a38e3cd04a148664",
        Service:   "query",
        Cmd:       "Explain",
        Data:      {
            UUID:  "<MySQL UUID>",
            Db:    "mysql",
            Query: "select user, host from user where host = '%'"
        }
    }
    ```

    Table Info
    ```js
    {
        AgentUUID: "d00743103bae11e5a38e3cd04a148664",
        Service:   "query",
        Cmd:       "TableInfo",
        Data:      {
            UUID:  "<MySQL UUID>",
            Create: [
                {
                    Db:    "mysql",
                    Table: "user"
                }
            ],
            Index: [
                {
                    Db:    "mysql",
                    Table: "user"
                }
            ],
            Status: [
                {
                    Db:    "mysql",
                    Table: "user"
                }
            ]
        }
    }
    ```


+ Response 200

    Restart
    ```js
    {
        Id:    "b4135ac6-3d1b-11e5-a38e-3aca4a148664",
        Cmd:   "Restart",
        Error: ""
    }
    ```

    EXPLAIN
    ```js
    {
        Id:    "b4135ac6-3d1b-11e5-a38e-3aca4a148664",
        Cmd:   "Explain",
        Error:  "",
        Data:   {
            Classic: [
                {
                    Id:           1,
                    SelectType:   "ALL",
                    Table:        "user",
                    Partitions:   "", // split by comma
                    Type:         "ref",
                    PossibleKeys: "PRIMARY,idx1", // split by comma
                    Key:          "PRIMARY",
                    KeyLen:       "8",
                    Ref:          "user",
                    Rows:         123,
                    Extra:        "Using where" // split by semicolon
                }
            ],
            JSON: "<EXPLAIN FORMAT=JSON output as string>" // as of MySQL 5.6.5
        }
    }
    ```

    Table Info
    ```js
    {
        Id:    "b4135ac6-3d1b-11e5-a38e-3aca4a148664",
        Cmd:   "TableInfo",
        Error:  "",
        Data:   {
            "mysql.user": {
                Create: "<SHOW CREATE TABLE output as string>",
                Index: {
                    "PRIMARY": {
                        Table:        "user",
                        NonUnique:    false,
                        KeyName:      "PRIMARY",
                        SeqInIndex:   0,
                        ColumnName:   "user",
                        Collation:    "latin1",
                        Cardinality:  16,
                        SubPart:      null,
                        Packed:       null,
                        Null:         null,
                        IndexType:    "BTREE",
                        Comment:      null,
                        IndexComment: null
                    },
                },
                Status: {
                    Name:          "user",
                    Engine:        "InnoDB",
                    Version:       "Antelope",
                    RowFormat:     "compact",
                    Rows:          32,
                    AvgRowLength:  11012,
                    DataLength:    102421942,
                    MaxDataLength: 4567839103921,
                    IndexLength:   102402,
                    DataFree:      0,
                    AutoIncrement: null,
                    CreateTime:    "2015-01-01T00:00:00Z",
                    UpdateTime:    "2015-09-09T00:00:00Z",
                    CheckTime:     null,
                    Collation:     "utf8",
                    Checksum:      null,
                    CreateOptions: null,
                    Comment:       null
                },
                Errors: [] // error strings, if any
            }
        }
    }
    ```

+ Response 203

    ```js
    {
        Error: "Agent failed to execute command because..."
    }
    ```

+ Response 404

    ```js
    {
        Error: "agent not connected"
    }
    ```

### GET /agents/{uuid}/config
Get the agent config for all its services.

+ Response 200

    ```js
    {
        Agent: {
            UUID        "d00743103bae11e5a38e3cd04a148664",
            ApiHostname "fipar.tor.net",
            Keepalive   63,
            PidFile     "percona-agent.pid"
        },
        Data: {
            Encoding     string `json:",omitempty"`
            SendInterval uint   `json:",omitempty"`
            Blackhole    string `json:",omitempty"` // dev
            Limits: {
                MaxAge:  86400,
                MaxSize: 104857600,
                MaxFiles 1000
            }
        },
        Log: {
            Level:   "info",
            File:    "",
            Offline: false
        }
        QAN: [
            {
                UUID:              "521740123bae11e5a38e3aca4a148664",
                CollectFrom:       "slowlog",
                Start:             [],
                Stop:              [],
                Interval:          60,
                MaxSlowLogSize:    0,
                RemoveOldSlowLogs: "true",
                ExampleQueries:    "true",
                WorkerRunTime:     50,
                ReportLimit:       200
            }
        ]
    }
    ```

### GET /agents/{uuid}/log?begin,end,minLevel,maxLevel,service
Get the agent log during the given time range, between the given log levels, and matching (like) the internal service.

Required Args:
+ begin: ISO timestamp, UTC (`2015-01-01T00:00:00`)
+ end: ISO timestamp, UTC (`2015-01-02T00:00:00`)

Optional Args:
+ minLevel: log level, 3 (error) to 7 (debug), inclusive
+ maxLevel: log level, 3 (error) to 7 (debug), inclusive
+ service: agent internal service name in `LIKE` clause

Log Levels:
+ 3 - error
+ 4 - warning
+ 5 - notice
+ 6 - info
+ 7 - debug

The response is a list of `LogEntry` resources, sorted ascending by timestamp.

+ Response 200

    ```js
    [
        {
            Ts:      "2015-08-07T01:43:40.189941883Z",
            Level:   6,
            Service: "log",
            Msg:     "Started"
        },
        {
            Ts:      "2015-08-07T01:43:40.199983794Z",
            Level:   6,
            Service: "mrms-manager",
            Msg:     "Started"
        }
    ]
    ```

### PUT /agents/{uuid}/status
Get the real-time status of a running agent. The same can be done by calling `PUT /agents/{uuid}/cmd` with the `Status` command, but the `PUT` returns a `Reply` whereas this route only returns the status, keyed on agent internal services and sub-services. The status values are human-readable, they are not intended for programmatically use, matching, or analysis.

+ Response 200

    ```js
    {
        "agent": "Idle",
        "agent-cmd-handler": "Idle",
        "agent-ws": "Connected ws://tor.fipar.net:9001/agents/2635db7b00b54140569fc581f750a600/cmd",
        "agent-ws-link": "ws://tor.fipar.net:9001/agents/2635db7b00b54140569fc581f750a600/cmd",
        "data": "Running",
        "data-sender": "Idle",
        "data-sender-1d": "",
        "data-sender-last": "",
        "data-spooler": "Idle",
        "data-spooler-count": "1",
        "data-spooler-oldest": "2015-08-06 18:12:00.009243553 +0000 UTC",
        "data-spooler-size": "528.00 B",
        "data-ws": "",
        "data-ws-link": "ws://tor.fipar.net:9001/agents/2635db7b00b54140569fc581f750a600/data",
        "instance": "Running",
        "instance-mrms": "Idle",
        "instance-repo": "Idle",
        "log": "Running",
        "log-buf1": "0",
        "log-buf2": "0",
        "log-chan": "2",
        "log-file": "",
        "log-level": "info",
        "log-relay": "Idle",
        "log-ws": "Connected ws://tor.fipar.net:9001/agents/2635db7b00b54140569fc581f750a600/log",
        "log-ws-link": "ws://tor.fipar.net:9001/agents/2635db7b00b54140569fc581f750a600/log",
        "mrm-monitor": "Idle",
        "mrms": "Running",
        "qan": "Running",
        "qan-analyzer-tor.fipar.net": "Idle",
        "qan-analyzer-tor.fipar.net-last-interval": "",
        "qan-analyzer-tor.fipar.net-next-interval": "5.9s",
        "qan-analyzer-tor.fipar.net-worker": "",
        "query": "Idle",
    }
    ```

# Group Instances

Instances are running software and services. All data is associated with an instance.

Instances are identified by a UUID (without hyphens) and a user-configurable name. When an instance is created, the UUID can be set by the client or, when left blank, by the API.

Instances have a subsystem type which determines the type of instance and data. Currently, there are three subsystems:
+ os
+ agent
+ mysql

Instances have three major properties: DSN, distro, and version. The DSN specifies how to connect to the instance. *WARNING*: DSNs contain passwords and are sent and stored in cleartext. The distro and version are, for example, "Percona Server"/"5.6.26" for a MySQL instance.

Instances, except for OS instances, usually have a parent instance. The parent instance runs the child instance. For cases like Amazon RDS, there is no parent instance.

## Instance [/instances]

+ Model

    ```js
    {
        Subsystem:  "mysql",
        UUID:       "521740123bae11e5a38e3aca4a148664",
        ParentUUID: "c0093fe29b9de9fa7387aaff091101ac",
        Name:       "fipar-db-01",
        DSN:        "percona:percona@tcp(db01.fipar.net)/",
        Distro:     "MySQL",
        Version:    "3.23.55",
        Created:    "2015-07-01T00:00:00",
        Deleted:    "0000-00-00T00:00:00"
    }
    ```

### POST /instances
Create an instance. The only two fields are required: `Subystem` and `Name`.

+ Request
    + Body

        ```js
        {
            Subsystem:  "mysql",
            ParentUUID: "c0093fe29b9de9fa7387aaff091101ac",
            Name:       "fipar-db-01",
            DSN:        "percona:percona@tcp(db01.fipar.net)/",
            Distro:     "MySQL",
            Version:    "3.23.55"
        }
        ```

+ Response
    + Headers
        Location: /instances/{uuid}

### GET /instances/{uuid}
Get an instance by UUID.

+ Response 200
[Instance][]

### PUT /instances/{uuid}
Update an instance. Only `Name`, `DSN`, `Distro`, `Version`, and `ParentUUID` can be changed.

### DELETE /instances/{uuid}
Delete an instance. Data associated with the instance is not removed.

# Group Query Analytics
Query Analytics is a group of reports about query metrics (like average query time) from a slow log or the Performance Schema. Each route is a different report, and the resources vary accordingly. Reports have three attributes which allow clients to combine them:
+ MySQL instance UUID
+ begin time (UTC)
+ end time (UTC)

The begin and end times define the time range: `ts >= begin AND ts < end`. In addition to these three attributes, query-specific reports define a query ID.

## GET /qan/profile/{uuid}?begin,end
Get the top 10 slowest queries by total query time. A query profile ranks queries according to their percentage of the grand total value for a query metric statistic, in the given time range. The default query metric statistic is total query time (`Query_time_sum`). For example, if the grand total query time is 100s and some query has a total query time of 50s, then it accounts for 50% of the grand total. After the percentage for all queries is calculated, basic info (a profile) about the top 10 queries is returned.

Required Args:
+ begin: ISO timestamp, UTC (`2015-01-01T00:00:00`)
+ end: ISO timestamp, UTC (`2015-01-02T00:00:00`)

The response includes a list of queries where index 0 is the grand total value, and subsequent indexes correspond to the query ranks: `Query[1]` is the slowest query, `Query[2]` is the 2nd slowest, etc.

The `Rank` substructure is set by the API and is currently read-only.

`TotalTime` is the sum of existing data intervals. If it is less than the request time range, then there are gaps in the data. (Zero values are valid and included; gaps mean no data at all.) `QPS` and other values are computed using `TotalTime`.

`Load` is the ratio of query execution time to real interval time: `Query_time_sum / (End - Begin)`. This represents average concurrency. For example, if the interval time is 3600s (1h) and a query has 7200s of execution time, its load = 7200 / 3600 = 2. On average, the query was executing concurrently in 2 threads, which accounts for it having twice as much execution time as real time.

+ Response 200

    + Body

        ```js
        {
            InstanceId: "521740123bae11e5a38e3aca4a148664",
            Begin:      "2015-01-01T00:00:00",
            End:        "2015-01-02T00:00:00"
            TotalTime:  86400,
            RankBy: {
                Metric: "Query_time",
                Stat:   "sum",
                Limit:  10
            }
            Query: [
                {
                    Rank:       0,
                    Percentage: 0,
                    Id:         "",
                    Abstract:   "",
                    QPS:        503.848491,
                    Load:       1.0145,
                    Stats: {
                        Cnt: 100200333,
                        Sum: 100.0,
                        Min: 0.000001,
                        Avg: 0.101101,
                        Med: 0.900000,
                        P95: 0.555555,
                        Max: 1.999999,
                    }
                },
                {
                    Rank:       1,
                    Percentage: 80.75,
                    Id:         "94350EA2AB8AAC34",
                    Abstract:   "SELECT foo",
                    QPS:        500.1,
                    Load:       0.9001,
                    Stats: {
                        Cnt: 100033,
                        Sum: 80.8,
                        Min: 0.000001,
                        Avg: 1.101101,
                        Med: 1.550000,
                        P95: 1.000055,
                        Max: 5.999999,
                    }
                },
            ]
        }
        ```

## GET /qan/report/{uuid}/query/{queryId}?begin,end
Get a query report. This route is usually called after getting the query profile for the same time range. A query report provides full info and metrics about a query.

+ Response 200

    + Body

        ```js
        {
            InstanceId: "521740123bae11e5a38e3aca4a148664",
            Begin:      "2015-05-01T00:00:00Z",
            End:        "2015-05-02T00:00:00Z",
            Metrics: {
                Lock_time: {
                    Cnt: 112737,
                    Avg: 0.00046755606559,
                    Max: 0.11822800336828,
                    Med: 0,
                    Min: 0,
                    P95: 0.05693119094000,
                    Sum: 1.07500000586115
                },
                Query_time: {
                    Avg: 0.00055605306818,
                    Cnt: 112737,
                    Max: 0.23546500504076,
                    Med: 0.00045487173675,
                    Min: 0.49999993684468,
                    P95: 0.00139238274225,
                    Sum: 62.6877849483862
                },
                Row_exmined: {
                    Avg: 42.4091,
                    Cnt: 112737,
                    Max: 757,
                    Med: 0,
                    Min: 0,
                    P95: 345.1795,
                    Sum: 4781070.0
                },
                Row_set: {
                    Avg: 39.4602,
                    Cnt: 112737,
                    Max: 757,
                    Med: 0,
                    Min: 0,
                    P95: 311.3897,
                    Sum: 4448624.0
                }
            },
            Query: {
                Id:          "94350EA2AB8AAC34",
                Abstract:    "SELECT foo",
                Fingerprint: "select option_name, option_value from foo where autoload = ?",
                FirstSeen:   "2014-05-01T03:11:06Z",
                LastSeen:    "2015-06-18T21:36:00Z",
                Status:      "new",
                Tables:      [
                    {
                        Db: "db1",
                        Table: "foo"
                    }
                ]
            },
            Example: {
                Ts:        "2015-01-01T00:00:00Z",
                Db:        "o1",
                QueryTime: 0.00111,
                Query:     "SELECT option_name, option_value FROM foo WHERE autoload=1"
            }
        }
        ```

### GET /qan/config/{uuid}
Get the Query Analytics config for the given MySQL instance UUID. If no agent is running Query Analytics for the MySQL instance, `AgentUUID` will be an empty string.

+ Response 200

    ```js
    {
        AgentUUID: "d00743103bae11e5a38e3cd04a148664",
        Config: {
            UUID:              "521740123bae11e5a38e3aca4a148664",
            CollectFrom:       "slowlog",
            Start:             [],
            Stop:              [],
            Interval:          60,
            MaxSlowLogSize:    0,
            RemoveOldSlowLogs: "true",
            ExampleQueries:    "true",
            WorkerRunTime:     50,
            ReportLimit:       200            
        }
    }
    ```

# Group Queries

Query routes provide access to individual queries identified by a query ID, a 16- or 32-character hexadecimal value like `92F3B1B361FB0E5B`. Some routes require additional identifiers, like an instance UUID. Queries are independent of instances because the same logical query can be executed on many different instances. One exception is query examples: these are per-instance, per-day.

## Query [/queries]

+ Model

    ```js
    {
        Id:          "9C8DEE410FA0E0C8",
        Abstract:    "SELECT tbl1",
        Fingerprint: "select col from tbl1 where id=?",
        FirstSeen:   "2014-05-01T03:11:06Z",
        LastSeen:    "2015-06-18T21:36:00Z",
        Status:      "new",
        Tables:      [
            {
                Db: "db1",
                Table: "tbl1"
            }
        ]
    }
    ```

## GET /queries/{queryId}
Get a query by query ID.

+ Response 200

    [Query][]

## Tables [/queries/{queryId}/tables]

+ Model

    ```js
    [
        {
            Db:    "mysql",
            Table: "user"
        }
    ]
    ```

### GET /queries/{queryId}/tables
List the query's table. The API cannot parse every type of query, so the list can be empty. Use `PUT` to provide a table list manually.

+ Response 200

    [Tables][]

### PUT /queries/{queryId}/tables
Update a query's table list. The provided table list replaces the current one.

+ Request

    [Tables][]

 + Response 204


## Examples [/queries/{queryId}/exmaples]

+ Model

    ```js
    [
        {
            QueryId:      "9C8DEE410FA0E0C8",
            InstanceUUID: "521740123bae11e5a38e3aca4a148664",
            Period:       "2015-05-01T00:00:00Z",
            Ts:           "2015-05-01T04:39:00Z",
            Db:           "db1",
            QueryTime:    0.101352,
            Query:        "SELECT * FROM db1.foo WHERE id IN (1,2,3)"
        }
    ]
    ```

### GET /queries/{queryId}/examples?instance
List the query's examples. Query examples are stored per-instance, per-day (the period), and having the greatest query time. For example, if query X on instance Z executes 100 times today, only the one with the greatest query time is stored. Tomorrow, a new example is stored.

An optional `instance` argument can be provided to select a single instance by UUID. For example: `?instance=521740123bae11e5a38e3aca4a148664`.

The response is sorted descending by `Period`.

+ Response 200

    ```js
    [
        {
            QueryId:      "9C8DEE410FA0E0C8",
            InstanceUUID: "521740123bae11e5a38e3aca4a148664",
            Period:       "2015-05-01T00:00:00Z",
            Ts:           "2015-05-01T04:39:00Z",
            Db:           "db1",
            QueryTime:    0.101352,
            Query:        "SELECT * FROM db1.foo WHERE id IN (1,2,3)"
        }
    ]
    ```

### PUT /queries/{queryId}/tables
Update a query's example `DB`. Only `Db` can be changed. `QueryId`, `InstanceUUID`, and `Period` are required to identify a single query example.

+ Request

    ```js
    {
        QueryId:      "9C8DEE410FA0E0C8",
        InstanceUUID: "521740123bae11e5a38e3aca4a148664",
        Period:       "2015-05-01T00:00:00Z",
        Db:           "new-db1",
    }
    ```

 + Response 204
