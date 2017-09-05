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

	"github.com/go-sql-driver/mysql"
	"github.com/nu7hatch/gouuid"
	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/models"
)

// A Processor updates oN tables for certain commands and replies. It is
// per-agent and NOT concurrent[1], so cmds is not guarded with a mutex,
// because cmds to agent must be serialized (including cmd=Status; agent
// parallelizes these internally).
//
// [1] Be sure ws.NewConcurrentMultiplexer(..., concurrency=0) in NewLocalAgent()!
//     Else the multiplexer will call AfterRecv() concurrently.
type Processor struct {
	agentId        uint
	version        proto.Version
	agentConfigMgr models.AgentConfigManager
	cmds           map[string]*proto.Cmd
}

func NewProcessor(agentId uint, agentConfigMgr models.AgentConfigManager) *Processor {
	return &Processor{
		agentId:        agentId,
		agentConfigMgr: agentConfigMgr,
		// --
		cmds: make(map[string]*proto.Cmd),
	}
}

func (p *Processor) BeforeSend(data interface{}) (string, []byte, error) {
	// Return the id of this cmd so the multiplexer knows to return its reply
	// to us and no some other
	cmd := data.(*proto.Cmd)
	if cmd.Id == "" {
		// This cmd is from a truly local caller, i.e. some controller calling
		// AgentCommunicator.Send(), so we--the sender--set the id. Else,
		// if it's already set, then the cmd originated from another API
		// and the local caller is api.Processor.handleRequest() being used
		// by an api.Link acting on behalf of the remote API/caller. In this
		// latter case, although the api.Link is local, the cmd originated
		// in a remote API.
		id, _ := uuid.NewV4()
		cmd.Id = id.String()
	}
	bytes, err := json.Marshal(cmd)
	if err != nil {
		return "", nil, err
	}
	p.cmds[cmd.Id] = cmd
	return cmd.Id, bytes, nil
}

func (p *Processor) AfterRecv(bytes []byte) (string, interface{}, error) {
	// NOTE: 3rd return value (error) is for internal problems, not reply.Error.
	//       Internal problems are like read-only db, fipar, etc. If
	//       reply.Error != "", then cmd failed and we ignore it here.

	// Decode reply from agent.
	reply := &proto.Reply{}
	if err := json.Unmarshal(bytes, reply); err != nil {
		return "", nil, err
	}

	// Keep-alive from agent to API to prevent half-open TCP connection.
	// This is not a real reply; there was no Ping cmd from API or user.
	// https://jira.percona.com/browse/PCT-765
	// Returning id="" err=nil cauess the multiplexer to ignore this.
	if reply.Cmd == "Pong" {
		return "", nil, nil
	}

	// Look up the cmd for this reply, then remove it because the cmd is done.
	cmd := p.cmds[reply.Id]
	if cmd == nil {
		// We got a reply for a cmd that we didn't send.
		return "", nil, fmt.Errorf("unknown reply: %#v", reply)
	}
	delete(p.cmds, reply.Id)

	// If cmd failed then ignore it, don't update the db. The reply is usually
	// sent back as the HTTP response, and the caller is expected to check if
	// reply.Error != "" and report it to user.
	if reply.Error != "" {
		return reply.Id, reply, nil
	}

	// The cmd was successful, so update db if necessary to reflect any state
	// change inside the agent. This processor is serliazing cmds and replies
	// so don't worry about ordering--it's FIFO with the agent and here.
	var err error
	switch cmd.Cmd {
	case "StartService", "StartTool", "SetConfig", "RestartTool":
		err = p.handleSetConfig(cmd, reply)
	case "StopService", "StopTool":
		err = p.handleRemoveConfig(cmd, reply)
	case "Version":
		err = p.handleVersion(cmd, reply)
	case "GetAllConfigs":
		err = p.handleGetAllConfigs(cmd, reply)
	}

	return cmd.Id, reply, err
}

func (p *Processor) Timeout(id string) {
	cmd := p.cmds[id]
	delete(p.cmds, cmd.Id)
}

// --------------------------------------------------------------------------

func (p *Processor) handleSetConfig(cmd *proto.Cmd, reply *proto.Reply) error {
	return p.updateConfig("set", cmd, reply)
}

func (p *Processor) handleRemoveConfig(cmd *proto.Cmd, reply *proto.Reply) error {
	return p.updateConfig("remove", cmd, reply)
}

type configHeader struct {
	UUID string
}

func (p *Processor) updateConfig(change string, cmd *proto.Cmd, reply *proto.Reply) error {
	service := cmd.Service
	otherUUID := ""
	var setConfig, runningConfig []byte

	switch cmd.Cmd {
	case "StartService", "StopService":
		var s proto.ServiceData
		if err := json.Unmarshal(cmd.Data, &s); err != nil {
			return err
		}
		service = s.Name
		setConfig = s.Config
	case "StartTool", "RestartTool":
		var h configHeader
		if err := json.Unmarshal(cmd.Data, &h); err != nil {
			return err
		}
		otherUUID = h.UUID
		setConfig = cmd.Data
		runningConfig = reply.Data
	case "StopTool":
		otherUUID = string(cmd.Data)
	}

	switch change {
	case "set":
		err := p.agentConfigMgr.SetConfig(p.agentId, service, otherUUID, sanitizeConfig(setConfig), runningConfig)
		if err != nil {
			return err
		}
	case "remove":
		err := p.agentConfigMgr.RemoveConfig(p.agentId, service, otherUUID)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("agent.Processor.updateServiceState: invalid change: %s", change)
	}
	return nil
}

func (p *Processor) handleVersion(cmd *proto.Cmd, reply *proto.Reply) error {
	// Every time the agent reports its verseion we update pct.agents.version
	// because it's critical for presenting and configuring version features
	// correctly.
	// pctv3: update oN.instances.properties.version
	return nil
}

func (p *Processor) handleGetAllConfigs(cmd *proto.Cmd, reply *proto.Reply) error {
	// GetAllConfigs is generally only sent in agent.LocalAgent.Start (after
	// Version) to ensure the db is in sync with all configs the agent really
	// has, in case something somehow changed offline or on the agent-side.
	// It's like GetConfig but there are configs for all internal services and
	// whatever tools are running.
	configs := []models.AgentConfig{}
	fmt.Printf("====== Raw config: %v , \n\n ==== ID: %s \n", string(reply.Data), p.agentId)
	if err := json.Unmarshal(reply.Data, &configs); err != nil {
		return fmt.Errorf("proto.Reply.Data is not a valid list of proto.AgentConfig: %s", err)
	}

	err := p.agentConfigMgr.UpdateConfigs(p.agentId, configs, true)
	if err != nil {
		return fmt.Errorf("Cannot update agent configs: %s", err)
	}

	return nil
}

func sanitizeConfig(config []byte) []byte {
	configSet := map[string]string{}
	err := json.Unmarshal(config, &configSet)
	if err != nil {
		return config
	}
	if configDSN, ok := configSet["DSN"]; ok {
		dsn, err := mysql.ParseDSN(configDSN)
		if err != nil {
			return config
		}
		dsn.Passwd = "****"
		configSet["DSN"] = dsn.FormatDSN()
	}

	buf, _ := json.Marshal(configSet)
	return buf
}
