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
	"github.com/percona/pmm/proto"
)

type DbHandler interface {
	Create(agent proto.Agent) (string, error)
	Get(uuid string) (uint, *proto.Agent, error)
	GetAll() ([]proto.Agent, error)
	SetConfig(agentId uint, service, otheUUID string, set, running []byte) error
	RemoveConfig(agentId uint, service, otherUUID string) error
	Update(agentId uint, agent proto.Agent) error
	UpdateConfigs(agentId uint, configs []proto.AgentConfig, reset bool) error
	UpdateVersion(agentId uint, version proto.Version) error
}
