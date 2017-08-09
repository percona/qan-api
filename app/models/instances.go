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
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/percona/qan-api/app/db/mysql"
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

// Instance represents MySQL or Mongo DB, Server or Agent.
type Instance struct {
	Subsystem  SubSystem `db:"subsystem_id"`
	ParentUUID *string   `db:"parent_uuid"`
	ID         uint      `db:"instance_id"` // internal ID for joining data tables
	UUID       string    // primary ID, for accessing API
	Name       string    // secondary ID, for human readability
	DSN        *string   // type-specific DSN, if any
	Distro     *string
	Version    *string
	Created    time.Time
	Deleted    *time.Time
	Links      map[string]string `json:",omitempty"`
}

// SubSystem represents an enum of instance type
// SubSystem implements the Scanner interface so
// it can be used as a scan destination, similar to NullString
type SubSystem string

// Scan implements the Scanner interface.
func (ss *SubSystem) Scan(src interface{}) error {
	if index, ok := src.(int64); ok {
		subsys, _ := GetSubsystemById(uint(index))
		*ss = SubSystem(subsys.Name)
		return nil
	}
	return errors.New("Cannot convert index to name")
}

// Value implements the driver Valuer interface.
func (ss *SubSystem) Value() (driver.Value, error) {
	subsys, err := GetSubsystemByName(string(*ss))
	return subsys.Id, err
}

// GetAll select not deleted instances.
func (instanceMgr InstanceManager) GetAll() (*[]Instance, error) {
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

// GetByName select instace by subsystem type instance name and optionaly by parrent UUID
func (instanceMgr InstanceManager) GetByName(subsystemName, instanceName, parentUUID string) (uint, *Instance, error) {
	queryGetInstanceByName := `
		SELECT subsystem_id, instance_id, parent_uuid, uuid, dsn, name,  distro, version, created, deleted
			FROM instances
			WHERE subsystem_id = :subsystem_id AND name = :name
	`
	if parentUUID != "" {
		queryGetInstanceByName += "AND parent_uuid = :parent_uuid"
	}

	instance := Instance{}
	subsystem, _ := GetSubsystemByName(subsystemName)
	m := map[string]interface{}{
		"subsystem_id": subsystem.Id,
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
func (instanceMgr InstanceManager) GetInstanceID(uuid string) (uint, error) {
	var instanceID uint
	query := "SELECT instance_id FROM instances WHERE uuid = ?"
	log.Println("Query from sqlite")
	if err := instanceMgr.conns.SQLite.Get(&instanceID, query, uuid); err != nil {
		return 0, mysql.Error(err, "SELECT instances")
	}
	return instanceID, nil
}

// GetInstanceID get instance by instance UUID
func (instanceMgr InstanceManager) Get(uuid string) (uint, *Instance, error) {
	const query = `
		SELECT subsystem_id, instance_id, parent_uuid, uuid, dsn, name, distro, version, created, deleted
			FROM instances
			WHERE uuid = ?
	`
	instance := Instance{}
	log.Println("Query from sqlite")
	if err := instanceMgr.conns.SQLite.Get(&instance, query, uuid); err != nil {
		return 0, nil, mysql.Error(err, "SELECT instances")
	}
	return instance.ID, &instance, nil
}

func (instanceMgr InstanceManager) Create(in Instance) (uint, error) {
	const query = `
		INSERT INTO instances
			(subsystem_id, parent_uuid, uuid, dsn, name, distro, version)
			VALUES
			(:subsystem_id, :parent_uuid, :uuid, :dsn, :name, :distro, :version)
		`
	if in.UUID == "" {
		u4, _ := uuid.NewV4()
		in.UUID = strings.Replace(u4.String(), "-", "", -1)
	}

	log.Println("Query from sqlite")
	result, err := instanceMgr.conns.SQLite.NamedExec(query, in)
	if err != nil {
		return 0, mysql.Error(err, "MySQLHandlerCreate INSERT instances")
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("cannot get instance last insert id")
	}

	return uint(id), nil
}
