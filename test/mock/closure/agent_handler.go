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

package closure

import (
	"github.com/percona/pmm/proto"
)

type AgentHandlerMock struct {
	DbConnectionMock
	SetConfigMock     func(agentId uint, service, otherUUID string, config []byte) error
	RemoveConfigMock  func(agentId uint, service, otherUUID string) error
	UpdateConfigsMock func(agentId uint, configs []proto.AgentConfig, reset bool) error
	CreateAgentMock   func(agent proto.Agent) (string, error)
	UpdateAgentMock   func(agentId uint, agent proto.Agent) error
	UpdateVersionMock func(agentId uint, version proto.Version) error
	AgentVersionsMock func(agentId uint) (map[string]string, error)
	GetMock           func(uuid string) (uint, *proto.Agent, error)
}

func (a *AgentHandlerMock) OpenDbh(dsn string, dbKey string) error {
	return nil
}

func (a *AgentHandlerMock) CloseDbh(dbKey string) error {
	return nil
}

func (a *AgentHandlerMock) Get(uuid string) (uint, *proto.Agent, error) {
	if a.GetMock != nil {
		return a.GetMock(uuid)
	}
	return 0, nil, nil
}

func (a *AgentHandlerMock) GetAll() ([]proto.Agent, error) {
	return nil, nil
}

func (a *AgentHandlerMock) Delete(agentId uint) (err error) {
	return nil
}

func (a *AgentHandlerMock) Remove(agentId uint) (err error) {
	return nil
}

func (a *AgentHandlerMock) SetConfig(agentId uint, service, otherUUID string, config []byte) error {
	if a.SetConfigMock != nil {
		return a.SetConfigMock(agentId, service, otherUUID, config)
	}
	return nil
}

func (a *AgentHandlerMock) RemoveConfig(agentId uint, service, otherUUID string) error {
	if a.RemoveConfigMock != nil {
		return a.RemoveConfigMock(agentId, service, otherUUID)
	}
	return nil
}

func (a *AgentHandlerMock) UpdateConfigs(agentId uint, configs []proto.AgentConfig, reset bool) error {
	if a.UpdateConfigs != nil {
		return a.UpdateConfigsMock(agentId, configs, reset)
	}
	return nil
}

func (a *AgentHandlerMock) Update(agentId uint, agent proto.Agent) error {
	return a.UpdateAgentMock(agentId, agent)
}

func (a *AgentHandlerMock) UpdateVersion(agentId uint, version proto.Version) (err error) {
	return a.UpdateVersionMock(agentId, version)
}

func (a *AgentHandlerMock) Create(agent proto.Agent) (string, error) {
	return a.CreateAgentMock(agent)
}

func (a *AgentHandlerMock) AgentVersions(agentId uint) (versions map[string]string, err error) {
	return a.AgentVersionsMock(agentId)
}
