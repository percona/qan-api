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
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/test/mock"
	. "gopkg.in/check.v1"
)

type DirTestSuite struct {
	hostname string
	replies  map[string]*proto.Reply
	mockComm *mock.AgentCommunicator
	sendChan chan *proto.Reply
	recvChan chan *proto.Cmd
	dir      *agent.LocalDirectory
}

var _ = Suite(&DirTestSuite{})

func (s *DirTestSuite) SetUpTest(t *C) {
	s.replies = map[string]*proto.Reply{}
	s.mockComm = mock.NewAgentCommunicator(s.replies)
	s.sendChan = make(chan *proto.Reply, 3)
	s.recvChan = make(chan *proto.Cmd, 3)
	s.dir = agent.NewLocalDirectory()
}

func (s *DirTestSuite) TearDownTest(t *C) {
}

func (s *DirTestSuite) TearDownSuite(t *C) {
}

// --------------------------------------------------------------------------

func (s *DirTestSuite) TestAddGetRemove(t *C) {
	id := uint(1)

	// Add local agent to directory.
	err := s.dir.Add(id, s.mockComm)
	t.Check(err, IsNil)

	// We should be able to get the same agent comm back.
	gotComm := s.dir.Get(id)
	t.Check(gotComm, Equals, s.mockComm)

	// And we should be able to remove the comm, from local and global.
	s.dir.Remove(id)
	gotComm = s.dir.Get(id)
	t.Check(gotComm, IsNil) // local gone
}

func (s *DirTestSuite) TestAddDuplicate(t *C) {
	// Of course, duplicates should not be allowed.
	id := uint(1)

	// Add local agent to directory.
	err := s.dir.Add(id, s.mockComm)
	t.Check(err, IsNil)

	// Add it again.
	err = s.dir.Add(id, s.mockComm)
	t.Check(err, Equals, shared.ErrDuplicateAgent)
}

func (s *DirTestSuite) TestRefresh(t *C) {
	// Add 3 agents to directory.
	comms := []*mock.AgentCommunicator{}
	for id := uint(1); id <= 3; id++ {
		comms = append(comms, mock.NewAgentCommunicator(s.replies))
		err := s.dir.Add(id, comms[id-1])
		t.Assert(err, IsNil)
	}

	// Let's simulate 1 is dead, 2 is ok, and 3 is ok but it's global cache
	// entry was lost somehow, so it needs to be restored.
	agent1 := s.dir.Get(uint(1))
	t.Assert(agent1, NotNil)
	t.Assert(agent1, Equals, comms[0])
	comms[0].Alive = false // shared.AgentCommunicator interface doesn't have Alive

	agent2 := s.dir.Get(uint(2))
	t.Assert(agent2, NotNil)

	agent3 := s.dir.Get(uint(3))
	t.Assert(agent3, NotNil)

	s.dir.Refresh(2 * time.Second)

	// Agent 2 should still be ok, and the same comm.
	agent2Check := s.dir.Get(uint(2))
	t.Check(agent2Check, NotNil)
	t.Check(agent2Check, Equals, agent2)

	// Agent 1 should be gone.
	agent1Check := s.dir.Get(uint(1))
	t.Check(agent1Check, IsNil)
}
