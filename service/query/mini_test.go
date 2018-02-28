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
		{
			"select c from t where id=?",
			"SELECT t",
			[]qp.Table{{Db: "", Table: "t"}},
		},
		{ // #1
			"select c from db.t where id=?",
			"SELECT db.t",
			[]qp.Table{{Db: "db", Table: "t"}},
		},
		{ // #2
			"select c from db.t, t2 where id=?",
			"SELECT db.t t2",
			[]qp.Table{
				{Db: "db", Table: "t"},
				{Db: "", Table: "t2"},
			},
		},
		{ // #3
			"SELECT /*!40001 SQL_NO_CACHE */ * FROM `film`",
			"SELECT film",
			[]qp.Table{{Db: "", Table: "film"}},
		},
		{ // #4
			"select c from ta join tb on (ta.id=tb.id) where id=?",
			"SELECT ta tb",
			[]qp.Table{
				{Db: "", Table: "ta"},
				{Db: "", Table: "tb"},
			},
		},
		{ // #5
			"select c from ta join tb on (ta.id=tb.id) join tc on (1=1) where id=?",
			"SELECT ta tb tc",
			[]qp.Table{
				{Db: "", Table: "ta"},
				{Db: "", Table: "tb"},
				{Db: "", Table: "tc"},
			},
		},

		/////////////////////////////////////////////////////////////////////
		// INSERT
		{ // #6
			"INSERT INTO my_table (a,b,c) VALUES (1, 2, 3)",
			"INSERT my_table",
			[]qp.Table{{Db: "", Table: "my_table"}},
		},
		{ // #7
			"INSERT INTO d.t (a,b,c) VALUES (1, 2, 3)",
			"INSERT d.t",
			[]qp.Table{{Db: "d", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// UPDATE
		{ // #8
			"update t set foo=?",
			"UPDATE t",
			[]qp.Table{{Db: "", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// DELETE
		{ // #9
			"delete from t where id in (?+)",
			"DELETE t",
			[]qp.Table{{Db: "", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// Other with partial support
		{ // #10
			"show status like ?",
			"SHOW STATUS",
			[]qp.Table{},
		},

		/////////////////////////////////////////////////////////////////////
		// Not support by sqlparser, falls back to mini.pl
		{ // #11
			"REPLACE INTO my_table (a,b,c) VALUES (1, 2, 3)",
			"REPLACE my_table",
			[]qp.Table{{Db: "", Table: "my_table"}},
		},
		{ // #12
			"OPTIMIZE TABLE `o2408`.`agent_log`",
			"OPTIMIZE `o2408`.`agent_log`",
			[]qp.Table{},
		},
		{ // #13
			"select c from t1 join t2 using (c) where id=?",
			"SELECT t1 t2",
			[]qp.Table{
				{Db: "", Table: "t1"},
				{Db: "", Table: "t2"},
			},
		},
		{ // #14
			"insert into data (?)",
			"INSERT data",
			[]qp.Table{},
		},
		{ // #15
			"call\n pita",
			"CALL pita",
			[]qp.Table{},
		},
		{ // #16 exceeds MAX_JOIN_DEPTH
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
			[]qp.Table{
				{"", "a"},
				{"", "b"}, {"", "c"}, {"", "d"}, {"", "e"},
				{"", "f"}, {"", "g"}, {"", "h"}, {"", "i"},
				{"", "j"}, {"", "k"}, {"", "l"}, {"", "m"},
				{"", "n"}, {"", "o"}, {"", "p"}, {"", "q"},
				{"", "r"}, {"", "s"}, {"", "t"}, {"", "u"},
				{"", "v"}, {"", "w"}, {"", "x"}, {"", "y"},
				{"", "z"},
			},
		},
		{ // #17
			"SELECT DISTINCT c\n FROM sbtest1\nWHERE id\nBETWEEN 1\nAND 100\nORDER BY  c\n",
			"SELECT sbtest1",
			[]qp.Table{{Db: "", Table: "sbtest1"}},
		},
		{ // #18
			"SELECT DISTINCT c FROM sbtest2 WHERE id BETWEEN 1 AND 100 ORDER BY c",
			"SELECT sbtest2",
			[]qp.Table{{Db: "", Table: "sbtest2"}},
		},
		// Don't remove the ; at the end of the next query.
		// There was an error in the past where a ; at the end was making the
		// parser to fail and we want to ensure it works now.
		{ // #19
			"SELECT * from `sysbenchtest`.`t6002_0`;",
			"SELECT sysbenchtest.t6002_0",
			[]qp.Table{{Db: "sysbenchtest", Table: "t6002_0"}},
		},
		{ // #20
			"use zapp",
			"USE",
			[]qp.Table{},
		},
		// Schema was set as default from the previous USE
		{ // #21
			"SELECT * from `t6003_0`;",
			"SELECT t6003_0",
			[]qp.Table{{Db: "", Table: "t6003_0"}},
		},
		{ // #22
			"CREATE TABLE t6004 (id int, a varchar(25)) engine=innodb",
			"CREATE TABLE t6004",
			[]qp.Table{{Db: "", Table: "t6004"}},
		},
		{ // #23
			"ALTER TABLE sakila.actor ADD COLUMN newcol int",
			"ALTER TABLE sakila.actor",
			[]qp.Table{{Db: "sakila", Table: "actor"}},
		},
		// Db & Table are empty because CREATE DATABASE is not yet supported by Vitess.sqlparser
		{ // #24
			"CREATE DATABASE percona",
			"CREATE DATABASE percona",
			[]qp.Table{},
		},
		{ // #25
			"create index idx ON percona (f1)",
			"CREATE index",
			[]qp.Table{{Db: "", Table: "percona"}},
		},
		{ // #26 override the default USE
			"create index idx ON brannigan.percona (f1)",
			"CREATE index",
			[]qp.Table{{Db: "brannigan", Table: "percona"}},
		},
		// PMM-1892. Upgraded Vitess libraries to support this query.
		// Notice that the query below is not exactly the same reported in the ticket; this
		// query has `auto_increment` between backticks because it is a reserved MySQL word
		// but MySQL accepts it anyway as a field name while Vitess doesn't.
		{
			"SELECT table_schema, table_name, column_name, `auto_increment`, " +
				"pow(2, CASE data_type WHEN 'tinyint' THEN 7 WHEN 'smallint' " +
				"THEN 15 WHEN 'mediumint' THEN 23 WHEN 'int' THEN 31 WHEN 'bigint' " +
				"THEN 63 end+(column_type LIKE '% unsigned'))-1 AS max_int FROM " +
				"information_schema.tables t JOIN information_schema.columns c " +
				"USING (table_schema,table_name) WHERE c.extra = 'auto_increment' " +
				"AND t.auto_increment IS NOT NULL",
			"SELECT information_schema.tables information_schema.columns",
			[]qp.Table{
				{Db: "information_schema", Table: "tables"},
				{Db: "information_schema", Table: "columns"},
			},
		},
		{ // #28
			"SELECT @@`version`",
			"SELECT",
			[]qp.Table{},
		},
	}

	// query.NewMini.Parse() should be safe for parallel parsing.
	t.Run("examples", func(t *testing.T) {
		for _, defaultDb := range []string{"", "Little Bobby Tables"} {
			for i, e := range examples {
				t.Run(e.query, func(t *testing.T) {
					defaultDb := defaultDb
					i := i
					tables := make([]qp.Table, len(e.tables))
					copy(tables, e.tables)
					e := e
					e.tables = tables

					t.Parallel()
					q, err := m.Parse(e.query, "", defaultDb)
					if err != nil {
						t.Errorf("Error in test # %d: %s", i, err)
					}
					// If there is default db then expect it in tables too.
					if defaultDb != "" {
						for i := range e.tables {
							if e.tables[i].Db == "" {
								e.tables[i].Db = defaultDb
							}
						}
					}

					if q.Abstract != e.abstract {
						t.Errorf("Test # %d: abstracts are different.\nWant: %s\nGot: %s", i, e.abstract, q.Abstract)
					}
					if !reflect.DeepEqual(q.Tables, e.tables) {
						t.Errorf("Test # %d: tables are different.\nWant: %#v\nGot: %#v", i, e.tables, q.Tables)
					}
				})
			}
		}
	})

}
