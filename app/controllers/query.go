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
	"fmt"
	"io/ioutil"

	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/app/shared"
	queryService "github.com/percona/qan-api/service/query"
	"github.com/revel/revel"
)

// Query is base truct for query controller endpoints
type Query struct {
	BackEnd
}

// Get is endpoint for GET /queries/:id
func (c *Query) Get(id string) revel.Result {
	queryMgr := models.NewQueryManager(c.Args["connsPool"])
	queries, err := queryMgr.Get([]string{id})
	if err != nil {
		return c.Error(err, "Query.Get: queryHandler.Get")
	}
	query, ok := queries[id]
	if !ok {
		return c.Error(shared.ErrNotFound, "")
	}
	return c.RenderJSON(query)
}

// GetTables is endpoint for GET /queries/:id/tables
func (c *Query) GetTables(id string) revel.Result {
	classID := c.Args["classId"].(uint)

	queryMgr := models.NewQueryManager(c.Args["connsPool"])
	tables, _, err := queryMgr.Tables(classID, shared.TableParser)
	if err != nil {
		return c.Error(err, "Query.GetTables: queryHandler.Tables")
	}

	return c.RenderJSON(tables)
}

// UpdateTables is endpoint for PUT /queries/:id/tables
func (c *Query) UpdateTables(id string) revel.Result {
	classID := c.Args["classId"].(uint)

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Query.UpdateTables: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	// We store tables as a JSON string, so we could just store the content
	// body as-is, but let's decode it to make sure it's valid and avoid
	// "garbage in, garbage out".
	var tables []queryService.Table
	err = json.Unmarshal(body, &tables)
	if err != nil {
		return c.BadRequest(err, "cannot decode Table array")
	}

	queryMgr := models.NewQueryManager(c.Args["connsPool"])
	err = queryMgr.UpdateTables(classID, tables)
	if err != nil {
		return c.Error(err, "Query.UpdateTables: queryHandler.Tables")
	}

	return c.RenderNoContent()
}

// GetExamples is endpoint for GET /queries/:id/examples
func (c *Query) GetExamples(id string) revel.Result {
	classID := c.Args["classId"].(uint)

	// ?instance=UUID (optional)
	var instanceID uint
	var instanceUUID string
	c.Params.Bind(&instanceUUID, "instance")
	if instanceUUID != "" {
		var err error
		instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
		instanceID, err = instanceMgr.GetInstanceID(instanceUUID)
		if err != nil {
			return c.Error(err, "Query.GetExamples: GetInstanceId")
		}
		if instanceID == 0 {
			// todo: make error to user reflect that the instance, not the query, is not found
			return c.Error(shared.ErrNotFound, "instance not found: "+instanceUUID)
		}
	}

	queryMgr := models.NewQueryManager(c.Args["connsPool"])
	examples, err := queryMgr.Examples(classID, instanceID)
	if err != nil {
		return c.Error(err, "Query.GetExamples: queryHandler.Examples")
	}

	return c.RenderJSON(examples)
}

// UpdateExample is endpoint for PUT /queries/:id/example
func (c *Query) UpdateExample(id string) revel.Result {
	classID := c.Args["classId"].(uint)

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Query.UpdateExample: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	var example models.Example
	err = json.Unmarshal(body, &example)
	if err != nil {
		return c.BadRequest(err, "cannot decode proto.query.Example")
	}

	if example.QueryID != id {
		return c.BadRequest(nil,
			fmt.Sprintf("query ID mismatch: %s (URI) != %s (proto.Query.Example.QueryId)",
				id, example.QueryID))
	}

	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
	instanceID, err := instanceMgr.GetInstanceID(example.InstanceUUID)
	if err != nil {
		return c.Error(err, "Query.UpdateExample: GetInstanceId")
	}
	if instanceID == 0 {
		// todo: make error to user reflect that the instance, not the query, is not found
		return c.Error(shared.ErrNotFound, "query.Example.UUID not found: "+example.InstanceUUID)
	}

	queryMgr := models.NewQueryManager(c.Args["connsPool"])
	err = queryMgr.UpdateExample(classID, instanceID, example)
	if err != nil {
		return c.Error(err, "Query.UpdateExample: UpdateExample")
	}

	return c.RenderNoContent()
}
