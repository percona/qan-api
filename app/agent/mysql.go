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
	"database/sql"
	"fmt"
	"log"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/qan-api/app/instance"
)

type MySQLHandler struct {
	dbm db.Manager
	ih  instance.DbHandler
}

func NewMySQLHandler(dbm db.Manager, ih instance.DbHandler) *MySQLHandler {
	h := &MySQLHandler{
		dbm: dbm,
		ih:  ih,
	}
	return h
}

func (h *MySQLHandler) Create(agent proto.Agent) (string, error) {
	if agent.UUID == "" {
		var uuid string
		if err := h.dbm.DB().QueryRow("SELECT REPLACE(UUID(), '-', '')").Scan(&uuid); err != nil {
			return "", fmt.Errorf("mysql.agent.CreateAgent: uuid: %s", err)
		}
		agent.UUID = uuid
	}

	in := agentToInstance(agent)
	if _, err := h.ih.Create(in); err != nil {
		return "", err
	}

	return agent.UUID, nil
}

func (h *MySQLHandler) Get(uuid string) (uint, *proto.Agent, error) {
	id, in, err := h.ih.Get(uuid)
	if err != nil {
		return 0, nil, err
	}

	agent := &proto.Agent{
		ParentUUID: in.ParentUUID,
		UUID:       in.UUID,
		Hostname:   in.Name,
		Created:    in.Created,
		Deleted:    in.Deleted,
		Version:    in.Version,
	}

	return id, agent, nil
}

func (h *MySQLHandler) GetAll() ([]proto.Agent, error) {
	rows, err := h.dbm.DB().Query(
		"SELECT uuid, parent_uuid, name, version, created, deleted" +
			" FROM instances" +
			" WHERE subsystem_id = 2 ORDER by name")
	if err != nil {
		return nil, mysql.Error(err, "MySQLHandler.GetAll SELECT instances")
	}
	defer rows.Close()

	agents := []proto.Agent{}
	var version sql.NullString
	var deleted mysqlDriver.NullTime
	for rows.Next() {
		agent := proto.Agent{}
		err = rows.Scan(
			&agent.UUID,
			&agent.ParentUUID,
			&agent.Hostname,
			&version,
			&agent.Created,
			&deleted,
		)
		if err != nil {
			return nil, err
		}
		agent.Version = version.String
		agent.Deleted = deleted.Time
		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return agents, nil
}

func (h *MySQLHandler) SetConfig(agentId uint, service, otherUUID string, set, running []byte) error {
	otherId, err := instance.GetInstanceId(h.dbm.DB(), otherUUID)
	if err != nil {
		return err
	}

	// Both configs are NOT NULL.
	if running == nil {
		running = set
	}

	// in_file = set because "set" is a reserved word: https://dev.mysql.com/doc/refman/5.5/en/keywords.html
	_, err = h.dbm.DB().Exec(
		"INSERT INTO agent_configs (agent_instance_id, other_instance_id, service, in_file, running)"+
			" VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE in_file=VALUES(in_file), running=VALUES(running)",
		agentId,
		otherId,
		service,
		string(set),
		string(running),
	)
	return mysql.Error(err, "MySQLHandler.SetConfig INSERT ODKU agent_configs")
}

func (h *MySQLHandler) RemoveConfig(agentId uint, service, otherUUID string) error {
	otherId, err := instance.GetInstanceId(h.dbm.DB(), otherUUID)
	if err != nil {
		return err
	}
	_, err = h.dbm.DB().Exec("DELETE FROM agent_configs WHERE agent_instance_id = ? and other_instance_id = ? and service = ?",
		agentId, otherId, service)
	return mysql.Error(err, "MySQLHandler.RemoveConfig DELETE agent_configs")
}

func (h *MySQLHandler) GetConfigs(agentId uint) ([]proto.AgentConfig, error) {
	rows, err := h.dbm.DB().Query("SELECT service, COALESCE(i.uuid, '') uuid, in_file, running, updated" +
		" FROM agent_configs c LEFT JOIN instances i ON (c.other_instance_id = i.instance_id)")
	if err != nil {
		return nil, mysql.Error(err, "MySQLHandler.GetConfigs SELECT agent_configs instances")
	}
	defer rows.Close()

	configs := []proto.AgentConfig{}
	for rows.Next() {
		config := proto.AgentConfig{}
		err = rows.Scan(
			&config.Service,
			&config.UUID,
			&config.Set,
			&config.Running,
			&config.Updated,
		)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return configs, nil
}

func (h *MySQLHandler) Update(agentId uint, agent proto.Agent) error {
	in := agentToInstance(agent)
	return h.ih.Update(in)

}

func (h *MySQLHandler) UpdateConfigs(agentId uint, configs []proto.AgentConfig, reset bool) error {
	tx, err := h.dbm.DB().Begin()
	if err != nil {
		return err
	}

	ok := false
	defer func() {
		if !ok {
			tx.Rollback()
		}
	}()

	if reset {
		if _, err = tx.Exec("DELETE FROM agent_configs WHERE agent_instance_id = ?", agentId); err != nil {
			return mysql.Error(err, "MySQLHandler.UpdateConfigs DELETE agent_configs")
		}
	}

	for _, config := range configs {
		if config.Running == "" {
			log.Printf("WARN: agent_id=%d: %s running config is empty", agentId, config.Service)
		}

		otherId, err := instance.GetInstanceId(h.dbm.DB(), config.UUID)
		if err != nil {
			return mysql.Error(err, "MySQLHandler.UpdateConfigs")
		}

		_, err = tx.Exec(
			"INSERT INTO agent_configs (agent_instance_id, service, other_instance_id, in_file, running)"+
				" VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE in_file=VALUES(in_file), running=VALUES(running)",
			agentId,
			config.Service,
			otherId,
			config.Set,
			config.Running,
		)
		if err != nil {
			return mysql.Error(err, "MySQLHandler.UpdateConfigs INSERT agent_configs")
		}
	}

	ok = true
	return tx.Commit()
}

func (h *MySQLHandler) UpdateVersion(agentId uint, version proto.Version) error {
	// todo
	return nil
}

// --------------------------------------------------------------------------

func agentToInstance(agent proto.Agent) proto.Instance {
	return proto.Instance{
		Subsystem:  "agent",
		ParentUUID: agent.ParentUUID,
		UUID:       agent.UUID,
		Name:       agent.Hostname,
		Version:    agent.Version,
	}
}
