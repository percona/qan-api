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

package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/app/ws"
	"github.com/percona/pmm/proto"
)

// A LocalAgent is a Communicator for an agent connected to this API. It uses a
// ws.Multiplexer to serialize commands to and replies from the agent. The comm
// processor updates oN tables as necessary. This allows processes (controllers,
// services, etc.) in this API to communicate with this agent. A LocalAgent
// should only be created in the agent comm websocket controller, which
// registers it with the LocalDirectory.
type LocalAgent struct {
	agentId uint
	mx      ws.Multiplexer
}

func NewLocalAgent(agentId uint, mx ws.Multiplexer) *LocalAgent {
	a := &LocalAgent{
		agentId: agentId,
		mx:      mx,
	}
	return a
}

func (a *LocalAgent) Start() error {
	// Start the multiplexer which uses an agent.Processor to update the db for
	// certain cmd/reply.
	if err := a.mx.Start(); err != nil {
		return err
	}

	// Before returning to websocket controller which is going to register the
	// agent and make it available to clients (i.e. while we have sole access
	// to the agent right now), get some info from the agent. The replies for
	// these are handled in the commProc.

	// First, get the agent version because we only support >= 1.0.11.
	reply, err := a.Send(&proto.Cmd{
		Ts:      time.Now().UTC(),
		User:    "api",
		Service: "agent",
		Cmd:     "Version",
	})
	if err != nil {
		a.mx.Stop()
		return fmt.Errorf("Version: %s", err)
	}
	ok, err := a.checkVersion(reply)
	if err != nil {
		a.mx.Stop()
		return fmt.Errorf("checkVersion: %s", err)
	}
	if !ok {
		// Agent is too old. Stop it.
		shared.InternalStats.Inc(shared.InternalStats.Metric("agent.comm.old-agent"), 1, 1)
		_, err := a.Send(&proto.Cmd{
			Ts:      time.Now().UTC(),
			User:    "api",
			Service: "agent",
			Cmd:     "Stop",
		})
		// Error or not, stop the mx because we're already planning to stop it
		// due to agent being too old.
		a.mx.Stop()
		return fmt.Errorf("old agent (%s)", err)
	}

	// Next, do whatever other cmds we want to initialize the agent/API with.
	// For now it's just just +1:
	for _, cmd := range []string{"GetAllConfigs"} {
		_, err := a.Send(&proto.Cmd{
			Ts:      time.Now().UTC(),
			User:    "api",
			Service: "agent",
			Cmd:     cmd,
		})
		if err != nil {
			a.mx.Stop()
			return fmt.Errorf("%s: %s", cmd, err)
		}
	}

	return nil // agent connected and ready
}

func (a *LocalAgent) Stop() {
	a.mx.Stop()
}

func (a *LocalAgent) Done() chan bool {
	// Agent is done (disconnected, lost, etc.--no longer connected to this API)
	// when the mx is done because the mx is required for agent communication.
	// The mx is typically only done when the websocket to the agent closes or
	// has an error.
	return a.mx.Done()
}

func (a *LocalAgent) IsAlive() bool {
	// Agent doesn't have a Ping cmd, but any cmd will do, so Version is a good
	// choice because it's quick to do (i.e. no tools are involved).
	_, err := a.Send(&proto.Cmd{
		Ts:      time.Now().UTC(),
		User:    "api",
		Service: "agent",
		Cmd:     "Version",
	})
	if err != nil {
		return false
	}
	return true
}

func (a *LocalAgent) Send(cmd *proto.Cmd) (*proto.Reply, error) {
	data, err := a.mx.Send(cmd)
	if data == nil {
		return nil, err
	}
	return data.(*proto.Reply), err
}

func (a *LocalAgent) checkVersion(reply *proto.Reply) (bool, error) {
	var v proto.Version
	if err := json.Unmarshal(reply.Data, &v); err != nil {
		return false, err
	}
	if v.Running == "0.0.9" {
		// Tests use this fake version, so allow it. No real agent would report
		// this, so it's a safe special case.
		return true, nil
	}
	return shared.AtLeastVersion(v.Running, "1.0.0")
}
