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

package auth_test

import (
	"testing"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/auth"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/config"
	testDb "github.com/percona/qan-api/tests/setup/db"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type HttpTestSuite struct {
	testDb *testDb.Db
	dbh    *auth.MySQLHandler
	auth   *auth.AuthDb
}

var _ = Suite(&HttpTestSuite{})

func (s *HttpTestSuite) SetUpSuite(t *C) {
	dsn := config.Get("mysql.dsn")

	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	err := s.testDb.Start()
	t.Assert(err, IsNil)

	s.dbh = auth.NewMySQLHandler(db.DBManager)
	s.auth = auth.NewAuthDb(s.dbh)
}

// --------------------------------------------------------------------------

func (s *HttpTestSuite) TestAgentOK(t *C) {
	agentId, res, err := s.auth.Agent("212")
	t.Check(err, IsNil)
	t.Check(agentId, Equals, uint(2))
	t.Check(res, DeepEquals, &proto.AuthResponse{Code: 200, Error: ""})
}
