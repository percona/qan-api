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
	"reflect"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	qp "github.com/percona/pmm/proto/qan"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/qan"
	"github.com/percona/qan-api/config"
	"github.com/percona/qan-api/service/query"
	"github.com/percona/qan-api/stats"
	"github.com/percona/qan-api/test"
	testDb "github.com/percona/qan-api/tests/setup/db"
	. "gopkg.in/check.v1"
)

type MySQLTestSuite struct {
	testDb    *testDb.Db
	ih        instance.DbHandler
	m         *query.Mini
	nullStats *stats.Stats
	qcmPK     string
}

var _ = Suite(&MySQLTestSuite{})

func (s *MySQLTestSuite) SetUpSuite(t *C) {
	dsn := config.Get("mysql.dsn")
	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	if err := s.testDb.Start(); err != nil {
		t.Fatalf("Could not prepare org db for %s: %s", dsn, err)
	}

	// Run the query minifier (distil).
	s.m = query.NewMini(config.ApiRootDir + "/service/query")
	go s.m.Run()

	nullStats := stats.NewStats(&statsd.NoopClient{}, "test", "localhost", "mm", "1.0")
	s.nullStats = &nullStats

	s.qcmPK = "query_class_id,instance_id,start_ts"

	// Create instance handler
	s.ih = instance.NewMySQLHandler(db.DBManager)
}

func (s *MySQLTestSuite) SetUpTest(t *C) {
	s.testDb.TruncateDataTables()
}

func (s *MySQLTestSuite) TearDownTest(t *C) {
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

func (s *MySQLTestSuite) TestSimple(t *C) {
	var err error

	qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)

	data1, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/data1_v3.json")
	t.Assert(err, IsNil)

	report1 := qp.Report{}
	err = json.Unmarshal(data1, &report1)
	t.Assert(err, IsNil)

	data2, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/data2_v3.json")
	t.Assert(err, IsNil)

	report2 := qp.Report{}
	err = json.Unmarshal(data2, &report2)

	t.Assert(err, IsNil)
	now, _ := time.Parse("2006-01-02T15:04:05", "2014-04-16T18:17:58")

	report1.StartTs = now.Add(-3 * time.Second)
	report1.EndTs = now.Add(-2 * time.Second)
	report1.StartOffset = 0
	report1.EndOffset = 1000
	err = qanHandler.Write(report1)
	t.Assert(err, IsNil)

	report2.StartTs = now.Add(-2 * time.Second)
	report2.EndTs = now.Add(-1 * time.Second)
	report2.StartOffset = 0
	report2.EndOffset = 1000
	err = qanHandler.Write(report2)
	t.Assert(err, IsNil)

	if diff := test.TableDiff(s.testDb.DB(), "query_global_metrics", "instance_id,start_ts", config.TestDir+"/qan/001/qgm.tab"); diff != "" {
		t.Error(diff)
	}
	if diff := test.TableDiff(s.testDb.DB(), "query_class_metrics", s.qcmPK, config.TestDir+"/qan/001/qcm.tab"); diff != "" {
		t.Error(diff)
	}
}

func (s *MySQLTestSuite) TestQueryExample(t *C) {
	var err error

	now, _ := time.Parse("2006-01-02T15:04:05", "2014-04-16T18:17:58")

	qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)

	data1, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/slow001_v3.json")
	t.Assert(err, IsNil)
	report1 := qp.Report{}
	err = json.Unmarshal(data1, &report1)
	t.Assert(err, IsNil)
	report1.StartTs = now.Add(-3 * time.Second)
	report1.EndTs = now.Add(-2 * time.Second)
	err = qanHandler.Write(report1)
	t.Assert(err, IsNil)
	if diff := test.TableDiff(s.testDb.DB(), "query_examples", "query_class_id", config.TestDir+"/qan/001/query_examples_1.tab"); diff != "" {
		t.Error(diff)
	}

	// Example should be updated when Query_time is greater.
	data2, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/slow002_v3.json")
	t.Assert(err, IsNil)
	report2 := qp.Report{}
	err = json.Unmarshal(data2, &report2)
	t.Assert(err, IsNil)
	report2.StartTs = now.Add(-2 * time.Second)
	report2.EndTs = now.Add(-1 * time.Second)
	err = qanHandler.Write(report2)
	t.Assert(err, IsNil)
	if diff := test.TableDiff(s.testDb.DB(), "query_examples", "query_class_id", config.TestDir+"/qan/001/query_examples_2.tab"); diff != "" {
		t.Error(diff)
	}
}

func (s *MySQLTestSuite) TestUpdateTablesInClasses(t *C) {
	var err error

	now, _ := time.Parse("2006-01-02T15:04:05", "2014-04-16T18:17:58")

	qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)

	data1, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/slow001_v3.json")
	t.Assert(err, IsNil)
	report1 := qp.Report{}
	err = json.Unmarshal(data1, &report1)
	t.Assert(err, IsNil)
	report1.StartTs = now.Add(-3 * time.Second)
	report1.EndTs = now.Add(-2 * time.Second)
	err = qanHandler.Write(report1)
	t.Assert(err, IsNil)
	if diff := test.TableDiff(s.testDb.DB(), "query_examples", "query_class_id", config.TestDir+"/qan/001/query_examples_1.tab"); diff != "" {
		t.Error(diff)
	}

	// Cleanup tables field in query classes to test if they are getting updated
	s.testDb.DB().Exec("UPDATE query_classes set tables = ''")

	// Example should be updated when Query_time is greater.
	data2, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/slow002_v3.json")
	t.Assert(err, IsNil)
	report2 := qp.Report{}
	err = json.Unmarshal(data2, &report2)
	t.Assert(err, IsNil)
	report2.StartTs = now.Add(-2 * time.Second)
	report2.EndTs = now.Add(-1 * time.Second)
	err = qanHandler.Write(report2)
	t.Assert(err, IsNil)
	if diff := test.TableDiff(s.testDb.DB(), "query_examples", "query_class_id", config.TestDir+"/qan/001/query_examples_2.tab"); diff != "" {
		t.Error(diff)
	}

	var gotTables []string
	wantTables := []string{
		"[{\"Db\":\"test\",\"Table\":\"n\"}]", "[{\"Db\":\"\",\"Table\":\"n\"}]",
	}

	rows, err := s.testDb.DB().Query("SELECT `tables` FROM query_classes")
	t.Assert(err, IsNil)
	for rows.Next() {
		var tables string
		err := rows.Scan(&tables)
		t.Assert(err, IsNil)
		gotTables = append(gotTables, tables)
	}
	t.Assert(rows.Err(), IsNil)
	if !reflect.DeepEqual(gotTables, wantTables) {
		t.Errorf("tables samples in query classes were not updated")
	}
}

func (s *MySQLTestSuite) TestQueryExampleUsesDefaultDb(t *C) {
	var err error

	data, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/slow001_v3.json")
	t.Assert(err, IsNil)

	report := qp.Report{}
	err = json.Unmarshal(data, &report)
	t.Assert(err, IsNil)

	now, _ := time.Parse("2006-01-02T15:04:05", "2014-04-16T18:17:58")
	report.StartTs = now.Add(-3 * time.Second)
	report.EndTs = now.Add(-2 * time.Second)
	report.Class = report.Class[0:1]
	report.Class[0].Fingerprint = "select c from t where id=?"
	report.Class[0].Example.Query = "select c from t where id=100"

	qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)
	err = qanHandler.Write(report)
	t.Assert(err, IsNil)

	var tables string
	s.testDb.DB().QueryRow("SELECT tables FROM query_classes WHERE checksum='3A99CC42AEDCCFCD'").Scan(&tables)
	t.Check(tables, Equals, `[{"Db":"test","Table":"t"}]`)
}

func (s *MySQLTestSuite) TestLastSeen(t *C) {
	t.Skip("todo")
	var err error

	data1, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/data1_v3.json")
	t.Assert(err, IsNil)

	report1 := qp.Report{}
	err = json.Unmarshal(data1, &report1)
	t.Assert(err, IsNil)

	now, _ := time.Parse("2006-01-02T15:04:05", "2014-04-16T18:17:58")

	report1.StartTs = now.Add(-61 * time.Minute).UTC()
	report1.EndTs = now.Add(-60 * time.Minute).UTC()
	report1.StartOffset = 0
	report1.EndOffset = 1000

	// Create qanHandler
	qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)

	// Write metrics first time to set last seen.
	err = qanHandler.Write(report1)
	t.Assert(err, IsNil)

	// first_seen should be set to class.Query.Ts, or interval.StartTs.
	// In this case it's the latter because the former isn't present in
	// the input.
	var firstSeen, lastSeen time.Time
	err = s.testDb.DB().QueryRow("SELECT first_seen, last_seen FROM query_classes WHERE checksum='1000000000000001'").Scan(&firstSeen, &lastSeen)
	t.Assert(err, IsNil)
	t.Check(lastSeen.Unix(), Equals, report1.StartTs.Unix())
	if lastSeen.Before(firstSeen) {
		t.Errorf("First seen is > last seen %v : %v", firstSeen, lastSeen)
	}

	// Next, later interval to pretend like query was last seen more recently.
	report1.StartTs = now.Add(-1 * time.Minute).UTC()
	report1.EndTs = now.Add(-0 * time.Minute).UTC()
	report1.StartOffset = 2000
	report1.EndOffset = 3000

	err = qanHandler.Write(report1)
	t.Assert(err, IsNil)

	// first_seen should be updated to the new interval.StartTs (because there's
	// no class.Query.Ts in the input).
	err = s.testDb.DB().QueryRow("SELECT last_seen FROM query_classes WHERE checksum='1000000000000001'").Scan(&lastSeen)
	t.Assert(err, IsNil)
	t.Check(lastSeen.Unix(), Equals, report1.StartTs.Unix())
}

func (s *MySQLTestSuite) TestBadFingerprints(t *C) {
	t.Skip("todo")
	var err error

	data1, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/005/bad-fingerprint001_v3.json")
	t.Assert(err, IsNil)
	report := qp.Report{}
	err = json.Unmarshal(data1, &report)
	t.Assert(err, IsNil)

	now := time.Now()

	report.StartTs = now
	report.EndTs = now

	// Create qanHandler
	qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)

	// Write metrics first time to set last seen.
	err = qanHandler.Write(report)
	t.Assert(err, IsNil)

	var fingerprint string
	err = s.testDb.DB().QueryRow("SELECT fingerprint FROM query_classes WHERE checksum='1000000000000001'").Scan(&fingerprint)
	t.Assert(err, IsNil)
	t.Check(fingerprint, Equals, "call  pita")
}

func (s *MySQLTestSuite) TestWorkaroundPSBug830286(t *C) {
	t.Skip("todo")
	// https://bugs.launchpad.net/percona-server/+bug/830286
	var err error

	data, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/005/ps-bug-830286_v3.json")
	t.Assert(err, IsNil)
	report := qp.Report{}
	err = json.Unmarshal(data, &report)
	t.Assert(err, IsNil)

	now := time.Now()
	report.StartTs = now
	report.EndTs = now
	report.StartOffset = 0
	report.EndOffset = 1000

	qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)

	err = qanHandler.Write(report)
	t.Assert(err, IsNil)

	// @todo Below code is dead. Not sure what it ought to test.
	var val uint64
	s.testDb.DB().QueryRow("SELECT Rows_read_max FROM query_global_samples WHERE query_global_sample_id=1").Scan(&val)
	t.Check(val, Equals, uint64(0))
	s.testDb.DB().QueryRow("SELECT Rows_read_max FROM query_class_samples WHERE query_class_sample_id=1").Scan(&val)
	t.Check(val, Equals, uint64(0))
}

func (s *MySQLTestSuite) TestRateLimit(t *C) {
	t.Skip("todo")
	var err error

	data1, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/slow003_v3.json")
	t.Assert(err, IsNil)

	report1 := qp.Report{}
	err = json.Unmarshal(data1, &report1)
	t.Assert(err, IsNil)

	now, _ := time.Parse("2006-01-02T15:04:05", "2014-04-16T18:17:58")

	report1.StartTs = now.Add(-3 * time.Second)
	report1.EndTs = now.Add(-2 * time.Second)
	report1.SlowLogFile = "slow.log"

	report1.StartOffset = 0
	report1.EndOffset = 1000

	qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)

	err = qanHandler.Write(report1)
	t.Assert(err, IsNil)

	if diff := test.TableDiff(s.testDb.DB(), "query_global_metrics", "query_global_metrics_id", config.TestDir+"/qan/001/query_global_metrics_3.tab"); diff != "" {
		t.Error(diff)
	}
	if diff := test.TableDiff(s.testDb.DB(), "query_class_metrics", "query_class_metrics_id", config.TestDir+"/qan/001/query_class_metrics_3.tab"); diff != "" {
		t.Error(diff)
	}

	// Now we are going to check with rate limit set
	// Do some clean up to keep qet4.tab as simple as possible because it already
	// has a lot of fields
	s.testDb.TruncateDataTables()
	// Prepare the data with rate limit set
	data2, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/slow004_v3.json")
	t.Assert(err, IsNil)

	report2 := qp.Report{}
	err = json.Unmarshal(data2, &report2)
	t.Assert(err, IsNil)

	report2.StartTs = now.Add(-3 * time.Second)
	report2.EndTs = now.Add(-2 * time.Second)
	report2.SlowLogFile = "slow.log"
	report2.StartOffset = 0
	report2.EndOffset = 1000

	// Write metrics and check results
	err = qanHandler.Write(report2)
	t.Assert(err, IsNil)

	if diff := test.TableDiff(s.testDb.DB(), "query_global_metrics", "query_global_metrics_id", config.TestDir+"/qan/001/query_global_metrics_4.tab"); diff != "" {
		t.Error(diff)
	}
	if diff := test.TableDiff(s.testDb.DB(), "query_class_metrics", "query_class_metrics_id", config.TestDir+"/qan/001/query_class_metrics_4.tab"); diff != "" {
		t.Error(diff)
	}

	// Test empty rateType. rateType and rateLimit must be nil and the
	// multiplier must be 1
	s.testDb.TruncateDataTables()
	// Prepare the data with rate limit set
	data3, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/001/slow005_v3.json")
	t.Assert(err, IsNil)

	report3 := qp.Report{}
	err = json.Unmarshal(data3, &report3)
	t.Assert(err, IsNil)

	report3.StartTs = now.Add(-3 * time.Second)
	report3.EndTs = now.Add(-2 * time.Second)
	report3.SlowLogFile = "slow.log"
	report3.StartOffset = 0
	report3.EndOffset = 1000

	// Write metrics and check results
	err = qanHandler.Write(report3)
	t.Assert(err, IsNil)

	if diff := test.TableDiff(s.testDb.DB(), "query_global_metrics", "query_global_metrics_id", config.TestDir+"/qan/001/query_global_metrics_5.tab"); diff != "" {
		t.Error(diff)
	}
	if diff := test.TableDiff(s.testDb.DB(), "query_class_metrics", "query_class_metrics_id", config.TestDir+"/qan/001/query_class_metrics_5.tab"); diff != "" {
		t.Error(diff)
	}
}

func (s *MySQLTestSuite) Test009(t *C) {
	// PCT-861: Add Performance Schema metrics to query report
	// https://jira.percona.com/browse/PCT-861
	// Input is 2 query classes with values for all 10 new perf schema metrics.
	/*
		input := config.ApiRootDir + "/test/qan/009/"

		var err error

		// Make a qanHandler.
		qanHandler := qan.NewMySQLMetricWriter(db.DBManager, s.ih, s.m, s.nullStats)

		// Read input, parse into a qan.Result, then make into a qan.Report.
		data, err := ioutil.ReadFile(input + "data1.json")
		t.Assert(err, IsNil)
		result := &qan.Result{}
		err = json.Unmarshal(data, &result)
		t.Assert(err, IsNil)
		now, _ := time.Parse("2006-01-02T15:04:05", "2014-04-16T18:17:58")
		interval := &qan.Interval{
			StartTime: now.Add(-1 * time.Minute),
			StopTime:  now,
		}
		report := qan.MakeReport(qan.Config{ServiceInstance: s.si}, interval, result)

		// Write the report.
		err = qanHandler.WriteV2( report)
		t.Assert(err, IsNil)

		// Check the data in the tables.
		if diff := test.TableDiff(s.testDb.DB(), "query_global_metrics", "query_global_metrics_id", input+"query_global_metrics.tab"); diff != "" {
			t.Error(diff)
		}
		if diff := test.TableDiff(s.testDb.DB(), "query_class_metrics", "query_class_metrics_id", input+"query_class_metrics.tab"); diff != "" {
			t.Error(diff)
		}
	*/
}
