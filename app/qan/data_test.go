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
	"database/sql"
	"encoding/json"
	"io"
	"io/ioutil"
	"time"

	"github.com/percona/pmm/proto"
	qp "github.com/percona/pmm/proto/qan"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/qan"
	"github.com/percona/qan-api/config"
	"github.com/percona/qan-api/service/query"
	"github.com/percona/qan-api/stats"
	"github.com/percona/qan-api/test"
	"github.com/percona/qan-api/test/mock"
	testDb "github.com/percona/qan-api/tests/setup/db"
	. "gopkg.in/check.v1"
)

type DataTestSuite struct {
	dbh           *qan.MySQLMetricWriter
	db            *sql.DB
	testDb        *testDb.Db
	mini          *query.Mini
	wsConn        *mock.WebsocketConnector
	sendDataChan  chan interface{}
	recvDataChan  chan interface{}
	sendBytesChan chan []byte
	recvBytesChan chan []byte
}

var _ = Suite(&DataTestSuite{})

func (s *DataTestSuite) SetUpSuite(t *C) {
	dsn := config.Get("mysql.dsn")
	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	err := s.testDb.Start()
	t.Assert(err, IsNil)

	s.mini = query.NewMini(config.ApiRootDir + "/service/query")
	go s.mini.Run()

	// Create instance handler
	ih := instance.NewMySQLHandler(db.DBManager)

	// Make real dbh.
	s.dbh = qan.NewMySQLMetricWriter(db.DBManager, ih, s.mini, stats.NullStats())

	// Make aux MySQL connection.
	s.db, err = sql.Open("mysql", dsn)
	err = s.db.Ping()
	t.Assert(err, IsNil)

	s.sendDataChan = make(chan interface{}, 1)
	s.recvDataChan = make(chan interface{}, 1)
	s.sendBytesChan = make(chan []byte, 1)
	s.recvBytesChan = make(chan []byte, 1)
	s.wsConn = mock.NewWebsocketConnector(s.sendDataChan, s.recvDataChan, s.sendBytesChan, s.recvBytesChan)
}

func (s *DataTestSuite) SetUpTest(t *C) {
	s.testDb.TruncateDataTables()
}

// --------------------------------------------------------------------------

func (s *DataTestSuite) TestSaveData(t *C) {
	// Load data for report
	data, err := ioutil.ReadFile(config.ApiRootDir + "/test/qan/002/data_v3.json")
	t.Assert(err, IsNil)
	report := &qp.Report{}
	err = json.Unmarshal(data, report)
	t.Assert(err, IsNil)

	// Wrap the report in a proto.Data.
	now, _ := time.Parse("2006-01-02T15:04:05", "2014-04-16T18:17:58")
	report.StartTs = now.Add(-1 * time.Second)
	report.EndTs = now
	report.StartOffset = 0
	report.EndOffset = 1000
	report.SlowLogFile = "slow.log"
	reportData, _ := json.Marshal(report)
	pData := &proto.Data{
		ProtocolVersion: "1.0",
		Data:            reportData,
	}
	dataBytes, _ := json.Marshal(pData)

	// Call SaveData which will wait on wsConn.RecvBytes().
	errChan := make(chan error, 1)
	go func() {
		errChan <- qan.SaveData(s.wsConn, 2, s.dbh, stats.NullStats())
	}()

	// Send data, wait for a response.
	s.sendBytesChan <- dataBytes
	v := <-s.recvDataChan
	r := v.(proto.Response)
	t.Check(r.Code, Equals, uint(200))
	t.Check(r.Error, Equals, "")

	// Simulate agent closing the connection.
	s.wsConn.RecvError(io.EOF)
	s.sendBytesChan <- nil

	// Wait for SaveData to return.
	err = <-errChan
	t.Check(err, Equals, io.EOF)

	if diff := test.TableDiff(s.testDb.DB(), "query_class_metrics", "query_class_id,instance_id", config.TestDir+"/qan/002/qcm.tab"); diff != "" {
		t.Error(diff)
	}
}
