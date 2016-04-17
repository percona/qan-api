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

import "database/sql"

type DbConnectionMock struct {
	OpenMock     func(dsn string) error
	GetDsnMock   func() string
	OpenDbMock   func(dbKey string) error
	OpenDbhMock  func(dsn string, dbKey string) error
	ConnectMock  func(dsn string, tries uint) error
	CloseMock    func() error
	CloseDbhMock func(dbKey string) error
	UseMock      func(db string) error
	BeginMock    func() error
	CommitMock   func() error
}

func (a *DbConnectionMock) Open(dsn string) (err error) {
	return a.OpenMock(dsn)
}

func (a *DbConnectionMock) GetDsn() string {
	return a.GetDsnMock()
}

func (a *DbConnectionMock) AddDb(dsn string, dbKey string) {
}

func (a *DbConnectionMock) OpenDb(dbKey string) error {
	return a.OpenDbMock(dbKey)
}
func (a *DbConnectionMock) Dbh(dbKey string) *sql.DB {
	return nil
}

func (a *DbConnectionMock) OpenDbh(dsn string, dbKey string) error {
	return a.OpenDbhMock(dsn, dbKey)
}

func (a *DbConnectionMock) Connect(dsn string, tries uint) error {
	return a.ConnectMock(dsn, tries)
}

func (a *DbConnectionMock) Close() (err error) {
	return a.Close()
}

func (a *DbConnectionMock) CloseDbh(dbKey string) error {
	return a.CloseDbhMock(dbKey)
}

func (a *DbConnectionMock) Use(db string) (err error) {
	return a.Use(db)
}

func (a *DbConnectionMock) Begin() (err error) {
	return a.BeginMock()
}

func (a *DbConnectionMock) Commit() (err error) {
	return a.CommitMock()
}
