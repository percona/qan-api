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

package models

import (
	"fmt"
	"log"
	"time"
)

// AgentConfig - configuration of agent.
type AgentConfig struct {
	Service string    `db:"service"` // agent service (data, log, qan, etc.)
	UUID    string    `db:"uuid"`    // of MySQL instance if Service = qan
	Set     string    `db:"set"`     // config/config.go as set explicitly in the file
	Running string    `db:"running"` // ^ as the agent is running it with default applied
	Updated time.Time `db:"updated"`
}

type argsAgentConfig struct {
	AgentInstanceID uint   `db:"agent_instance_id"`
	OtherInstanceID uint   `db:"other_instance_id"`
	Service         string `db:"service"`
	InFile          string `db:"in_file"`
	Running         string `db:"running"`
}

// RunningQAN - agent running config - deprecated - must not to be asked by agent any more
type RunningQAN struct {
	AgentUUID     string `db:"agent_uuid"`
	SetConfig     string `json:",omitempty" db:"in_file"`
	RunningConfig string `json:",omitempty" db:"running"`
}

// AgentConfigManager contains methods to work with DB, server, agent instances;
// Rudimental - agent must store its config locally
type AgentConfigManager struct {
	conns *ConnectionsPool
}

// NewAgentConfigManager returns AgentConfigManager with db connections pool.
func NewAgentConfigManager(conns interface{}) AgentConfigManager {
	connsPool := conns.(*ConnectionsPool)
	return AgentConfigManager{connsPool}
}

// SetConfig store config into db.
func (acm *AgentConfigManager) SetConfig(agentID uint, service, otherUUID string, set, running []byte) error {
	instanceMgr := NewInstanceManager(acm.conns)
	otherID, err := instanceMgr.GetInstanceID(otherUUID)
	if err != nil {
		return fmt.Errorf("Cannot cannot get instance id agent_config.go@SetConfig: %v", err)
	}

	// Both configs are NOT NULL.
	if running == nil {
		running = set
	}

	// in_file = set because "set" is a reserved word: https://dev.mysql.com/doc/refman/5.5/en/keywords.html
	const query = `
		REPLACE INTO agent_configs (agent_instance_id, other_instance_id, service, in_file, running)
		VALUES (:agent_instance_id, :other_instance_id, :service, :in_file, :running)
	`

	nstmt, err := acm.conns.SQLite.PrepareNamed(query)
	if err != nil {
		return fmt.Errorf("Cannot prepare named statement for agent_config.go@SetConfig: %v", err)
	}

	args := argsAgentConfig{agentID, otherID, service, string(set), string(running)}
	_, err = nstmt.Exec(args)
	if err != nil {
		return fmt.Errorf("Cannot execute named statement for agent_config.go@SetConfig: %v", err)
	}

	log.Println("Agent config was set.")
	return nil
}

// RemoveConfig remove agent confog from db.
func (acm *AgentConfigManager) RemoveConfig(agentID uint, service, otherUUID string) error {

	instanceMgr := NewInstanceManager(acm.conns)
	otherID, err := instanceMgr.GetInstanceID(otherUUID)
	if err != nil {
		return fmt.Errorf("Cannot cannot get instance id agent_config.go@SetConfig: %v", err)
	}

	const query = `
		DELETE FROM agent_configs
		WHERE agent_instance_id = :agent_instance_id AND other_instance_id = :other_instance_id AND service = :service
	`

	nstmt, err := acm.conns.SQLite.PrepareNamed(query)
	if err != nil {
		return fmt.Errorf("Cannot prepare named statement for agent_config.go@RemoveConfig: %v", err)
	}

	args := argsAgentConfig{
		AgentInstanceID: agentID,
		OtherInstanceID: otherID,
		Service:         service,
	}
	_, err = nstmt.Exec(args)
	if err != nil {
		return fmt.Errorf("Cannot execute named statement for agent_config.go@RemoveConfig: %v", err)
	}

	log.Println("Agent config was removed.")
	return nil
}

// GetConfigs returns all configs of agent.
// TODO: do we need agentID at all?
func (acm *AgentConfigManager) GetConfigs(agentID uint) ([]AgentConfig, error) {
	const query = `
		SELECT service, i.uuid, in_file, running, updated
		FROM agent_configs c LEFT JOIN instances i ON (c.other_instance_id = i.instance_id)
	`

	configs := []AgentConfig{}
	err := acm.conns.SQLite.Select(&configs, query)
	if err != nil {
		return nil, fmt.Errorf("Cannot get configs: %v", err)
	}

	return configs, nil
}

// UpdateConfigs - update agent configuration in db.
func (acm *AgentConfigManager) UpdateConfigs(agentID uint, configs []AgentConfig, reset bool) error {
	tx := acm.conns.SQLite.MustBegin()

	ok := false
	defer func() {
		if !ok {
			tx.Rollback()
		}
	}()

	if reset {
		_, err := tx.Exec("DELETE FROM agent_configs WHERE agent_instance_id = ?", agentID)
		if err != nil {
			return fmt.Errorf("Cannot delete agent config: %v", err)
		}
	}

	const query = `
		REPLACE INTO agent_configs (agent_instance_id, service, other_instance_id, in_file, running)
		VALUES (:agent_instance_id, :service, :other_instance_id, :in_file, :running)
	`

	nstmt, err := tx.PrepareNamed(query)
	if err != nil {
		return fmt.Errorf("Cannot prepare named fo UpdateConfigs: %v", err)
	}
	fmt.Printf("============\n configs: %+v \n ================ \n", configs)

	for _, config := range configs {
		if config.Running == "" {
			log.Printf("WARN: agent_id=%d: %s running config is empty", agentID, config.Service)
		}

		// -----
		// instanceMgr := NewInstanceManager(acm.conns)
		// otherID, err := instanceMgr.GetInstanceID(config.UUID)
		// TODO: fix this. agent or pmm-admin must send mysql or mongo instance uuid config.UUID
		var otherID uint
		const q = `
			SELECT instance_id FROM instances
			WHERE subsystem_id = 'mysql' AND
			parent_uuid = (SELECT parent_uuid FROM instances WHERE instance_id = ?)
		`
		err = tx.Get(&otherID, q, agentID)
		// -----
		if err != nil {
			return fmt.Errorf("Cannot get instance id to update agent config: %v", err)
		}

		args := argsAgentConfig{
			AgentInstanceID: agentID,
			OtherInstanceID: otherID,
			Service:         config.Service,
			InFile:          config.Set,
			Running:         config.Running,
		}

		_, err = nstmt.Exec(args)
		if err != nil {
			return fmt.Errorf("Cannot execute fo UpdateConfigs: %v", err)
		}
	}

	ok = true
	return tx.Commit()
}

// GetQAN - select RunningQAN
func (acm *AgentConfigManager) GetQAN(instanceID uint) ([]RunningQAN, error) {
	const query = `
		SELECT IFNULL(i.uuid, '') AS agent_uuid, c.in_file, c.running
		FROM agent_configs c
		LEFT JOIN instances i ON (c.agent_instance_id = i.instance_id)
		WHERE service='qan' AND other_instance_id = ?
	`
	configs := []RunningQAN{}
	err := acm.conns.SQLite.Select(&configs, query, instanceID)
	if err != nil {
		return nil, fmt.Errorf("Cannot select RunningQAN: %v", err)
	}
	return configs, nil
}
