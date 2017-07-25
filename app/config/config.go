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

package config

import (
	_ "github.com/go-sql-driver/mysql"
	pc "github.com/percona/pmm/proto/config"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/stats"
)

type MySQLHandler struct {
	dbm   db.Manager
	stats *stats.Stats
}

func NewMySQLHandler(dbm db.Manager, stats *stats.Stats) *MySQLHandler {
	h := &MySQLHandler{
		dbm:   dbm,
		stats: stats,
	}
	return h
}

func (h *MySQLHandler) GetQAN(instanceId uint) ([]pc.RunningQAN, error) {
	q := "SELECT COALESCE(i.uuid, '') AS agentUUID, c.in_file, c.running" +
		" FROM agent_configs c" +
		" LEFT JOIN instances i ON (c.agent_instance_id = i.instance_id)" +
		" WHERE service='qan' AND other_instance_id = ?"
	rows, err := h.dbm.DB().Query(q, instanceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := []pc.RunningQAN{}
	for rows.Next() {
		config := pc.RunningQAN{}
		if err := rows.Scan(&config.AgentUUID, &config.SetConfig, &config.RunningConfig); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, nil
}
