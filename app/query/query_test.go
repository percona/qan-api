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
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	queryProto "github.com/percona/pmm/proto/query"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/query"
	"github.com/percona/qan-api/config"
	queryService "github.com/percona/qan-api/service/query"
	"github.com/percona/qan-api/stats"
	"github.com/percona/qan-api/test"
	testDb "github.com/percona/qan-api/tests/setup/db"
	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type TestSuite struct {
	testDb    *testDb.Db
	nullStats *stats.Stats
}

var _ = Suite(&TestSuite{})

func (s *TestSuite) SetUpSuite(t *C) {
	// Create test_o1 database.
	dsn := config.Get("mysql.dsn")
	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	if err := s.testDb.Start(); err != nil {
		t.Fatalf("Could not prepare org db for %s: %s", dsn, err)
	}

	nullStats := stats.NewStats(&statsd.NoopClient{}, "test", "localhost", "mm", "1.0")
	s.nullStats = &nullStats
}

func (s *TestSuite) SetUpTest(t *C) {
	s.testDb.TruncateDataTables()
}

func (s *TestSuite) TearDownTest(t *C) {
	// Ensure that no prepared statements are leaked (not closed).
	// There's a little race condition here: MySQL seems not to close
	// stmts instantly, so check twice.
	ps := uint(0)
	for i := 0; i < 2; i++ {
		if ps = test.PrepStmtCount(s.testDb.DB()); ps == 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Prepared_stmt_count=%d, expected 0", ps)
}

// --------------------------------------------------------------------------

func (s *TestSuite) TestSimple(t *C) {
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/may-2015")

	qh := query.NewMySQLHandler(db.DBManager, s.nullStats)
	got, err := qh.Get([]string{"92F3B1B361FB0E5B", "94350EA2AB8AAC34", "407C5D658AA2568B"})
	t.Check(err, IsNil)

	j, err := ioutil.ReadFile(config.TestDir + "/query/may-2015.json")
	t.Assert(err, IsNil)
	var expect map[string]queryProto.Query
	err = json.Unmarshal(j, &expect)
	t.Assert(err, IsNil)

	assert.Equal(t, expect, got)
}

func (s *TestSuite) TestTables(t *C) {
	m := queryService.NewMini(config.ApiRootDir + "/service/query")
	go m.Run()
	defer m.Stop()

	s.testDb.LoadDataInfiles(config.TestDir + "/qan/may-2015")

	qh := query.NewMySQLHandler(db.DBManager, s.nullStats)
	got, err := qh.Tables(328, m)
	assert.NoError(t, err)
	assert.Equal(t, []queryProto.Table([]queryProto.Table{{Db: "percona", Table: "cache"}}), got)
}
