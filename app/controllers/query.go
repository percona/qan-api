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

	queryProto "github.com/percona/pmm/proto/query"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/query"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/stats"
	"github.com/revel/revel"
)

type Query struct {
	BackEnd
}

// GET /queries/:id
func (c *Query) Get(id string) revel.Result {
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Query.Get: dbm.Open")
	}
	queryHandler := query.NewMySQLHandler(dbm, stats.NullStats())
	queries, err := queryHandler.Get([]string{id})
	if err != nil {
		return c.Error(err, "Query.Get: queryHandler.Get")
	}
	query, ok := queries[id]
	if !ok {
		return c.Error(shared.ErrNotFound, "")
	}
	return c.RenderJSON(query)
}

// GET /queries/:id/tables
func (c *Query) GetTables(id string) revel.Result {
	classId := c.Args["classId"].(uint)

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Query.GetTables: dbm.Open")
	}

	queryHandler := query.NewMySQLHandler(dbm, stats.NullStats())
	tables, _, err := queryHandler.Tables(classId, shared.TableParser)
	if err != nil {
		return c.Error(err, "Query.GetTables: queryHandler.Tables")
	}

	return c.RenderJSON(tables)
}

// PUT /queries/:id/tables
func (c *Query) UpdateTables(id string) revel.Result {
	classId := c.Args["classId"].(uint)

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
	var tables []queryProto.Table
	err = json.Unmarshal(body, &tables)
	if err != nil {
		return c.BadRequest(err, "cannot decode Table array")
	}

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Query.UpdateTables: dbm.Open")
	}

	queryHandler := query.NewMySQLHandler(dbm, stats.NullStats())
	if err := queryHandler.UpdateTables(classId, tables); err != nil {
		return c.Error(err, "Query.UpdateTables: queryHandler.Tables")
	}

	return c.RenderNoContent()
}

// GET /queries/:id/examples
func (c *Query) GetExamples(id string) revel.Result {
	classId := c.Args["classId"].(uint)

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Query.GetTables: dbm.Open")
	}

	// ?instance=UUID (optional)
	var instanceId uint
	var instanceUUID string
	c.Params.Bind(&instanceUUID, "instance")
	if instanceUUID != "" {
		var err error
		instanceId, err = instance.GetInstanceId(dbm.DB(), instanceUUID)
		if err != nil {
			return c.Error(err, "Query.GetExamples: GetInstanceId")
		}
		if instanceId == 0 {
			// todo: make error to user reflect that the instance, not the query, is not found
			return c.Error(shared.ErrNotFound, "instance not found: "+instanceUUID)
		}
	}

	queryHandler := query.NewMySQLHandler(dbm, stats.NullStats())
	examples, err := queryHandler.Examples(classId, instanceId)
	if err != nil {
		return c.Error(err, "Query.GetExamples: queryHandler.Examples")
	}

	return c.RenderJSON(examples)
}

// PUT /queries/:id/example
func (c *Query) UpdateExample(id string) revel.Result {
	classId := c.Args["classId"].(uint)

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Query.UpdateExample: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	var example queryProto.Example
	err = json.Unmarshal(body, &example)
	if err != nil {
		return c.BadRequest(err, "cannot decode proto.query.Example")
	}

	if example.QueryId != id {
		return c.BadRequest(nil,
			fmt.Sprintf("query ID mismatch: %s (URI) != %s (proto.Query.Example.QueryId)",
				id, example.QueryId))
	}

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Query.UpdateExample: dbm.Open")
	}

	instanceId, err := instance.GetInstanceId(dbm.DB(), example.InstanceUUID)
	if err != nil {
		return c.Error(err, "Query.UpdateExample: GetInstanceId")
	}
	if instanceId == 0 {
		// todo: make error to user reflect that the instance, not the query, is not found
		return c.Error(shared.ErrNotFound, "query.Example.UUID not found: "+example.InstanceUUID)
	}

	ih := query.NewMySQLHandler(dbm, stats.NullStats())
	if err := ih.UpdateExample(classId, instanceId, example); err != nil {
		return c.Error(err, "Query.UpdateExample: UpdateExample")
	}

	return c.RenderNoContent()
}
