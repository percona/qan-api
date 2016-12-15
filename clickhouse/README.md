- export data from MySQL pmm db to CSV file
`sudo mysql pmm_perf_test < qan-api/clickhouse/pmm-export-from-mysql.sql`

- if need split CSV file into smaller chunk. Ex:.
`sudo split -l 100000 /var/lib/mysql-files/query_class_metrics.csv /tmp/pmm/query_class_metrics/`

- create db : 
`clickhouse-client --port 9090 --query="CREATE DATABASE pmm" `

- create tables: 
`clickhouse-client --port 9090 -n -m -d pmm < qan-api/clickhouse/pmm.ch.sql `

- import data into ClickHouse Db. Ex:.
`for f in sudo cat /tmp/pmm/query_class_metrics/*; do sudo cat $f | clickhouse-client --port 9090 --query="INSERT INTO pmm.query_class_metrics FORMAT CSV"; done`
