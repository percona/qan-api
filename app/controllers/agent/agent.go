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

package agent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/agent"
	"github.com/percona/qan-api/app/controllers"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/shared"
	"github.com/revel/revel"
)

type Agent struct {
	controllers.BackEnd
}

// GET /agents
func (c Agent) List() revel.Result {
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Agent.List: dbm.Open")
	}
	agentHandler := agent.NewMySQLHandler(dbm, instance.NewMySQLHandler(dbm))
	agents, err := agentHandler.GetAll()
	if err != nil {
		return c.Error(err, "Agent.List: ah.GetAll")
	}

	httpBase := c.Args["httpBase"].(string)
	for i, a := range agents {
		agents[i].Links = map[string]string{
			"self":   fmt.Sprintf("%s/agents/%s", httpBase, a.UUID),
			"status": fmt.Sprintf("%s/agents/%s/status", httpBase, a.UUID),
			"cmd":    fmt.Sprintf("%s/agents/%s/cmd", httpBase, a.UUID),
			"log":    fmt.Sprintf("%s/agents/%s/log", httpBase, a.UUID),
			"data":   fmt.Sprintf("%s/agents/%s/data", httpBase, a.UUID),
		}
	}

	return c.RenderJSON(agents)
}

// POST /agents
func (c Agent) Create() revel.Result {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Agent.Create: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	var newAgent proto.Agent
	if err = json.Unmarshal(body, &newAgent); err != nil {
		return c.BadRequest(err, "cannot decode proto.Agent")
	}

	// todo: validate agent

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Agent.Create: dbm.Open")
	}
	agentHandler := agent.NewMySQLHandler(dbm, instance.NewMySQLHandler(dbm))
	uuid, err := agentHandler.Create(newAgent)
	if err != nil {
		if err == shared.ErrDuplicateEntry {
			ih := instance.NewMySQLHandler(dbm)
			id, _ := instance.GetInstanceId(dbm.DB(), newAgent.UUID)
			if id == 0 {
				_, in, err := ih.GetByName("agent", newAgent.Hostname, "")
				if err != nil {
					return c.Error(err, "Agent.Create: ih.GetByName")
				}
				uri := c.Args["httpBase"].(string) + "/agents/" + in.UUID
				c.Response.Out.Header().Set("Location", uri)
			}
		}
		return c.Error(err, "Agent.Create: ah.Create")
	}

	return c.RenderCreated(c.Args["httpBase"].(string) + "/agents/" + uuid)
}

// GET /agents/:uuid
func (c Agent) Get(uuid string) revel.Result {
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Agent.Get: dbm.Open")
	}
	agentHandler := agent.NewMySQLHandler(dbm, instance.NewMySQLHandler(dbm))
	_, agent, err := agentHandler.Get(uuid)
	if err != nil {
		return c.Error(err, "Agent.Get: ah.Get")
	}

	httpBase := c.Args["httpBase"].(string)
	wsBase := c.Args["wsBase"].(string)
	agent.Links = map[string]string{
		"self": fmt.Sprintf("%s/agents/%s", httpBase, uuid),
		"cmd":  fmt.Sprintf("%s/agents/%s/cmd", wsBase, uuid),
		"log":  fmt.Sprintf("%s/agents/%s/log", wsBase, uuid),
		"data": fmt.Sprintf("%s/agents/%s/data", wsBase, uuid),
	}

	return c.RenderJSON(agent)
}

// PUT /agents/:uuid
func (c Agent) Update(uuid string) revel.Result {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Agent.Update: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	var newAgent proto.Agent
	if err = json.Unmarshal(body, &newAgent); err != nil {
		return c.BadRequest(err, "cannot decode proto.Agent")
	}

	// todo: validate agent

	// Connect to MySQL.
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Agent.Update: dbm.Open")
	}

	agentId := c.Args["agentId"].(uint)
	agentHandler := agent.NewMySQLHandler(dbm, instance.NewMySQLHandler(dbm))
	if err = agentHandler.Update(agentId, newAgent); err != nil {
		return c.Error(err, "Agent.Update: ah.Update")
	}

	return c.RenderNoContent()
}

// DELETE /agents/:uuid
func (c Agent) Delete(uuid string) revel.Result {
	// todo
	return c.RenderNoContent()
}
