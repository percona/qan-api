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

	"github.com/percona/qan-api/app/agent"
	"github.com/percona/qan-api/test/mock"
	"github.com/percona/pmm/proto"
	. "gopkg.in/check.v1"
)

type CommTestSuite struct {
	sendChan chan interface{}
	recvChan chan interface{}
	mockMx   *mock.Multiplexer
}

var _ = Suite(&CommTestSuite{})

func (s *CommTestSuite) SetUpSuite(t *C) {
	s.sendChan = make(chan interface{}, 3)
	s.recvChan = make(chan interface{}, 3)
	s.mockMx = mock.NewMultiplexer(s.sendChan, s.recvChan)
}

// --------------------------------------------------------------------------

func (s *CommTestSuite) TestLocal(t *C) {
	agentId := uint(1)
	comm := agent.NewLocalAgent(agentId, s.mockMx)

	// When we start the comm it's going to send Version and GetAllConfigs
	// cmds, the standard pre-init sequence for agents before the comm returns
	// to the controller which makes the agent available in the system. So
	// queue proto.Reply on the send chan for the mock mx to send back to
	// comm.Start().Send().mx.Send().
	v := proto.Version{
		Running:   "0.0.9",
		Revision:  "abc",
		Installed: "0.0.9",
	}
	vdata, _ := json.Marshal(v)
	s.sendChan <- &proto.Reply{Cmd: "Version", Data: vdata}
	s.sendChan <- &proto.Reply{Cmd: "GetAllConfig"}

	err := comm.Start()
	t.Check(err, IsNil)

	// Check that comm.Start() did in fact send Version and GetAllConfigs.
	n := len(s.recvChan)
	t.Assert(n, Not(Equals), 0)
	t.Check(n, Equals, 2)
	gotReply := make([]*proto.Cmd, n)
	for i := 0; i < n; i++ {
		v := <-s.recvChan
		gotReply[i] = v.(*proto.Cmd)
	}
	t.Check(gotReply[0].Cmd, Equals, "Version")
	t.Check(gotReply[1].Cmd, Equals, "GetAllConfigs")

	// While we're already started, check IsAlive() too.

	// Queue the reply to the liveliness check, which is a Version cmd.
	s.sendChan <- &proto.Reply{Cmd: "Version"}

	alive := comm.IsAlive()
	t.Check(alive, Equals, true)

	n = len(s.recvChan)
	t.Assert(n, Not(Equals), 0)
	t.Check(n, Equals, 1)
	gotReply = make([]*proto.Cmd, n)
	for i := 0; i < n; i++ {
		v := <-s.recvChan
		gotReply[i] = v.(*proto.Cmd)
	}
	t.Check(gotReply[0].Cmd, Equals, "Version")
}

func (s *CommTestSuite) TestStopOldAgent(t *C) {
	t.Skip("There are no old agents")

	agentId := uint(1)
	comm := agent.NewLocalAgent(agentId, s.mockMx)

	v := proto.Version{
		Running:   "1.0.10", // only >= 1.0.11 supported
		Revision:  "abc",
		Installed: "1.0.10",
	}
	vdata, _ := json.Marshal(v)
	s.sendChan <- &proto.Reply{Cmd: "Version", Data: vdata}
	s.sendChan <- &proto.Reply{Cmd: "Stop"} // must reply to Stop cmd else we'll deadlock the chans

	// Comm fails to start because agent is too old. Error is like: "old agent (%!s(<nil>))".
	// The "<nil>" is because there's no error sending the Stop cmd.
	err := comm.Start()
	t.Check(err, NotNil)

	// Check that comm.Start() did in fact send Version and GetAllConfigs.
	n := len(s.recvChan)
	t.Assert(n, Not(Equals), 0)
	t.Check(n, Equals, 2)
	gotReply := make([]*proto.Cmd, n)
	for i := 0; i < n; i++ {
		v := <-s.recvChan
		gotReply[i] = v.(*proto.Cmd)
	}
	t.Check(gotReply[0].Cmd, Equals, "Version")
	t.Check(gotReply[1].Cmd, Equals, "Stop") // here it is
}
