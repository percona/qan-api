// agent - handle agent's endpoints
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

	"github.com/percona/qan-api/app/controllers"
	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/app/shared"
	"github.com/revel/revel"
)

// Agent - base struct for methods to work with agent instance.
type Agent struct {
	controllers.BackEnd
}

// List - GET /agents show all undeleted agents.
func (c Agent) List() revel.Result {
	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
	agentsPtr, err := instanceMgr.GetAllAgents()
	if err != nil {
		return c.Error(err, "Agent.List: models.InstanceManager.GetAllAgents()")
	}
	agents := *agentsPtr
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

// Create - POST /agents register new agent if not exist
func (c Agent) Create() revel.Result {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Agent.Create: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	// fix Instance.Name vs Agent.Hostname
	type Inst models.Instance
	newAgent := struct {
		Inst
		Hostname string
	}{}

	if err = json.Unmarshal(body, &newAgent); err != nil {
		return c.BadRequest(err, "cannot decode models.Instance(Agent)")
	}

	// TODO: need to unify agent and instance struct.
	instance := models.Instance(newAgent.Inst)
	instance.Name = newAgent.Hostname

	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
	agent, err := instanceMgr.Create(&instance)
	if err != nil {
		if err == shared.ErrDuplicateEntry {
			_, err = instanceMgr.GetInstanceID(newAgent.UUID)
			if err != nil {
				_, in, err := instanceMgr.GetByName("agent", newAgent.Hostname, "")
				if err != nil {
					return c.Error(err, "Agent.Create: ih.GetByName")
				}
				uri := c.Args["httpBase"].(string) + "/agents/" + in.UUID
				c.Response.Out.Header().Set("Location", uri)
			}
		}
		return c.Error(err, "Agent.Create: ah.Create")
	}

	return c.RenderCreated(c.Args["httpBase"].(string) + "/agents/" + agent.UUID)
}

// Get - GET /agents/:uuid returns agent instance
func (c Agent) Get(uuid string) revel.Result {
	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])

	_, agent, err := instanceMgr.Get(uuid)
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

// Update -  PUT /agents/:uuid
func (c Agent) Update(uuid string) revel.Result {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "Agent.Update: ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	// fix Instance.Name vs Agent.Hostname
	type Inst models.Instance
	newAgent := struct {
		Inst
		Hostname string
	}{}
	if err = json.Unmarshal(body, &newAgent); err != nil {
		return c.BadRequest(err, "cannot decode models.Instance(Agent)")
	}

	// TODO: need to unify agent and instance struct.
	instance := models.Instance(newAgent.Inst)
	instance.Name = newAgent.Hostname

	instance.UUID = uuid
	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
	err = instanceMgr.Update(instance)
	if err != nil {
		return c.Error(err, "Agent.Update: ah.Update")
	}

	return c.RenderNoContent()
}

// Delete - DELETE /agents/:uuid
// TODO: do we need this method?
func (c Agent) Delete(uuid string) revel.Result {
	return c.RenderNoContent()
}
