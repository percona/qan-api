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

package instance

import (
	"database/sql"
	"fmt"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/pmm/proto"
)

type DbHandler interface {
	Create(in proto.Instance) (uint, error)
	Get(uuid string) (uint, *proto.Instance, error)
	Update(in proto.Instance) error
	Delete(uuid string) error
}

// --------------------------------------------------------------------------

func GetInstanceId(db *sql.DB, uuid string) (uint, error) {
	if uuid == "" {
		return 0, nil
	}
	var instanceId uint
	err := db.QueryRow("SELECT instance_id FROM instances WHERE uuid = ?", uuid).Scan(&instanceId)
	if err != nil {
		return 0, mysql.Error(err, "SELECT instances")
	}
	return instanceId, nil
}

// --------------------------------------------------------------------------

type MySQLHandler struct {
	dbm db.Manager
}

func NewMySQLHandler(dbm db.Manager) *MySQLHandler {
	n := &MySQLHandler{
		dbm: dbm,
	}
	return n
}

func (h *MySQLHandler) Create(in proto.Instance) (uint, error) {
	if in.ParentUUID != "" {
		id, err := GetInstanceId(h.dbm.DB(), in.ParentUUID)
		if err != nil {
			return 0, fmt.Errorf("Error while checking parent uuid: %v", err)
		}
		if id == 0 {
			return 0, fmt.Errorf("invalid parent uuid %s", in.ParentUUID)
		}
	}

	var dsn interface{}
	if in.DSN != "" {
		dsn = in.DSN
	}

	// todo: validate higher up
	subsys, err := GetSubsystemByName(in.Subsystem)
	if err != nil {
		return 0, err
	}

	res, err := h.dbm.DB().Exec(
		"INSERT INTO instances (subsystem_id, parent_uuid, uuid, dsn, name, distro, version) VALUES (?, ?, ?, ?, ?, ?, ?)",
		subsys.Id, in.ParentUUID, in.UUID, dsn, in.Name, in.Distro, in.Version)
	if err != nil {
		return 0, mysql.Error(err, "MySQLHandlerCreate INSERT instances")
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("cannot get instance last insert id")
	}

	return uint(id), nil
}

func (h *MySQLHandler) Get(uuid string) (uint, *proto.Instance, error) {
	query := "SELECT subsystem_id, instance_id, parent_uuid, uuid, dsn, name, distro, version, created, deleted" +
		" FROM instances" +
		" WHERE uuid = ?"
	return h.getInstance(query, uuid)
}

func (h *MySQLHandler) GetByName(subsystem, name string) (uint, *proto.Instance, error) {
	s, err := GetSubsystemByName(subsystem)
	if err != nil {
		return 0, nil, err
	}

	query := "SELECT subsystem_id, instance_id, parent_uuid, uuid, dsn, name,  distro, version, created, deleted" +
		" FROM instances" +
		" WHERE subsystem_id = ? AND name = ?"

	return h.getInstance(query, s.Id, name)
}

func (h *MySQLHandler) GetAll() ([]proto.Instance, error) {
	query := "SELECT subsystem_id, instance_id, parent_uuid, uuid, dsn, name, distro, version, created, deleted" +
		" FROM instances " +
                " WHERE deleted IS NULL " +
		" ORDER BY name"
	rows, err := h.dbm.DB().Query(query)
	if err != nil {
		return nil, mysql.Error(err, "MySQLHandler.GetAll SELECT instances")
	}
	defer rows.Close()

	instances := []proto.Instance{}
	for rows.Next() {
		in := proto.Instance{}
		var instanceId, subsystemId uint
		var dsn, parentUUID, distro, version sql.NullString
		var deleted mysqlDriver.NullTime
		err = rows.Scan(
			&subsystemId,
			&instanceId,
			&parentUUID,
			&in.UUID,
			&dsn,
			&in.Name,
			&distro,
			&version,
			&in.Created,
			&deleted,
		)
		if err != nil {
			return nil, err
		}

		in.ParentUUID = parentUUID.String
		in.DSN = dsn.String
		in.Distro = distro.String
		in.Version = version.String
		in.Deleted = deleted.Time
		subsystem, err := GetSubsystemById(subsystemId) // todo: cache
		if err != nil {
			return nil, err
		}
		in.Subsystem = subsystem.Name

		instances = append(instances, in)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return instances, nil
}

func (h *MySQLHandler) getInstance(query string, params ...interface{}) (uint, *proto.Instance, error) {
	in := &proto.Instance{}

	var instanceId, subsystemId uint
	var dsn, parentUUID, distro, version sql.NullString
	var deleted mysqlDriver.NullTime

	err := h.dbm.DB().QueryRow(query, params...).Scan(
		&subsystemId,
		&instanceId,
		&parentUUID,
		&in.UUID,
		&dsn,
		&in.Name,
		&distro,
		&version,
		&in.Created,
		&deleted,
	)
	if err != nil {
		return 0, nil, mysql.Error(err, "MySQLHandler.Get SELECT instances")
	}

	in.ParentUUID = parentUUID.String
	in.DSN = dsn.String
	in.Distro = distro.String
	in.Version = version.String
	in.Deleted = deleted.Time
	subsystem, err := GetSubsystemById(subsystemId)
	if err != nil {
		return 0, nil, err
	}
	in.Subsystem = subsystem.Name

	return instanceId, in, nil
}

func (h *MySQLHandler) Update(in proto.Instance) error {
	if in.ParentUUID != "" {
		id, err := GetInstanceId(h.dbm.DB(), in.ParentUUID)
		if err != nil {
			return fmt.Errorf("Error while checking parent uuid: %v", err)
		}
		if id == 0 {
			return fmt.Errorf("invalid parent uuid %s", in.ParentUUID)
		}
	}

	_, err := h.dbm.DB().Exec(
		"UPDATE instances SET parent_uuid = ?, dsn = ?, name = ?, distro = ?, version = ? WHERE uuid = ?",
		in.ParentUUID, in.DSN, in.Name, in.Distro, in.Version, in.UUID)
	if err != nil {
		return mysql.Error(err, "MySQLHandler.Update UPDATE instances")
	}

	// todo: return error if no rows affected

	return nil
}

func (h *MySQLHandler) Delete(uuid string) error {
	_, err := h.dbm.DB().Exec("UPDATE instances SET deleted = NOW() WHERE uuid = ?", uuid)
	return mysql.Error(err, "MySQLHandler.Delete UPDATE instances")
}
