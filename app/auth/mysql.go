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

package auth

import (
	_ "github.com/go-sql-driver/mysql"

	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/db/mysql"
)

type MySQLHandler struct {
	dbm db.Manager
}

func NewMySQLHandler(dbm db.Manager) *MySQLHandler {
	h := &MySQLHandler{
		dbm: dbm,
	}
	return h
}

func (h *MySQLHandler) GetAgentId(uuid string) (uint, error) {
	var instanceId uint
	if err := h.dbm.Open(); err != nil {
		return 0, mysql.Error(err, "auth.MySQLHandler.GetAgentId: dbm.Open")
	}
	err := h.dbm.DB().QueryRow(
		"SELECT instance_id FROM instances WHERE uuid = ? AND subsystem_id = 2 AND deleted = 0",
		uuid).Scan(&instanceId)
	return instanceId, mysql.Error(err, "auth.MySQLHandler.GetAgentId: SELECT instances")
}
