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

package metrics_test

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/daniel-nichter/deep-equal"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/metrics"
	"github.com/percona/qan-api/config"
	"github.com/percona/qan-api/stats"
	"github.com/percona/qan-api/test"
	testDb "github.com/percona/qan-api/tests/setup/db"
	mp "github.com/percona/pmm/proto/metrics"
	. "gopkg.in/check.v1"
)

type QueryTestSuite struct {
	testDb    *testDb.Db
	orgDb     *sql.DB
	nullStats *stats.Stats
	mysqlId   uint
}

var _ = Suite(&QueryTestSuite{})

func (s *QueryTestSuite) SetUpSuite(t *C) {
	dsn := config.Get("mysql.dsn")
	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	if err := s.testDb.Start(); err != nil {
		t.Fatalf("Could not prepare org db: %s", err)
	}

	nullStats := stats.NewStats(&statsd.NoopClient{}, "test", "localhost", "mm", "1.0")
	s.nullStats = &nullStats

	s.mysqlId = 1259
}

func (s *QueryTestSuite) SetUpTest(t *C) {
	s.testDb.DB().Exec("TRUNCATE TABLE query_classes")
	s.testDb.DB().Exec("TRUNCATE TABLE query_examples")
	s.testDb.DB().Exec("TRUNCATE TABLE query_class_metrics")
	s.testDb.DB().Exec("TRUNCATE TABLE query_global_metrics")
}

func (s *QueryTestSuite) TearDownTest(t *C) {
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
	s.testDb.TruncateAllTables(s.testDb.DB())
}

// --------------------------------------------------------------------------

func (s *QueryTestSuite) TestSimple(t *C) {
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/may-2015")
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/may-2015/01")

	begin := time.Date(2015, time.May, 01, 0, 0, 0, 0, time.UTC)
	end := time.Date(2015, time.May, 02, 0, 0, 0, 0, time.UTC)

	// query_class_id=141 is the SELECT statement
	mh := metrics.NewQueryMetricsHandler(db.DBManager, s.nullStats)
	got, err := mh.Get(s.mysqlId, 141, begin, end)
	t.Check(err, IsNil)

	j, err := ioutil.ReadFile(config.TestDir + "/metrics/may-2015-01.json")
	t.Assert(err, IsNil)
	var expect map[string]mp.Stats
	err = json.Unmarshal(j, &expect)
	t.Assert(err, IsNil)

	diff, err := deep.Equal(got, expect)
	t.Assert(err, IsNil)
	t.Check(diff, IsNil)
}
