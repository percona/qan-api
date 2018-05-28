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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/percona/pmm/proto"
	pc "github.com/percona/pmm/proto/config"
	"github.com/percona/qan-api/app/agent"
	mock "github.com/percona/qan-api/test/mock/closure"
	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
)

type ProcTestSuite struct {
	agentId      uint
	agentUUID    string
	mysqlUUID    string
	agentHandler *mock.AgentHandlerMock
	proc         *agent.Processor
	// --
	gotAgentId uint
	gotService string
	gotUUID    string
	gotSet     string
	gotRunning string
}

var _ = Suite(&ProcTestSuite{})

func (s *ProcTestSuite) SetUpSuite(t *C) {
	s.agentId = 1
	s.agentUUID = "212"
	s.mysqlUUID = "313"
}

func (s *ProcTestSuite) SetUpTest(t *C) {
	// Make a mock db.AgentHandler so we don't have to use a real db.
	// The agent.Processor doesn't know or care; that's why it's an interface.
	s.gotAgentId = uint(0)
	s.gotService = "not set" // to ensure "" isn't by accident
	s.gotUUID = "not set"    // to ensure "" isn't by accident
	s.gotSet = "not set"     // to ensure "" isn't by accident
	s.gotRunning = "not set" // to ensure "" isn't by accident
	s.agentHandler = &mock.AgentHandlerMock{
		SetConfigMock: func(agentId uint, service, otherUUID string, set, running []byte) error {
			s.gotAgentId = agentId
			s.gotService = service
			s.gotUUID = otherUUID
			s.gotSet = string(set)
			s.gotRunning = string(running)
			return nil
		},
	}
	s.proc = agent.NewProcessor(s.agentId, s.agentHandler)
}

// --------------------------------------------------------------------------

func (s *ProcTestSuite) TestStartServiceQAN(c *C) {

	// Test how StartService to QAN is processed. It should cause agent.Processor
	// to call its db.AgentHandler.SetConfig() with the config data in the
	// cmd. So first we make a full SetConfig cmd:
	exampleQueries := false
	config := pc.QAN{
		UUID: s.mysqlUUID,
		Start: []string{
			"SET GLOBAL slow_query_log=OFF",
			"SET GLOBAL long_query_time=0.123",
			"SET GLOBAL slow_query_log=ON",
		},
		Stop: []string{
			"SET GLOBAL slow_query_log=OFF",
			"SET GLOBAL long_query_time=10",
		},
		Interval:       300,        // 5 min
		MaxSlowLogSize: 1073741824, // 1 GiB
		ExampleQueries: &exampleQueries,
	}
	data, err := json.Marshal(config)
	c.Assert(err, IsNil)

	cmd := &proto.Cmd{
		Service:   "qan",
		Cmd:       "StartTool",
		Data:      data,
		Ts:        time.Now(),
		AgentUUID: s.agentUUID,
	}

	// In real world, some process (like a controller) would call
	// agent.LocalAgent.Send(cmd) to send the cmd and wait for a reply.
	// This calls its internal shared.Multiplexer.Send() (a ws.Multiplexer
	// instance) which handles concurrency in from clients and serialization
	// out to the agent, and vice versa. The multiplexer uses the comm.Processor
	// (an agent.Processor in this case) to do pre- and post-processing of the
	// data. So before sending to agent the mx calls:
	id1, bytes, err := s.proc.BeforeSend(cmd)
	c.Check(id1, Not(Equals), "")
	c.Check(bytes, Not(HasLen), 0)
	c.Check(err, IsNil)

	// At this point, the mx would remember id1 and send bytes to the agent.
	// The agent receives the bytes (as a proto.Cmd) and sends back bytes
	// (a proto.Reply). The mx receives these bytes and passes them to:
	reply := cmd.Reply(nil)
	bytes, err = json.Marshal(reply)
	c.Assert(err, IsNil)

	id2, v, err := s.proc.AfterRecv(bytes)
	c.Check(id1, Equals, id2)
	c.Assert(v, NotNil)
	c.Check(err, IsNil)

	// At this point, the mx would route v back to the id1 caller. But we've
	// bypassed all that for this unit test.

	// Sanity check the reply, but what we're really testing is...
	gotReply := v.(*proto.Reply)
	c.Check(gotReply.Id, Equals, id1)
	c.Check(gotReply.Error, Equals, "")

	// The agent.Processor should have called AgentHandler.UpdateConfig(),
	// so our mock instances of that should have received the proper values from
	// the original cmd.
	c.Check(s.gotAgentId, Equals, uint(1))
	c.Check(s.gotUUID, Equals, s.mysqlUUID)
	c.Check(s.gotService, Equals, cmd.Service)
	c.Check(s.gotSet, Equals, string(data))
	c.Check(s.gotRunning, Equals, "")
}

func (s *ProcTestSuite) TestStopServiceQAN(c *C) {
	// StopService is nearly identical to StartService except a different agent
	// handler is called:
	s.agentHandler.SetConfigMock = nil
	s.agentHandler.RemoveConfigMock = func(agentId uint, service, otherUUID string) error {
		s.gotAgentId = agentId
		s.gotService = service
		s.gotUUID = otherUUID
		return nil
	}

	cmd := &proto.Cmd{
		Service:   "qan",
		Cmd:       "StopTool",
		Ts:        time.Now(),
		Data:      []byte(s.mysqlUUID),
		AgentUUID: s.agentUUID,
	}
	_, bytes, err := s.proc.BeforeSend(cmd)
	c.Check(bytes, Not(HasLen), 0)
	c.Check(err, IsNil)

	reply := cmd.Reply(nil)
	bytes, err = json.Marshal(reply)
	c.Assert(err, IsNil)

	_, v, err := s.proc.AfterRecv(bytes)
	c.Assert(v, NotNil)
	c.Check(err, IsNil)

	c.Check(s.gotAgentId, Equals, s.agentId)
	c.Check(s.gotService, Equals, cmd.Service)
	c.Check(s.gotUUID, Equals, s.mysqlUUID)
}

func (s *ProcTestSuite) TestStartServiceAgent(c *C) {
	// Sending a StartService cmd to the agent has a different meaning because
	// the agent can't start itself. It means to make the agent start another
	// internal service specified by the included proto.ServiceData. E.g. if the
	// log service was off somehow, this is how we'd start it:
	sd := &proto.ServiceData{
		Name:   "log",
		Config: []byte("..."),
	}
	data, err := json.Marshal(sd)
	c.Assert(err, IsNil)
	cmd := &proto.Cmd{
		Service:   "agent",
		Cmd:       "StartService",
		Data:      data,
		Ts:        time.Now(),
		AgentUUID: s.agentUUID,
	}

	id1, bytes, err := s.proc.BeforeSend(cmd)
	c.Check(id1, Not(Equals), "")
	c.Check(bytes, Not(HasLen), 0)
	c.Check(err, IsNil)

	reply := cmd.Reply(nil)
	bytes, err = json.Marshal(reply)
	c.Assert(err, IsNil)

	id2, v, err := s.proc.AfterRecv(bytes)
	c.Check(id1, Equals, id2)
	c.Assert(v, NotNil)
	c.Check(err, IsNil)

	c.Check(s.gotAgentId, Equals, uint(1))
	c.Check(s.gotService, Equals, sd.Name) // yes it's ServiceData.Name not cmd.Service
	c.Check(s.gotUUID, Equals, "")
	c.Check(s.gotSet, Equals, string(sd.Config)) // yes it's ServiceData.Config
	c.Check(s.gotRunning, Equals, "")
}

func (s *ProcTestSuite) TestAfterRecvError(c *C) {
	// When an error happens in SendRecv, we should get it back.
	/*
		// Override the default mock agent handler from SetUpTest:
		expectErr := fmt.Errorf("internal error")
		s.agentHandler = &mock.AgentHandlerMock{
			UpdateConfigMock: func(agentId uint, service string, instanceType string, instanceId uint, config string, running uint) error {
				return expectErr
			},
		}
		s.proc = agent.NewProcessor(s.agentId, s.agentHandler)

		// Make and send a cmd. It doesn't have to be correct/complete.
		config := &qan.Config{
			UUID:           "313",
			Interval:       300,
			MaxSlowLogSize: 1073741824,
			WorkerRunTime:  600,
		}

		data, err := json.Marshal(config)
		c.Assert(err, IsNil)
		cmd := &proto.Cmd{
			Service:   "qan",
			Cmd:       "StartService",
			Data:      data,
			Ts:        time.Now(),
			AgentUUID: s.agentUUID,
		}
		id1, bytes, err := s.proc.BeforeSend(cmd)
		c.Check(id1, Not(Equals), "")
		c.Check(bytes, Not(HasLen), 0)
		c.Check(err, IsNil) // no error yet

		reply := cmd.Reply(nil)
		bytes, err = json.Marshal(reply)
		c.Assert(err, IsNil)

		id2, v, gotErr := s.proc.AfterRecv(bytes)
		c.Check(id1, Equals, id2)
		c.Check(v, NotNil)
		c.Check(gotErr, Equals, expectErr) // did we get the error?

		// QAN report emails should not have been enabled because of the error.
		c.Check(s.gotEmailReports, Equals, "")
	*/
}

func (s *ProcTestSuite) TestPong(c *C) {
	// Agents send Pong replies to keep the cmd websocket alive. The API doesn't
	// send Ping cmd; the agent pushes Pong so it's an exception that's handled
	// simply by dropping it.
	reply := &proto.Reply{Cmd: "Pong"}
	bytes, err := json.Marshal(reply)
	c.Assert(err, IsNil)

	// Don't call BeforeSend() because Pong isn't sent, only received.

	id, v, err := s.proc.AfterRecv(bytes)
	c.Check(id, Equals, "")
	c.Check(v, IsNil)
	c.Check(err, IsNil)
}

func (s *ProcTestSuite) TestReplyError(c *C) {
	// When there's a proto.Reply.Error, the agent failed to handle the cmd,
	// and so far that means we don't do any processing, we just ignore errors
	// and pass them back up to the caller.

	// Make and send any cmd, doeesn't have to be correct/complete. -- Well not
	// any cmd: let's do StartService to ensure that no processing is done,
	// i.e. the internal func to handle this cmd/reply is not called.
	config := pc.QAN{
		UUID:           "313",
		Interval:       300,
		MaxSlowLogSize: 1073741824,
	}
	data, err := json.Marshal(config)
	c.Assert(err, IsNil)
	cmd := &proto.Cmd{
		Service:   "qan",
		Cmd:       "StartService",
		Data:      data,
		Ts:        time.Now(),
		AgentUUID: s.agentUUID,
	}

	_, bytes, err := s.proc.BeforeSend(cmd)
	c.Check(bytes, Not(HasLen), 0)
	c.Check(err, IsNil)

	// Makek a reply with an error.
	expectErr := fmt.Errorf("agent error")
	reply := cmd.Reply(nil, expectErr)
	bytes, err = json.Marshal(reply)
	c.Assert(err, IsNil)

	// Simulate receiving that reply.
	_, v, err := s.proc.AfterRecv(bytes)
	c.Assert(v, NotNil)
	c.Check(err, IsNil)

	// Make sure we got the same reply we sent.
	gotReply := v.(*proto.Reply)
	c.Check(gotReply.Error, Equals, expectErr.Error())

	// The agent handler mock should not have been called. We only need to
	// check one var because they're all set at the same time if called.
	c.Check(s.gotService, Equals, "not set")
	c.Check(s.gotUUID, Equals, "not set")
}

func (s *ProcTestSuite) TestSetConfig(c *C) {
	c.Skip("This is somehow broken; In proc.go I still see SetConfig cmd, but config from cmd.Data is not saved; So this should be fixed or SetConfig command should be removed")

	config := pc.Log{
		Level: "debug",
	}
	data, err := json.Marshal(config)
	c.Assert(err, IsNil)
	cmd := &proto.Cmd{
		Service:   "log",
		Cmd:       "SetConfig",
		Data:      data,
		Ts:        time.Now(),
		AgentUUID: s.agentUUID,
	}

	_, bytes, err := s.proc.BeforeSend(cmd)
	c.Check(bytes, Not(HasLen), 0)
	c.Check(err, IsNil)

	reply := cmd.Reply(nil)
	bytes, err = json.Marshal(reply)
	c.Assert(err, IsNil)

	_, v, err := s.proc.AfterRecv(bytes)
	c.Assert(v, NotNil)
	c.Check(err, IsNil)

	c.Check(s.gotAgentId, Equals, uint(1))
	c.Check(s.gotService, Equals, cmd.Service)
	c.Check(s.gotUUID, Equals, "")
	c.Check(s.gotSet, Equals, string(data))
	c.Check(s.gotRunning, Equals, string(data))
}

func (s *ProcTestSuite) TestVersion(c *C) {
	var gotVersion proto.Version
	s.agentHandler.UpdateVersionMock = func(agentId uint, version proto.Version) error {
		s.gotAgentId = agentId
		gotVersion = version
		return nil
	}

	cmd := &proto.Cmd{
		Service:   "agent",
		Cmd:       "Version",
		Ts:        time.Now(),
		AgentUUID: s.agentUUID,
	}

	_, bytes, err := s.proc.BeforeSend(cmd)
	c.Check(bytes, Not(HasLen), 0)
	c.Check(err, IsNil)

	expectVersion := proto.Version{
		Installed: "1.0.10",
		Running:   "1.0.11",
	}
	reply := cmd.Reply(expectVersion)
	bytes, err = json.Marshal(reply)
	c.Assert(err, IsNil)

	_, v, err := s.proc.AfterRecv(bytes)
	c.Assert(v, NotNil)
	c.Check(err, IsNil)

	c.Check(s.gotAgentId, Equals, uint(1))
	c.Check(gotVersion, DeepEquals, expectVersion)
}

func (s *ProcTestSuite) TestGetAllConfigs(c *C) {
	gotConfigs := []proto.AgentConfig{}
	gotReset := false
	s.agentHandler.UpdateConfigsMock = func(agentId uint, configs []proto.AgentConfig, reset bool) error {
		s.gotAgentId = agentId
		gotConfigs = configs
		gotReset = reset
		return nil
	}

	cmd := &proto.Cmd{
		Service:   "agent",
		Cmd:       "GetAllConfigs",
		Ts:        time.Now(),
		AgentUUID: s.agentUUID,
	}

	_, bytes, err := s.proc.BeforeSend(cmd)
	c.Check(bytes, Not(HasLen), 0)
	c.Check(err, IsNil)

	// We don't need to send back real configs. We're just checking that
	// the mock agent handler ^ is called with what we send.
	expectConfigs := []proto.AgentConfig{
		{
			Service: "data",
			Updated: time.Now().UTC(), // todo: why does this need to be set?
		},
		{
			Service: "log",
			Updated: time.Now().UTC(), // todo: why does this need to be set?
		},
	}
	reply := cmd.Reply(expectConfigs)
	bytes, err = json.Marshal(reply)
	c.Assert(err, IsNil)

	_, v, err := s.proc.AfterRecv(bytes)
	c.Assert(v, NotNil)
	c.Check(err, IsNil)

	c.Check(s.gotAgentId, Equals, uint(1))
	assert.Equal(c, expectConfigs, gotConfigs)

	c.Check(gotReset, Equals, true)
}

func (s *ProcTestSuite) TestUnknownCmd(c *C) {
	// The processor saves the ID of all cmd sent (ID given by BeforeSend),
	// then it matches replies to know where to send the reply back to. In case
	// a reply is received with no matching cmd, it should be dropped.
	reply := &proto.Reply{Cmd: "StartService", Id: "unknown"}
	bytes, err := json.Marshal(reply)
	c.Assert(err, IsNil)

	// Id is not returned which causes the multiplexer to drop the data,
	// id=""=/dev/null
	id, v, err := s.proc.AfterRecv(bytes)
	c.Check(id, Equals, "")
	c.Check(v, IsNil)
	c.Check(strings.HasPrefix(err.Error(), "unknown reply"), Equals, true)

	// The agent handler mock should not have been called.
	c.Check(s.gotService, Equals, "not set")
}
