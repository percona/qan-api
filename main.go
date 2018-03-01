// +build api2

package main

import (
	"fmt"
	"net/http"
	"path"

	"github.com/jmoiron/sqlx"
	_ "github.com/kshvakov/clickhouse" // register ClickHouse driver
	"github.com/mattes/migrate"
	_ "github.com/mattes/migrate/database/clickhouse"
	_ "github.com/mattes/migrate/database/sqlite3"
	bindata "github.com/mattes/migrate/source/go-bindata"
	_ "github.com/mattn/go-sqlite3" // register SQLite  driver
	"github.com/percona/qan-api/migrations"
	log "github.com/sirupsen/logrus"
)

const clickHouseDSN = "127.0.0.1:9000?database=pmm&read_timeout=10&write_timeout=20&debug=true"
const sqliteDSN = "/srv/qan-api/pmm.sqlite"

const clickHousePath = "migrations/clickhouse"
const sqlitePath = "migrations/sqlite"

const bind = "127.0.0.1:9001"

func main() {

	if err := runMigrations(); err != nil {
		log.Fatal("Migrations: ", err)
	}
	log.Println("Migrations applied.")

	clickHouseConnection, err := sqlx.Open("clickhouse", "tcp://"+clickHouseDSN)
	clickHouseConnection.Ping()
	if err != nil {
		log.Fatal(err)
	} else {
		log.Info("Connected to ClickHouse DB.")
	}

	sqliteConnection, err := sqlx.Open("sqlite3", sqliteDSN)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Info("Connected to SQLite.")
	}

	_ = sqliteConnection

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Info("Requested: /")
		fmt.Fprintf(w, "Hello world!")
	})
	log.Info("ListenAndServe: " + bind)
	if err := http.ListenAndServe(bind, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

func runMigrations() error {

	// run ClickHouse migrations
	c, _ := migrations.AssetDir(clickHousePath)

	clickHouseMigrations := bindata.Resource(c,
		func(name string) ([]byte, error) {
			return migrations.Asset(path.Join(clickHousePath, name))
		})

	clickHouseData, err := bindata.WithInstance(clickHouseMigrations)
	if err != nil {
		return err
	}

	mc, err := migrate.NewWithSourceInstance("go-bindata", clickHouseData, "clickhouse://"+clickHouseDSN)
	if err != nil {
		return err
	}

	// run up to the latest migration
	err = mc.Up()
	if err == migrate.ErrNoChange {
		return nil
	}

	// run SQLite migrations
	s, _ := migrations.AssetDir(sqlitePath)

	sqliteMigrations := bindata.Resource(s,
		func(name string) ([]byte, error) {
			return migrations.Asset(path.Join(sqlitePath, name))
		})

	sqliteData, err := bindata.WithInstance(sqliteMigrations)
	if err != nil {
		return err
	}

	ms, err := migrate.NewWithSourceInstance("go-bindata", sqliteData, "sqlite3://"+sqliteDSN)
	if err != nil {
		return err
	}

	// run up to the latest migration
	err = ms.Up()
	if err == migrate.ErrNoChange {
		return nil
	}

	return err

}
