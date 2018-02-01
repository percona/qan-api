/*
   Copyright (c) 2016, Percona LLC and/or its affiliates. All rights reserved.

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package query_test

import (
	"reflect"
	"testing"

	qp "github.com/percona/pmm/proto/query"
	"github.com/percona/qan-api/config"
	"github.com/percona/qan-api/service/query"
)

func TestParse(t *testing.T) {
	m := query.NewMini(config.ApiRootDir + "/service/query")
	go m.Run()
	defer m.Stop()

	//m.Debug = true

	type example struct {
		query    string
		abstract string
		tables   []qp.Table
	}
	examples := []example{
		/////////////////////////////////////////////////////////////////////
		// SELECT
		example{
			"select c from t where id=?",
			"SELECT t",
			[]qp.Table{qp.Table{Db: "", Table: "t"}},
		},
		example{ // #1
			"select c from db.t where id=?",
			"SELECT db.t",
			[]qp.Table{qp.Table{Db: "db", Table: "t"}},
		},
		example{ // #2
			"select c from db.t, t2 where id=?",
			"SELECT db.t t2",
			[]qp.Table{
				qp.Table{Db: "db", Table: "t"},
				qp.Table{Db: "", Table: "t2"},
			},
		},
		example{ // #3
			"SELECT /*!40001 SQL_NO_CACHE */ * FROM `film`",
			"SELECT film",
			[]qp.Table{qp.Table{Db: "", Table: "film"}},
		},
		example{ // #4
			"select c from ta join tb on (ta.id=tb.id) where id=?",
			"SELECT ta tb",
			[]qp.Table{
				qp.Table{Db: "", Table: "ta"},
				qp.Table{Db: "", Table: "tb"},
			},
		},
		example{ // #5
			"select c from ta join tb on (ta.id=tb.id) join tc on (1=1) where id=?",
			"SELECT ta tb tc",
			[]qp.Table{
				qp.Table{Db: "", Table: "ta"},
				qp.Table{Db: "", Table: "tb"},
				qp.Table{Db: "", Table: "tc"},
			},
		},

		/////////////////////////////////////////////////////////////////////
		// INSERT
		example{ // #6
			"INSERT INTO my_table (a,b,c) VALUES (1, 2, 3)",
			"INSERT my_table",
			[]qp.Table{qp.Table{Db: "", Table: "my_table"}},
		},
		example{ // #7
			"INSERT INTO d.t (a,b,c) VALUES (1, 2, 3)",
			"INSERT d.t",
			[]qp.Table{qp.Table{Db: "d", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// UPDATE
		example{ // #8
			"update t set foo=?",
			"UPDATE t",
			[]qp.Table{qp.Table{Db: "", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// DELETE
		example{ // #9
			"delete from t where id in (?+)",
			"DELETE t",
			[]qp.Table{qp.Table{Db: "", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// Other with partial support
		example{ // #10
			"show status like ?",
			"SHOW STATUS",
			[]qp.Table{},
		},

		/////////////////////////////////////////////////////////////////////
		// Not support by sqlparser, falls back to mini.pl
		example{ // #11
			"REPLACE INTO my_table (a,b,c) VALUES (1, 2, 3)",
			"REPLACE my_table",
			[]qp.Table{qp.Table{Db: "", Table: "my_table"}},
		},
		example{ // #12
			"OPTIMIZE TABLE `o2408`.`agent_log`",
			"OPTIMIZE `o2408`.`agent_log`",
			[]qp.Table{},
		},
		example{ // #13
			"select c from t1 join t2 using (c) where id=?",
			"SELECT t1 t2",
			[]qp.Table{
				qp.Table{Db: "", Table: "t1"},
				qp.Table{Db: "", Table: "t2"},
			},
		},
		example{ // #14
			"insert into data (?)",
			"INSERT data",
			[]qp.Table{},
		},
		example{ // #15
			"call\n pita",
			"CALL pita",
			[]qp.Table{},
		},
		example{ // #16 exceeds MAX_JOIN_DEPTH
			"select c from a" +
				" join b on (1=1) join c on (1=1) join d on (1=1) join e on (1=1)" +
				" join f on (1=1) join g on (1=1) join h on (1=1) join i on (1=1)" +
				" join j on (1=1) join k on (1=1) join l on (1=1) join m on (1=1)" +
				" join n on (1=1) join o on (1=1) join p on (1=1) join q on (1=1)" +
				" join r on (1=1) join s on (1=1) join t on (1=1) join u on (1=1)" +
				" join v on (1=1) join w on (1=1) join x on (1=1) join y on (1=1)" +
				" join z on (1=1)" +
				" where id=?",
			"SELECT a b c d e f g h i j k l m n o p q r s t u v w x y z",
			[]qp.Table{},
		},
		example{ // #17 exceeds MAX_JOIN_DEPTH
			"SELECT DISTINCT c\n FROM sbtest1\nWHERE id\nBETWEEN 1\nAND 100\nORDER BY  c\n",
			"SELECT sbtest1",
			[]qp.Table{qp.Table{Db: "", Table: "sbtest1"}},
		},
		example{ // #18 exceeds MAX_JOIN_DEPTH
			"SELECT DISTINCT c FROM sbtest2 WHERE id BETWEEN 1 AND 100 ORDER BY c",
			"SELECT sbtest2",
			[]qp.Table{qp.Table{Db: "", Table: "sbtest2"}},
		},
		// Don't remove the ; at the end of the next query.
		// There was an error in the past where a ; at the end was making the
		// parser to fail and we want to ensure it works now.
		example{ // #19
			"SELECT * from `sysbenchtest`.`t6002_0`;",
			"SELECT sysbenchtest.t6002_0",
			[]qp.Table{qp.Table{Db: "sysbenchtest", Table: "t6002_0"}},
		},
		example{ // #20
			"use zapp",
			"USE",
			[]qp.Table{},
		},
		// Schema was set as default from the previous USE
		example{ // #21
			"SELECT * from `t6003_0`;",
			"SELECT zapp.t6003_0",
			[]qp.Table{qp.Table{Db: "zapp", Table: "t6003_0"}},
		},
		example{ // #22
			"CREATE TABLE t6004 (id int, a varchar(25)) engine=innodb",
			"CREATE TABLE t6004",
			[]qp.Table{qp.Table{Db: "zapp", Table: "t6004"}},
		},
		example{ // #23
			"ALTER TABLE sakila.actor ADD COLUMN newcol int",
			"ALTER TABLE sakila.actor",
			[]qp.Table{qp.Table{Db: "sakila", Table: "actor"}},
		},
		// Db & Table are empty because CREATE DATABASE is not yet supported by Vitess.sqlparser
		example{ // #24
			"CREATE DATABASE percona",
			"CREATE DATABASE percona",
			[]qp.Table{},
		},
		example{ // #25
			"create index idx ON percona (f1)",
			"CREATE index",
			[]qp.Table{qp.Table{Db: "zapp", Table: "percona"}},
		},
		example{ // #26 override the default USE
			"create index idx ON brannigan.percona (f1)",
			"CREATE index",
			[]qp.Table{qp.Table{Db: "brannigan", Table: "percona"}},
		},
	}

	for i, e := range examples {
		q, err := m.Parse(e.query, "")
		if err != nil {
			t.Errorf("Error in test # %d: %s", i, err)
		}
		if q.Abstract != e.abstract {
			t.Errorf("Test # %d: abstracts are different.\nWant: %s\nGot: %s", i, e.abstract, q.Abstract)
		}
		if !reflect.DeepEqual(q.Tables, e.tables) {
			t.Errorf("Test # %d: tables are different.\nWant: %v\nGot: %v", i, e.tables, q.Tables)
		}
	}
}

func TestUpgradeVitess(t *testing.T) {
	m := query.NewMini(config.ApiRootDir + "/service/query")
	go m.Run()
	defer m.Stop()

	//m.Debug = true

	type example struct {
		query    string
		abstract string
		tables   []qp.Table
	}
	query := "SELECT table_schema, table_name, column_name, `auto_increment`, " +
		"pow(2, CASE data_type WHEN 'tinyint' THEN 7 WHEN 'smallint' " +
		"THEN 15 WHEN 'mediumint' THEN 23 WHEN 'int' THEN 31 WHEN 'bigint' " +
		"THEN 63 end+(column_type LIKE '% unsigned'))-1 AS max_int FROM " +
		"information_schema.tables t JOIN information_schema.columns c " +
		"USING (table_schema,table_name) WHERE c.extra = 'auto_increment' " +
		"AND t.auto_increment IS NOT NULL"
	examples := []example{
		/////////////////////////////////////////////////////////////////////
		// SELECT
		example{
			query,
			"SELECT information_schema.tables information_schema.columns",
			[]qp.Table{
				qp.Table{Db: "information_schema", Table: "tables"},
				qp.Table{Db: "information_schema", Table: "columns"},
			},
		},
	}

	for i, e := range examples {
		q, err := m.Parse(e.query, "")
		if err != nil {
			t.Errorf("Error in test # %d: %s", i, err)
		}
		if q.Abstract != e.abstract {
			t.Errorf("Test # %d: abstracts are different.\nWant: %s\nGot: %s", i, e.abstract, q.Abstract)
		}
		if !reflect.DeepEqual(q.Tables, e.tables) {
			t.Errorf("Test # %d: tables are different.\nWant: %v\nGot: %v", i, e.tables, q.Tables)
		}
	}
}
