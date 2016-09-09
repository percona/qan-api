package models

import (
	"log"

	_ "github.com/go-sql-driver/mysql" // do we need this here?
	"github.com/jmoiron/sqlx"

	"github.com/percona/qan-api/config"
)

var db *sqlx.DB

func init() {
	var err error
	dsn := config.Get("mysql.dsn")
	db, err = sqlx.Connect("mysql", dsn)

	if err != nil {
		log.Fatalln(err)
	} else {
		log.Println("connected to db.")
	}
}
