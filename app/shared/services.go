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

package shared

import (
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/service/query"
	"github.com/percona/qan-api/stats"
)

var AgentDirectory AgentFinder
var InternalStats stats.Stats = stats.NewStats(&statsd.NoopClient{}, "", "", "", "0")
var RouteStats stats.Stats = stats.NewStats(&statsd.NoopClient{}, "", "", "", "0")
var QueryAbstracter *query.Mini
var TableParser *query.Mini

// An AgentCommunicator sends proto.Cmd to a agent, handling concurrency and
// hiding whether the agent is local or remote.
type AgentCommunicator interface {
	Start() error
	Stop()
	Done() chan bool
	IsAlive() bool
	Send(cmd *proto.Cmd) (*proto.Reply, error)
}

// An AgentFinder provides local clients the AgentCommunicator for an agent.
type AgentFinder interface {
	Add(agentId uint, comm AgentCommunicator) error
	Remove(agentId uint)
	Get(agentId uint) AgentCommunicator
	Refresh(timeLimit time.Duration)
}
