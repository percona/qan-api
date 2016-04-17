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

package mock

import (
	"time"

	"github.com/percona/qan-api/app/shared"
	"github.com/percona/pmm/proto"
)

type AgentCommunicator struct {
	replies map[string]*proto.Reply
	// --
	doneChan chan bool
	Alive    bool
	Sent     []*proto.Cmd
}

func NewAgentCommunicator(replies map[string]*proto.Reply) *AgentCommunicator {
	a := &AgentCommunicator{
		replies: replies,
		// --
		doneChan: make(chan bool),
		Alive:    true,
		Sent:     []*proto.Cmd{},
	}
	return a
}

func (a *AgentCommunicator) Start() error {
	return nil
}

func (a *AgentCommunicator) Stop() {
}

func (a *AgentCommunicator) Done() chan bool {
	return a.doneChan
}

func (a *AgentCommunicator) IsAlive() bool {
	return a.Alive
}

func (a *AgentCommunicator) Send(cmd *proto.Cmd) (*proto.Reply, error) {
	a.Sent = append(a.Sent, cmd)
	return a.replies[cmd.AgentUUID], nil
}

// --------------------------------------------------------------------------

type AgentDirectory struct {
	comms map[uint]shared.AgentCommunicator
}

func NewAgentDirectory(comms map[uint]shared.AgentCommunicator) *AgentDirectory {
	d := &AgentDirectory{
		comms: comms,
	}
	return d
}

func (d *AgentDirectory) Add(agentId uint, comm shared.AgentCommunicator) error {
	return nil
}

func (d *AgentDirectory) Remove(agentId uint) {
}

func (d *AgentDirectory) Get(agentId uint) shared.AgentCommunicator {
	return d.comms[agentId]
}

func (d *AgentDirectory) Find(agentId uint) (shared.AgentCommunicator, error) {
	return d.comms[agentId], nil
}

func (d *AgentDirectory) Refresh(timeLimit time.Duration) {
}
