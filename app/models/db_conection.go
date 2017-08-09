package models

import (
	"log"

	_ "github.com/go-sql-driver/mysql" // register MySQL driver
	"github.com/jmoiron/sqlx"
	_ "github.com/kshvakov/clickhouse" // register ClickHouse  driver
	_ "github.com/mattn/go-sqlite3"    // register SQLite  driver

	"github.com/percona/qan-api/config"
)

// ConnectionsPool contains connection to databases;
type ConnectionsPool struct {
	MySQL      *sqlx.DB
	SQLite     *sqlx.DB
	ClickHouse *sqlx.DB
}

// NewConnectionsPool esteblish connection to databases or die.
func NewConnectionsPool() *ConnectionsPool {
	var err error
	conns := &ConnectionsPool{}

	dsn := config.Get("mysql.dsn")
	conns.MySQL, err = sqlx.Connect("mysql", dsn)

	if err != nil {
		log.Fatalln(err)
	} else {
		log.Println("Connected to MySQL DB.")
	}

	sqliteDb := config.Get("sqlite.db")
	conns.SQLite, err = sqlx.Connect("sqlite3", sqliteDb)

	if err != nil {
		log.Fatalln(err)
	} else {
		log.Println("Connected to SQLite DB.")
	}

	clickhouseDsn := config.Get("clickhouse.dsn")
	conns.ClickHouse, err = sqlx.Open("clickhouse", clickhouseDsn)
	conns.ClickHouse.Ping()
	if err != nil {
		log.Fatal(err)
	} else {
		log.Println("Connected to ClickHouse DB.")
	}
	return conns
}
