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

package agent_test

import (
	"time"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/agent"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/config"
	testDb "github.com/percona/qan-api/tests/setup/db"
	. "gopkg.in/check.v1"
)

type MySQLTestSuite struct {
	testDb *testDb.Db
}

var _ = Suite(&MySQLTestSuite{})

func (s *MySQLTestSuite) SetUpSuite(t *C) {
	dsn := config.Get("mysql.dsn")
	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	if err := s.testDb.Start(); err != nil {
		t.Fatalf("Could not prepare org db for %s: %s", dsn, err)
	}
}

func (s *MySQLTestSuite) SetUpTest(t *C) {
	s.testDb.DB().Exec("DELETE FROM instances WHERE instance_id > 3")
	s.testDb.DB().Exec("TRUNCATE TABLE agent_configs")
}

// --------------------------------------------------------------------------

func (s *MySQLTestSuite) TestSetConfigInternalService(t *C) {
	var (
		agentId uint   = 1
		service string = "log"
		set     []byte = []byte{}
		running []byte = []byte{}
	)

	// Create agentHandler
	agentHandler := agent.NewMySQLHandler(db.DBManager, instance.NewMySQLHandler(db.DBManager))

	// Update config (actual test)
	err := agentHandler.SetConfig(agentId, service, "", set, running)
	t.Assert(err, IsNil)

	var (
		gotAgentId uint
		gotOtherId uint
		gotService string
		gotSet     string
		gotRunning string
	)
	err = s.testDb.DB().QueryRow(
		"SELECT agent_instance_id, other_instance_id, service, in_file, running FROM agent_configs WHERE agent_instance_id=? AND other_instance_id=? AND service=?",
		agentId,
		0,
		service,
	).Scan(
		&gotAgentId,
		&gotOtherId,
		&gotService,
		&gotSet,
		&gotRunning,
	)
	t.Assert(err, IsNil)
	t.Check(gotAgentId, Equals, agentId)
	t.Check(gotOtherId, Equals, uint(0))
	t.Check(gotService, Equals, service)
	t.Check(gotSet, Equals, string(set))
	t.Check(gotRunning, Equals, string(running))

	// Do again to update existing config and check there's no dupe.
	set = []byte("new set")
	running = []byte("new running")
	err = agentHandler.SetConfig(agentId, service, "", set, running)
	t.Assert(err, IsNil)
	err = s.testDb.DB().QueryRow(
		"SELECT agent_instance_id, other_instance_id, service, in_file, running FROM agent_configs WHERE agent_instance_id=? AND other_instance_id=? AND service=?",
		agentId,
		0,
		service,
	).Scan(
		&gotAgentId,
		&gotOtherId,
		&gotService,
		&gotSet,
		&gotRunning,
	)
	t.Assert(err, IsNil)
	t.Check(gotAgentId, Equals, agentId)
	t.Check(gotOtherId, Equals, uint(0))
	t.Check(gotService, Equals, service)
	t.Check(gotSet, Equals, string(set))
	t.Check(gotRunning, Equals, string(running))

	// Let's check if there are no duplicates for key 42-"qan"
	var count int
	err = s.testDb.DB().QueryRow("SELECT COUNT(*) FROM agent_configs").Scan(&count)
	t.Assert(count, Equals, 1)
}

func (s *MySQLTestSuite) TestSetConfigTool(t *C) {
	var (
		agentId   uint   = 1
		service   string = "qan"
		otherId   uint   = 3
		mysqlUUID string = "313" // a MySQL instance
		set       []byte = []byte("qan config")
		running   []byte = []byte("qan config")
	)

	// Create agentHandler
	agentHandler := agent.NewMySQLHandler(db.DBManager, instance.NewMySQLHandler(db.DBManager))

	// Update config (actual test)
	err := agentHandler.SetConfig(agentId, service, mysqlUUID, set, running)
	t.Assert(err, IsNil)

	var (
		gotAgentId uint
		gotOtherId uint
		gotService string
		gotSet     string
		gotRunning string
	)
	err = s.testDb.DB().QueryRow(
		"SELECT agent_instance_id, other_instance_id, service, in_file, running FROM agent_configs WHERE agent_instance_id=? AND other_instance_id=? AND service=?",
		agentId,
		otherId,
		service,
	).Scan(
		&gotAgentId,
		&gotOtherId,
		&gotService,
		&gotSet,
		&gotRunning,
	)
	t.Assert(err, IsNil)
	t.Check(gotAgentId, Equals, agentId)
	t.Check(gotOtherId, Equals, otherId)
	t.Check(gotService, Equals, service)
	t.Check(gotSet, Equals, string(set))
	t.Check(gotRunning, Equals, string(running))

	// Set a config for the same MySQL instance but another tool.
	service = "mm-mysql"
	set = []byte("mm config")
	running = []byte("mm config")
	err = agentHandler.SetConfig(agentId, service, mysqlUUID, set, running)
	t.Assert(err, IsNil)
	err = s.testDb.DB().QueryRow(
		"SELECT agent_instance_id, other_instance_id, service, in_file, running FROM agent_configs WHERE agent_instance_id=? AND other_instance_id=? AND service=?",
		agentId,
		otherId,
		service,
	).Scan(
		&gotAgentId,
		&gotOtherId,
		&gotService,
		&gotSet,
		&gotRunning,
	)
	t.Assert(err, IsNil)
	t.Check(gotAgentId, Equals, agentId)
	t.Check(gotOtherId, Equals, otherId)
	t.Check(gotService, Equals, service)
	t.Check(gotSet, Equals, string(set))
	t.Check(gotRunning, Equals, string(running))

	// There should be two configs for the MySQL instance.
	var count int
	err = s.testDb.DB().QueryRow("SELECT COUNT(*) FROM agent_configs WHERE agent_instance_id = ?", agentId).Scan(&count)
	t.Assert(count, Equals, 2)
}

func (s *MySQLTestSuite) TestCreateWithUUID(t *C) {
	agentHandler := agent.NewMySQLHandler(db.DBManager, instance.NewMySQLHandler(db.DBManager))

	agent := proto.Agent{
		ParentUUID: "101",
		UUID:       "222",
		Hostname:   "new-host1",
		Version:    "1.0.0",
	}
	gotUUID, err := agentHandler.Create(agent)
	t.Assert(err, IsNil)
	t.Check(gotUUID, Equals, "222")

	gotAgentId, gotAgent, err := agentHandler.Get(gotUUID)
	t.Assert(err, IsNil)
	t.Check(gotAgentId, Equals, uint(4)) // might fail due to auto-inc
	t.Check(gotAgent.UUID, Equals, "222")
	t.Check(gotAgent.ParentUUID, Equals, "101")
	t.Check(gotAgent.Hostname, Equals, "new-host1")
	//t.Check(gotAgent.Version, Equals, "1.0.0")
}

func (s *MySQLTestSuite) TestCreateWithoutUUID(t *C) {
	agentHandler := agent.NewMySQLHandler(db.DBManager, instance.NewMySQLHandler(db.DBManager))

	agent := proto.Agent{
		ParentUUID: "101",
		Hostname:   "foo",
		Version:    "1.0.0",
	}
	gotUUID, err := agentHandler.Create(agent)
	t.Assert(err, IsNil)
	t.Check(gotUUID, HasLen, 32)

	gotAgentId, gotAgent, err := agentHandler.Get(gotUUID)
	t.Assert(err, IsNil)
	t.Check(gotAgentId, Not(Equals), uint(0))
	t.Check(gotAgent.UUID, Equals, gotUUID)
	t.Check(gotAgent.ParentUUID, Equals, "101")
	t.Check(gotAgent.Hostname, Equals, "foo")
	//t.Check(gotAgent.Version, Equals, "1.0.0")
}

func (s *MySQLTestSuite) TestUpdate(t *C) {
	agentHandler := agent.NewMySQLHandler(db.DBManager, instance.NewMySQLHandler(db.DBManager))

	agent := proto.Agent{
		UUID:     "212",
		Hostname: "new-host", // original is db01-agent
		Version:  "1.0.5",
	}
	err := agentHandler.Update(3, agent)
	t.Assert(err, IsNil)

	_, gotAgent, err := agentHandler.Get("212")
	t.Assert(err, IsNil)
	t.Check(gotAgent.UUID, Equals, "212") // not changed
	t.Check(gotAgent.Hostname, Equals, "new-host")
	t.Check(gotAgent.Version, Equals, "1.0.5")
}

func (s *MySQLTestSuite) TestUpdateConfigs(t *C) {
	s.testDb.LoadDataInfiles(config.TestDir + "/configs/002")

	agentHandler := agent.NewMySQLHandler(db.DBManager, instance.NewMySQLHandler(db.DBManager))

	// Create new configs to update loaded test configs which has logs for
	// log, data, and mm-server.  We'll update data and mm-server and provide
	// a new config.
	newConfigs := []proto.AgentConfig{
		{
			Service: "data",
		},
		{
			Service: "mm",
			UUID:    "313",
		},
		{ // new
			Service: "sysconfig",
			UUID:    "313",
		},
	}

	var agentId uint = 2
	err := agentHandler.UpdateConfigs(agentId, newConfigs, true)
	t.Assert(err, IsNil)

	gotConfigs, err := agentHandler.GetConfigs(agentId)
	t.Assert(err, IsNil)
	t.Assert(gotConfigs, HasLen, 3)

	// The return value is an unsorted array, so we need to find each specific config.
	// We'll just make a map of array indexes.
	c := make(map[string]int)
	for i, config := range gotConfigs {
		c[config.Service] = i
	}

	// 2014-01-01 00:00:00 comes from test data.
	t0, _ := time.Parse("2006-01-02 15:04:05", "2014-01-01 00:00:00")

	// Check new config first: sysconfig
	t.Check(gotConfigs[c["sysconfig"]].UUID, Equals, newConfigs[2].UUID)

	// Existing mm config, updated.
	t.Check(gotConfigs[c["mm"]].UUID, Equals, newConfigs[1].UUID)
	t.Check(gotConfigs[c["mm"]].Updated, Not(Equals), t0)

	// Existing data config, updated.
	t.Check(gotConfigs[c["data"]].UUID, Equals, newConfigs[0].UUID)
	t.Check(gotConfigs[c["data"]].Updated, Not(Equals), t0)

	// We didn't provide a log config so it was deleted because last arg was true.
}

func (s *MySQLTestSuite) TestGetAgents(t *C) {
	agentHandler := agent.NewMySQLHandler(db.DBManager, instance.NewMySQLHandler(db.DBManager))

	got, err := agentHandler.GetAll()
	t.Assert(err, IsNil)
	t.Assert(got, Not(HasLen), 0)
	t.Check(got, HasLen, 1)
	t.Check(got[0].UUID, Equals, "212")
	t.Check(got[0].ParentUUID, Equals, "101")
	t.Check(got[0].Hostname, Equals, "db01-agent")
	t.Check(got[0].Deleted, Equals, time.Time{})
}
