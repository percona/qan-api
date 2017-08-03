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

package mysql

import (
	"database/sql"
	"sync"
	"time"

	"github.com/percona/qan-api/config"
)

type Manager struct {
	dsn string
	db  *sql.DB
	*sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		dsn:     config.Get("mysql.dsn"),
		RWMutex: &sync.RWMutex{},
	}
}

// Deprecated
func (m *Manager) Open() error {
	m.Lock()
	defer m.Unlock()

	if m.db != nil {
		return nil
	}

	db, err := sql.Open("mysql", m.dsn)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}

	// Logically, we want 1 conneciton per request, but some request need 2
	// physical connections because conn 1 will start a transaction, then do
	// some other queries (e.g. get an instance ID given its UUID).
	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(2)
	// it should autoclose lost descriptors
	db.SetConnMaxLifetime(time.Duration(30) * time.Second)

	m.db = db
	return nil
}

func (m *Manager) DB() *sql.DB {
	return m.db
}

func (m *Manager) Close() error {
	m.Lock()
	defer m.Unlock()
	var err error
	if m.db != nil {
		err = m.db.Close()
		m.db = nil
	}
	return err
}
