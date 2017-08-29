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

// InstanceManager contains methods to work with DB, server, agent instances;
type InstanceManager struct {
	conns *ConnectionsPool
}

// NewInstanceManager returns InstanceManager with db connections pool.
func NewInstanceManager(conns interface{}) InstanceManager {
	connsPool := conns.(*ConnectionsPool)
	return InstanceManager{connsPool}
}

// InstanceDbHandler define methods for db hendler
// TODO: looks like this is rudimental
type InstanceDbHandler interface {
	Create(in Instance) (uint, error)
	Get(uuid string) (uint, *Instance, error)
	Update(in Instance) error
	Delete(uuid string) error
}

// Instance represents MySQL or Mongo DB, Server or Agent.
type Instance struct {
	ID         uint              `db:"instance_id" json:"Id"` // internal ID for joining data tables
	Subsystem  string            `db:"subsystem_id"`
	ParentUUID string            `db:"parent_uuid"`
	UUID       string            `db:"uuid"` // primary ID, for accessing API
	Name       string            `db:"name"` // secondary ID, for human readability
	DSN        string            `db:"dsn"`  // type-specific DSN, if any
	Distro     string            `db:"distro"`
	Version    string            `db:"version"`
	Created    time.Time         `db:"created"`
	Deleted    time.Time         `db:"deleted"`
	Links      map[string]string `json:",omitempty"`
}

// SubSystem represents an enum of instance type
// SubSystem implements the Scanner interface so
// it can be used as a scan destination, similar to NullString
// type SubSystem string

// // Scan implements the Scanner interface.
// func (ss *SubSystem) Scan(src interface{}) error {
// 	// if index, ok := src.(int64); ok {
// 	// 	subsys, _ := GetSubsystemById(uint(index))
// 	// 	*ss = SubSystem(subsys.Name)
// 	// 	return nil
// 	// }

// 	if val, ok := src.(string); ok {
// 		*ss = SubSystem(val)

// 	}
// 	return errors.New("Cannot convert Subsystem to string")
// 	// subsys, err := GetSubsystemByName(val)
// 	// if err != nil {
// 	// 	return errors.New("Cannot convert Subsystem to name")
// 	// }

// }

// // Value implements the driver Valuer interface.
// func (ss *SubSystem) Value() (driver.Value, error) {
// 	subsys, err := GetSubsystemByName(string(*ss))
// 	return subsys.Id, err
// }

// GetAll select not deleted instances.
func (instanceMgr *InstanceManager) GetAll() (*[]Instance, error) {
	const queryGetAllInstances = `
		SELECT subsystem_id, instance_id, parent_uuid, uuid, dsn, name, distro, version, created, deleted
			FROM instances
			WHERE deleted IS NULL OR deleted = '1970-01-01 00:00:01'
			ORDER BY name
	`
	instances := []Instance{}
	log.Println("Query from sqlite")
	err := instanceMgr.conns.SQLite.Select(&instances, queryGetAllInstances)
	return &instances, err
}

// GetAllAgents - select all undeleted agents
func (instanceMgr *InstanceManager) GetAllAgents() (*[]Instance, error) {
	const queryGetAllInstances = `
		SELECT subsystem_id, instance_id, parent_uuid, uuid, dsn, name, distro, version, created, deleted
			FROM instances
			WHERE subsystem_id = 'agent'
				AND (deleted IS NULL OR deleted = '1970-01-01 00:00:01')
			ORDER BY name
	`
	instances := []Instance{}
	log.Println("Query from sqlite")
	err := instanceMgr.conns.SQLite.Select(&instances, queryGetAllInstances)
	return &instances, err
}

// GetByName select instace by subsystem type instance name and optionaly by parrent UUID
func (instanceMgr *InstanceManager) GetByName(subsystemName, instanceName, parentUUID string) (uint, *Instance, error) {
	queryGetInstanceByName := `
		SELECT subsystem_id, instance_id, parent_uuid, uuid, dsn, name,  distro, version, created, deleted
			FROM instances
			WHERE subsystem_id = :subsystem_id AND name = :name
	`
	if parentUUID != "" {
		queryGetInstanceByName += "AND parent_uuid = :parent_uuid"
	}

	instance := Instance{}
	m := map[string]interface{}{
		"subsystem_id": subsystemName,
		"name":         instanceName,
		"parent_uuid":  parentUUID,
	}
	log.Println("Query from sqlite")
	if nstmt, err := instanceMgr.conns.SQLite.PrepareNamed(queryGetInstanceByName); err != nil {
		return 0, nil, err
	} else if err = nstmt.Get(&instance, m); err != nil {
		return 0, nil, err
	}
	return instance.ID, &instance, nil
}

// GetInstanceID get ID of instance by instance UUID
func (instanceMgr *InstanceManager) GetInstanceID(uuid string) (uint, error) {
	var instanceID uint
	const query = `
		SELECT instance_id FROM instances
		WHERE uuid = ? AND deleted = '1970-01-01 00:00:01'
	`
	if err := instanceMgr.conns.SQLite.Get(&instanceID, query, uuid); err != nil {
		return 0, fmt.Errorf("SELECT instances: %v", err)
	}
	return instanceID, nil
}

// Get get instance by instance UUID
func (instanceMgr *InstanceManager) Get(uuid string) (uint, *Instance, error) {
	const query = `
		SELECT * FROM instances
		WHERE uuid = ? AND deleted = '1970-01-01 00:00:01'
	`
	instance := Instance{}
	log.Println("Query from sqlite")
	if err := instanceMgr.conns.SQLite.Get(&instance, query, uuid); err != nil {
		return 0, nil, fmt.Errorf("SELECT instances: %v", err)
	}
	return instance.ID, &instance, nil
}

// Create - adds register instance.
func (instanceMgr *InstanceManager) Create(in *Instance) (*Instance, error) {
	// https://sqlite.org/lang_corefunc.html#randomblob
	const query = `
		INSERT INTO instances
			(subsystem_id, parent_uuid, uuid, dsn, name, distro, version)
			VALUES
			(:subsystem_id, :parent_uuid,
			case when :uuid = "" then lower(hex(randomblob(16))) else :uuid end,
			:dsn, :name, :distro, :version)
	`
	result, err := instanceMgr.conns.SQLite.NamedExec(query, in)
	if err != nil {
		return nil, fmt.Errorf("MySQLHandlerCreate INSERT instances: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("cannot get instance last insert id: %v", err)
	}
	const querySelectInstance = `
		SELECT * from instances WHERE instance_id = ?
	`
	instance := &Instance{}
	err = instanceMgr.conns.SQLite.Get(instance, querySelectInstance, id)
	if err != nil {
		return nil, fmt.Errorf("cannot get instance by id: %v", err)
	}

	return instance, nil
}

// Update instance.
func (instanceMgr *InstanceManager) Update(instance Instance) error {
	if instance.ParentUUID != "" {
		id, err := instanceMgr.GetInstanceID(instance.ParentUUID)
		if err != nil {
			return fmt.Errorf("Error while checking parent uuid: %v", err)
		}
		if id == 0 {
			return fmt.Errorf("invalid parent uuid %s", instance.ParentUUID)
		}
	}

	const query = `
		UPDATE instances
		SET parent_uuid = :parent_uuid, dsn = :dsn, name = :name,
			distro = :distro, version = :version, deleted = :deleted
		WHERE uuid = :uuid
	`
	_, err := instanceMgr.conns.SQLite.NamedExec(query, instance)
	if err != nil {
		return fmt.Errorf("MySQLHandler.Update UPDATE instances: %v", err)
	}

	return nil
}

// Delete - mark instance as deleted.
func (instanceMgr *InstanceManager) Delete(uuid string) error {
	const query = `
		UPDATE instances SET deleted = NOW() WHERE uuid = ?
	`
	_, err := instanceMgr.conns.SQLite.Exec(query, uuid)
	if err != nil {
		return fmt.Errorf("InstanceManager DELETE instances: %v", err)
	}
	return nil
}
