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

package qan_test

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/daniel-nichter/deep-equal"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/qan"
	"github.com/percona/qan-api/config"
	"github.com/percona/qan-api/stats"
	"github.com/percona/qan-api/test"
	testDb "github.com/percona/qan-api/tests/setup/db"
	qp "github.com/percona/pmm/proto/qan"
	. "gopkg.in/check.v1"
)

type ReporterTestSuite struct {
	testDb    *testDb.Db
	nullStats *stats.Stats
	mysqlId   uint
}

var _ = Suite(&ReporterTestSuite{})

func (s *ReporterTestSuite) SetUpSuite(t *C) {
	// Create test_o1 database.
	dsn := config.Get("mysql.dsn")
	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	if err := s.testDb.Start(); err != nil {
		t.Fatalf("Could not prepare org db: %s", err)
	}

	nullStats := stats.NewStats(&statsd.NoopClient{}, "test", "localhost", "mm", "1.0")
	s.nullStats = &nullStats

	s.mysqlId = 1259
}

func (s *ReporterTestSuite) SetUpTest(t *C) {
	s.testDb.TruncateDataTables()
}

func (s *ReporterTestSuite) TearDownTest(t *C) {
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

func (s *ReporterTestSuite) TestSimple(t *C) {
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/may-2015")
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/may-2015/01")

	begin := time.Date(2015, time.May, 01, 0, 0, 0, 0, time.UTC)
	end := time.Date(2015, time.May, 02, 0, 0, 0, 0, time.UTC)
	r := qp.RankBy{
		Metric: "Query_time",
		Stat:   "sum",
		Limit:  5,
	}

	qr := qan.NewReporter(db.DBManager, s.nullStats)
	got, err := qr.Profile(s.mysqlId, begin, end, r)
	t.Check(err, IsNil)

	j, err := ioutil.ReadFile(config.TestDir + "/qan/profile/may-2015-01.json")
	t.Assert(err, IsNil)
	var expect qp.Profile
	err = json.Unmarshal(j, &expect)
	t.Assert(err, IsNil)

	diff, err := deep.Equal(got, expect)
	t.Assert(err, IsNil)
	t.Check(diff, IsNil)
}

func (s *ReporterTestSuite) Test003(t *C) {
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/003")

	begin := time.Date(2013, time.July, 01, 0, 0, 0, 0, time.UTC)
	end := time.Date(2013, time.July, 02, 0, 0, 0, 0, time.UTC)
	r := qp.RankBy{
		Metric: "Query_time",
		Stat:   "sum",
		Limit:  5,
	}

	qr := qan.NewReporter(db.DBManager, s.nullStats)
	got, err := qr.Profile(3, begin, end, r)
	t.Check(err, IsNil)

	j, err := ioutil.ReadFile(config.TestDir + "/qan/profile/003-01.json")
	t.Assert(err, IsNil)
	var expect qp.Profile
	err = json.Unmarshal(j, &expect)
	t.Assert(err, IsNil)

	diff, err := deep.Equal(got, expect)
	t.Assert(err, IsNil)
	t.Check(diff, IsNil)
}
