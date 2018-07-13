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

package models_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"
	"fmt"

	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/config"
	"github.com/percona/qan-api/test"
	testDb "github.com/percona/qan-api/tests/setup/db"
	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
)

type ReporterTestSuite struct {
	testDb    *testDb.Db
	mysqlId   uint
}

var _ = Suite(&ReporterTestSuite{})

func (s *ReporterTestSuite) SetUpSuite(t *C) {
	// Create test_o1 database.
	dsn := config.Get("mysql.dsn")
	fmt.Println(dsn, config.SchemaDir, config.TestDir)
	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	if err := s.testDb.Start(); err != nil {
		t.Fatalf("Could not prepare org db for %s: %s", dsn, err)
	}

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
	t.Fatalf("Prepared_stmt_count=%d, expected 0.", ps)
}

// --------------------------------------------------------------------------

func (s *ReporterTestSuite) TestSimple(t *C) {
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/may-2015")
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/may-2015/01")

	begin := time.Date(2015, time.May, 01, 0, 0, 0, 0, time.UTC)
	end := time.Date(2015, time.May, 02, 0, 0, 0, 0, time.UTC)
	r := models.RankBy{
		Metric: "Query_time",
		Stat:   "sum",
		Limit:  5,
	}

	got, err := models.Report.Profile(s.mysqlId, begin, end, r, 0, "", false)
	t.Check(err, IsNil)

	expectedFile := config.TestDir + "/qan/profile/may-2015-01.json"
	updateTestData := os.Getenv("UPDATE_TEST_DATA")
	if updateTestData != "" {
		data, _ := json.MarshalIndent(&got, "", "  ")
		ioutil.WriteFile(expectedFile, data, 0666)
	}

	j, err := ioutil.ReadFile(expectedFile)
	t.Assert(err, IsNil)
	var expect models.Profile
	err = json.Unmarshal(j, &expect)
	t.Assert(err, IsNil)

	assert.Equal(t, expect, got)
}

func (s *ReporterTestSuite) Test003(t *C) {
	s.testDb.LoadDataInfiles(config.TestDir + "/qan/003")

	begin := time.Date(2013, time.July, 01, 0, 0, 0, 0, time.UTC)
	end := time.Date(2013, time.July, 02, 0, 0, 0, 0, time.UTC)
	r := models.RankBy{
		Metric: "Query_time",
		Stat:   "sum",
		Limit:  5,
	}

	got, err := models.Report.Profile(3, begin, end, r, 0, "", false)
	t.Check(err, IsNil)

	expectedFile := config.TestDir + "/qan/profile/003-01.json"
	updateTestData := os.Getenv("UPDATE_TEST_DATA")
	if updateTestData != "" {
		data, _ := json.MarshalIndent(&got, "", "  ")
		ioutil.WriteFile(expectedFile, data, 0666)

	}

	gotData, err := json.Marshal(got)
	t.Assert(err, IsNil)

	expectData, err := ioutil.ReadFile(expectedFile)
	t.Assert(err, IsNil)

	assert.JSONEq(t, string(expectData), string(gotData))
}
