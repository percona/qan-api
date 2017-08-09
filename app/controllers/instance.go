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

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/app/shared"
	"github.com/revel/revel"
)

// Instance controller
type Instance struct {
	BackEnd
}

// List uses for GET /instances
func (c *Instance) List() revel.Result {
	var instanceType, instanceName, parentUUID string
	c.Params.Bind(&instanceType, "type")
	c.Params.Bind(&instanceName, "name")
	c.Params.Bind(&parentUUID, "parent_uuid")
	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
	if instanceType != "" && instanceName != "" {
		_, in, err := instanceMgr.GetByName(instanceType, instanceName, parentUUID)
		if err != nil {
			return c.Error(err, "Instance.List: models.InstanceManager.GetByName")
		}
		return c.RenderJson(in)
	}

	instances, err := instanceMgr.GetAll()
	if err != nil {
		return c.Error(err, "Instance.List: models.InstanceManager.GetAll()")
	}
	return c.RenderJson(instances)
}

// Create uses for POST /instances
func (c *Instance) Create() revel.Result {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Instance.Create: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	inst := models.Instance{}
	err = json.Unmarshal(body, &inst)
	if err != nil {
		return c.BadRequest(err, "cannot decode proto.Instance")
	}
	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
	if &inst.ParentUUID != nil {
		id, err := instanceMgr.GetInstanceID(*inst.ParentUUID)
		if err != nil || id == 0 {
			return c.BadRequest(err, "Invalid parent uuid")
		}
	}

	_, err = instanceMgr.Create(inst)
	if err != nil && err != shared.ErrDuplicateEntry {
		return c.Error(err, "Instance.Create: models.InstanceManager.Create")
	}

	// TODO: investigate references and simplify
	if err == shared.ErrDuplicateEntry {
		id, _ := instanceMgr.GetInstanceID(inst.UUID)
		if id == 0 {
			_, inst2, err := instanceMgr.GetByName(string(inst.Subsystem), inst.Name, "")
			if err != nil {
				return c.Error(err, "Instance.Create: models.InstanceManager.GetByName")
			}
			inst = *inst2
		}
		uri := c.Args["httpBase"].(string) + "/instances/" + inst.UUID
		c.Response.Out.Header().Set("Location", uri)
	}

	return c.RenderCreated(c.Args["httpBase"].(string) + "/instances/" + inst.UUID)
}

// Get uses for GET /instances/:uuid
func (c *Instance) Get(uuid string) revel.Result {
	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
	_, instance, err := instanceMgr.Get(uuid)
	if err != nil {
		return c.Error(err, "Instance.Get: models.InstanceManager.Get")
	}
	return c.RenderJson(instance)
}

// Update uses for PUT /instances/:uuid
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

// Delete uses for DELETE /instances/:uuid
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
