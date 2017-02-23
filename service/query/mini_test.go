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
	"testing"

	. "github.com/go-test/test"
	qp "github.com/percona/pmm/proto/query"
	"github.com/percona/qan-api/config"
	"github.com/percona/qan-api/service/query"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type MiniTestSuite struct {
}

var _ = Suite(&MiniTestSuite{})

func (s *MiniTestSuite) TestParse(t *C) {
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
		example{
			"select c from db.t where id=?",
			"SELECT db.t",
			[]qp.Table{qp.Table{Db: "db", Table: "t"}},
		},
		example{
			"select c from db.t, t2 where id=?",
			"SELECT db.t t2",
			[]qp.Table{
				qp.Table{Db: "db", Table: "t"},
				qp.Table{Db: "", Table: "t2"},
			},
		},
		example{
			"SELECT /*!40001 SQL_NO_CACHE */ * FROM `film`",
			"SELECT film",
			[]qp.Table{qp.Table{Db: "", Table: "film"}},
		},
		example{
			"select c from ta join tb on (ta.id=tb.id) where id=?",
			"SELECT ta tb",
			[]qp.Table{
				qp.Table{Db: "", Table: "ta"},
				qp.Table{Db: "", Table: "tb"},
			},
		},
		example{
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
		example{
			"INSERT INTO my_table (a,b,c) VALUES (1, 2, 3)",
			"INSERT my_table",
			[]qp.Table{qp.Table{Db: "", Table: "my_table"}},
		},
		example{
			"INSERT INTO d.t (a,b,c) VALUES (1, 2, 3)",
			"INSERT d.t",
			[]qp.Table{qp.Table{Db: "d", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// UPDATE
		example{
			"update t set foo=?",
			"UPDATE t",
			[]qp.Table{qp.Table{Db: "", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// DELETE
		example{
			"delete from t where id in (?+)",
			"DELETE t",
			[]qp.Table{qp.Table{Db: "", Table: "t"}},
		},

		/////////////////////////////////////////////////////////////////////
		// Other with partial support
		example{
			"show status like ?",
			"SHOW STATUS",
			[]qp.Table{},
		},

		/////////////////////////////////////////////////////////////////////
		// Not support by sqlparser, falls back to mini.pl
		example{
			"REPLACE INTO my_table (a,b,c) VALUES (1, 2, 3)",
			"REPLACE my_table",
			[]qp.Table{},
		},
		example{
			"OPTIMIZE TABLE `o2408`.`agent_log`",
			"OPTIMIZE `o2408`.`agent_log`",
			[]qp.Table{},
		},
		example{
			"select c from t1 join t2 using (c) where id=?",
			"SELECT t1 t2",
			[]qp.Table{},
		},
		example{
			"insert into data (?)",
			"INSERT data",
			[]qp.Table{},
		},
		example{
			"call\n pita",
			"CALL pita",
			[]qp.Table{},
		},
		example{ // exceeds MAX_JOIN_DEPTH
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
	}

	for _, e := range examples {
		q, err := m.Parse(e.query, "")
		t.Check(err, IsNil)
		t.Check(q.Abstract, Equals, e.abstract)
		if ok, diff := IsDeeply(q.Tables, e.tables); !ok {
			t.Log(e.query)
			Dump(q.Tables) // got
			t.Error(diff)
		}
	}
}
