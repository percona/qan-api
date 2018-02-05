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

package controllers

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/nu7hatch/gouuid"
	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/shared"
	"github.com/revel/revel"
)

type Instance struct {
	BackEnd
}

// GET /instances
func (c *Instance) List() revel.Result {
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Instance.List: dbm.Open")
	}
	instanceHandler := instance.NewMySQLHandler(dbm)

	var instanceType, instanceName, parentUUID string
	c.Params.Bind(&instanceType, "type")
	c.Params.Bind(&instanceName, "name")
	c.Params.Bind(&parentUUID, "parent_uuid")
	if instanceType != "" && instanceName != "" {
		_, in, err := instanceHandler.GetByName(instanceType, instanceName, parentUUID)
		if err != nil {
			return c.Error(err, "Instance.List: ih.GetByName")
		}
		if in == nil {
			return c.Error(shared.ErrNotFound, "Instance.List: ih.GetByName")
		}
		return c.RenderJSON(in)
	} else {
		instances, err := instanceHandler.GetAll()
		if err != nil {
			return c.Error(err, "Instance.List: ih.GetAll")
		}
		return c.RenderJSON(instances)
	}
}

// POST /instances
func (c *Instance) Create() revel.Result {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Instance.Create: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	in := proto.Instance{}
	err = json.Unmarshal(body, &in)
	if err != nil {
		return c.BadRequest(err, "cannot decode proto.Instance")
	}

	if in.UUID == "" {
		u4, _ := uuid.NewV4()
		in.UUID = strings.Replace(u4.String(), "-", "", -1)
	}

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Instance.Create: dbm.Open")
	}
	ih := instance.NewMySQLHandler(dbm)
	if _, err := ih.Create(in); err != nil {
		if err == shared.ErrDuplicateEntry {
			id, _ := instance.GetInstanceId(dbm.DB(), in.UUID)
			if id == 0 {
				_, in2, err := ih.GetByName(in.Subsystem, in.Name, "")
				if err != nil {
					return c.Error(err, "Instance.Create: ih.GetByName")
				}
				in = *in2
			}
			uri := c.Args["httpBase"].(string) + "/instances/" + in.UUID
			c.Response.Out.Header().Set("Location", uri)
		}
		return c.Error(err, "Instance.Create: ih.Create")
	}

	return c.RenderCreated(c.Args["httpBase"].(string) + "/instances/" + in.UUID)
}

// GET /instances/:uuid
func (c *Instance) Get(uuid string) revel.Result {
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Instance.Get: dbm.Open")
	}
	instanceHandler := instance.NewMySQLHandler(dbm)
	_, instance, err := instanceHandler.Get(uuid)
	if err != nil {
		return c.Error(err, "Instance.Get: ih.Get")
	}
	return c.RenderJSON(instance)
}

// PUT /instances/:uuid
func (c *Instance) Update(uuid string) revel.Result {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Instance.Update: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	in := proto.Instance{}
	err = json.Unmarshal(body, &in)
	if err != nil {
		return c.BadRequest(err, "cannot decode proto.Instance")
	}

	// I don't want to use a different proto.Instance not having the uuid
	// to avoid having a million of different structs, so, the body can have
	// an uuid but I'm going to rewrite it with the value from the route.
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Instance.Update: dbm.Open")
	}
	in.UUID = uuid
	ih := instance.NewMySQLHandler(dbm)
	if err := ih.Update(in); err != nil {
		return c.Error(err, "Instance.Update: ih.Update")
	}

	uri := c.Args["httpBase"].(string) + "/instances/" + in.UUID
	c.Response.Out.Header().Set("Location", uri)

	return c.RenderNoContent()
}

// DELETE /instances/:uuid
func (c *Instance) Delete(uuid string) revel.Result {
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Instance.Delete: dbm.Open")
	}
	ih := instance.NewMySQLHandler(dbm)
	if err := ih.Delete(uuid); err != nil {
		return c.Error(err, "Instance.Delete: ih.Delete")
	}
	return c.RenderNoContent()
}
