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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/agent"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/stats"
	"github.com/pkg/errors"
	"github.com/revel/revel"

	"github.com/percona/qan-api/config"
)

// This is the path where we store zip files from the CollectInfo method from the agent.
// This path should be accessible from Nginx ta let the user download these files and attach
// them to emails for Percona Managed Services team.
// This variable is being populated in the init() method to avoid reading the config file
// every time we need to store a file.
var collectInfoFileStoragePath string

func init() {
	collectInfoFileStoragePath = config.Get("api.collect.path")
}

// PUT /agents/:uuid/cmd
func (c Agent) SendCmd(uuid string) revel.Result {
	agentId := c.Args["agentId"].(uint)

	// Read the proto.Cmd from the client.
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return c.Error(err, "ioutil.ReadAll")
	}
	if len(body) == 0 {
		return c.BadRequest(nil, "empty body (no data posted)")
	}

	// Decode the cmd.
	cmd := &proto.Cmd{}
	if err := json.Unmarshal(body, cmd); err != nil {
		return c.BadRequest(err, "cannot decode proto.Cmd")
	}

	// Get the agent.
	comm := shared.AgentDirectory.Get(agentId)
	if comm == nil {
		return c.Error(shared.ErrAgentNotConnected, "shared.AgentDirectory.Get")
	}

	// Send the command, get the agent's reply.
	reply, err := comm.Send(cmd)
	if err != nil {
		return c.Error(err, "comm.Send")
	}

	dst := struct {
		Filename string
		Data     []byte
	}{}
	err = json.Unmarshal(reply.Data, &dst)
	if dst.Filename != "" {
		err := c.writeResponseFile(dst.Filename, dst.Data)
		// Don't send the data to the UI
		dst.Data = []byte{}
		reply.Data, _ = json.Marshal(dst)
		if err != nil {
			reply.Error = fmt.Sprintf("cannot write output file %q: %s", dst.Filename, err.Error())
		}
	}

	return c.RenderJSON(reply)
}

func (c *Agent) writeResponseFile(filename string, data []byte) error {
	// Te response has a zip file encoded as base64 to be able to send
	// binary data as a json payload.
	dbuf := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	size, err := base64.StdEncoding.Decode(dbuf, data)

	// DecodedLen(len(dst.Data)) returns the MAXIMUM possible size but
	// the real size is the one returned by the Decode method so, we need
	// to write only those bytes.
	filename = path.Join(collectInfoFileStoragePath, filename)
	err = ioutil.WriteFile(filename, dbuf[:size], os.ModePerm)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot write the file %q", filename))
	}
	return nil
}

// GET /agents/:uuid/status
func (c Agent) Status(uuid string) revel.Result {
	agentId := c.Args["agentId"].(uint)

	// Get the agent.
	comm := shared.AgentDirectory.Get(agentId)
	if comm == nil {
		return c.Error(shared.ErrAgentNotConnected, "shared.AgentDirectory.Get")
	}

	// Send it the Status cmd.
	reply, err := comm.Send(&proto.Cmd{
		Ts:        time.Now().UTC(),
		AgentUUID: uuid,
		Cmd:       "Status",
	})
	if err != nil {
		return c.Error(err, "comm.Send")
	}

	// Decode the agent's reply, which should be its status.
	status := make(map[string]string)
	if reply.Error != "" {
		// Agent should never fail to report status, so when reply.Error is set
		// its most likely because agent is remote and no longer connected so
		// really the error is from the remote API, not the agent, but there's
		// no cleaner way to handle this.
		if reply.Error == shared.ErrAgentNotConnected.Error() {
			status["agent"] = "Not connected"
		} else {
			status["agent"] = fmt.Sprintf("error: %s", err)
		}
	} else {
		// Decode the reply data which should be a status map[string]string.
		if err := json.Unmarshal(reply.Data, &status); err != nil {
			c.Response.WriteHeader(http.StatusNonAuthoritativeInfo, "")
			status["agent"] = fmt.Sprintf("Invalid reply data: %s", err)
		}
	}

	return c.RenderJSON(status)
}

// GET /agents/:uuid/logs
func (c Agent) GetLog(uuid string) revel.Result {
	agentId := c.Args["agentId"].(uint)

	var beginTs, endTs string
	c.Params.Bind(&beginTs, "begin")
	c.Params.Bind(&endTs, "end")
	begin, end, err := shared.ValidateTimeRange(beginTs, endTs)
	if err != nil {
		return c.BadRequest(err, "invalid time range")
	}

	var minLevel, maxLevel byte
	c.Params.Bind(&beginTs, "minLevel")
	c.Params.Bind(&endTs, "maxLevel")
	if minLevel == 0 {
		minLevel = agent.MIN_LOG_LEVEL
	}
	if maxLevel == 0 {
		maxLevel = agent.MAX_LOG_LEVEL
	}

	var serviceLike string
	c.Params.Bind(&beginTs, "service")

	f := agent.LogFilter{
		Begin:       begin,
		End:         end,
		MinLevel:    minLevel,
		MaxLevel:    maxLevel,
		ServiceLike: serviceLike,
	}

	// Connect to MySQL.
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "Agent.GetLog: dbm.Open")
	}
	logHandler := agent.NewLogHandler(dbm, stats.NullStats())
	logs, err := logHandler.GetLog(agentId, f)
	if err != nil {
		return c.Error(err, "logHandler.GetLog")
	}

	return c.RenderJSON(logs)
}
