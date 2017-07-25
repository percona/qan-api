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

package instance_test

import (
	"testing"
	"time"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/db"
	appInstance "github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/config"
	testDb "github.com/percona/qan-api/tests/setup/db"
	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type InstanceTestSuite struct {
	testDb *testDb.Db
}

var _ = Suite(&InstanceTestSuite{})

func (s *InstanceTestSuite) SetUpSuite(t *C) {
	dsn := config.Get("mysql.dsn")
	s.testDb = testDb.NewDb(dsn, config.SchemaDir, config.TestDir)
	if err := s.testDb.Start(); err != nil {
		t.Fatalf("Could not prepare org db: %s", err)
	}
}

func (s *InstanceTestSuite) SetUpTest(t *C) {
	s.testDb.DB().Exec("TRUNCATE TABLE instances")
	s.testDb.LoadDataInfiles(config.ApiRootDir + "/test/instances/008")
}

func (s *InstanceTestSuite) TearDownSuite(t *C) {
	s.testDb.DB().Exec("TRUNCATE TABLE instances")
}

// --------------------------------------------------------------------------

func (s *InstanceTestSuite) TestGet(t *C) {
	ih := appInstance.NewMySQLHandler(db.DBManager)
	_, instance, err := ih.Get("00000000000000000000000000000000") // Invalid UUID
	t.Assert(err, NotNil)
	t.Check(err, Equals, shared.ErrNotFound)

	_, instance, err = ih.Get("2e6eef26d97b48b4af3179d414899d57")
	t.Assert(err, IsNil)

	expected := &proto.Instance{
		Subsystem:  "os",
		ParentUUID: "",
		UUID:       "2e6eef26d97b48b4af3179d414899d57",
		Name:       "os-001",
		DSN:        "",
		Created:    time.Date(2015, 04, 22, 19, 22, 46, 0, time.UTC),
	}
	assert.Equal(t, instance, expected)
}

func (s *InstanceTestSuite) TestCreate(t *C) {
	ih := appInstance.NewMySQLHandler(db.DBManager)

	instance := proto.Instance{
		Subsystem:  "agent",
		ParentUUID: "2e6eef26dc0093749bc7e7aa910a1112",
		UUID:       "200",
		Name:       "agent2",
		DSN:        "dsn",
		Distro:     "MySQL",
		Version:    "5.5.25",
	}

	id, err := ih.Create(instance)
	t.Assert(err, IsNil)
	t.Check(id, Equals, uint(6))

	// Should get HTTP code 409 (conflict) on creating duplicate instance.
	_, err = ih.Create(instance)
	t.Assert(err, Equals, shared.ErrDuplicateEntry)

	gotId, got, err := ih.Get(instance.UUID)
	t.Assert(err, IsNil)
	t.Check(gotId, Equals, uint(6))
	t.Check(got.Subsystem, Equals, "agent")
	t.Check(got.UUID, Equals, instance.UUID)
	t.Check(got.ParentUUID, Equals, instance.ParentUUID)
	t.Check(got.Name, Equals, instance.Name)
	t.Check(got.Distro, Equals, instance.Distro)
	t.Check(got.Version, Equals, instance.Version)
	t.Check(got.Created.After(time.Time{}), Equals, true)
	t.Check(got.Deleted.After(time.Time{}), Equals, false)
}

func (s *InstanceTestSuite) TestUpdate(t *C) {
	ih := appInstance.NewMySQLHandler(db.DBManager)

	mysqlUUID := "3d341070b1d74a84bb5f797c3ddbccc4"

	id, instance, err := ih.Get(mysqlUUID)
	t.Assert(err, IsNil)
	t.Assert(id, Equals, uint(3))

	instance.Name = "fipar"
	instance.DSN = "new dsn"
	err = ih.Update(*instance)
	t.Assert(err, IsNil)

	_, instance, err = ih.Get(mysqlUUID)
	t.Assert(err, IsNil)
	t.Check(instance.Name, Equals, "fipar")
	t.Check(instance.DSN, Equals, "new dsn")
}
