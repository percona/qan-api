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

package app

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/agent"
	"github.com/percona/qan-api/app/controllers"
	agentCtrl "github.com/percona/qan-api/app/controllers/agent"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/instance"
	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/app/query"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/config"
	queryService "github.com/percona/qan-api/service/query"
	"github.com/percona/qan-api/stats"
	"github.com/revel/revel"
)

// AppVersion Do not set this var. It's set by scripts/build. The official version is set
// in conf/app.conf.
var AppVersion = ""

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// CLOUD_API_HOSTNAME is only used for testing and should override api.hostname.
	hostname := os.Getenv("CLOUD_API_HOSTNAME")
	if hostname == "" {
		hostname = config.Get("api.hostname")
		if hostname == "" {
			panic("Set CLOUD_API_HOSTNAME env var or api.hostname in the config file")
		}
	}
	log.Println("Yooohooo! Deployed!!!")
	// Use real stats clients only in stage and prod.
	statsEnv := config.Get("stats.env")
	if statsEnv == "stage" || statsEnv == "prod" {
		for _, service := range []string{"", "route"} {
			statsClient, err := statsd.NewBufferedClient(
				config.Get("statsd.server"),
				"", // prefix -- DO NOT SET HERE
				250*time.Millisecond, // buffer time
				8932, // MTU for gigabit ethernet
			)
			if err != nil {
				panic(fmt.Sprintf("ERROR: statsd.NewBufferedClient: %s", err))
			}
			s := stats.NewStats(
				statsClient,
				statsEnv,
				config.Get("api.alias"),
				service,
				config.Get("stats.rate"),
			)
			switch service {
			case "": // internal: agent, api
				shared.InternalStats = s
			case "route":
				shared.RouteStats = s
			}
		}
	}

	shared.AgentDirectory = agent.NewLocalDirectory()
	go func() {
		t := time.NewTicker(1 * time.Minute)
		for range t.C {
			shared.AgentDirectory.Refresh(20 * time.Second)
		}
	}()

	// Run the query abstracter, used to consume QAN data.
	shared.QueryAbstracter = queryService.NewMini(config.ApiRootDir + "/service/query") // tables + abstract using Perl
	go shared.QueryAbstracter.Run()

	shared.TableParser = queryService.NewMini("") // only tables
	go shared.TableParser.Run()

	revel.Filters = []revel.Filter{
		revel.PanicFilter,             // Recover from panics and display an error page instead.
		revel.RouterFilter,            // Use the routing table to select the right Action
		revel.FilterConfiguringFilter, // A hook for adding or removing per-Action filters.
		revel.ParamsFilter,            // Parse parameters into Controller.Params.
		revel.ValidationFilter,        // Restore kept validation errors and save new ones from cookie.
		revel.InterceptorFilter,       // Run interceptors around the action.
		revel.ActionInvoker,           // Invoke the action.
	}

	connsPool := models.NewConnectionsPool()

	// Tasks to be run at the begin and end of every request
	revel.InterceptFunc(beforeController(connsPool), revel.BEFORE, revel.AllControllers)
	revel.InterceptFunc(afterController, revel.FINALLY, revel.AllControllers)

	// All access to agent resources (/agents/:uuid/*) must specify a valid agent.
	revel.InterceptFunc(authAgent, revel.BEFORE, &agentCtrl.Agent{})

	revel.InterceptFunc(getInstanceID, revel.BEFORE, &controllers.QAN{})
	revel.InterceptFunc(getQueryID, revel.BEFORE, &controllers.Query{})
}

// Copied from github.com/cactus/go-statsd-client/statsd/main.go
func includeStat(rate float32) bool {
	if rate < 1 {
		if rand.Float32() < rate {
			return true
		}
		return false
	}
	return true
}

func beforeController(connsPool *models.ConnectionsPool) revel.InterceptorFunc {
	return func(c *revel.Controller) revel.Result {
		if c.Action == "Home.Options" {
			return nil
		}

		if includeStat(shared.RouteStats.SampleRate) {
			c.Args["t"] = time.Now()
		}

		if c.Action == "Home.Ping" {
			c.Response.Out.Header().Set("X-Percona-QAN-API-Version", AppVersion)
		}

		// Create a MySQL db manager for the controller because most need it, but
		// don't open the connection yet, let the controller do that when it's
		// ready because it might return early (e.g. on invalid input).
		// The controller doesn't need to close it; we do that in afterController.
		// Deprecated
		c.Args["dbm"] = db.NewMySQLManager()

		// Pass connections pool to controller
		c.Args["connsPool"] = connsPool

		// Args for various controllers/routes.
		apiBasePath := os.Getenv("BASE_PATH")
		if apiBasePath == "" {
			apiBasePath = config.Get("api.base.path")
		}
		schema := "http"
		if strings.Contains(strings.ToLower(c.Request.Request.Proto), "https") {
			schema = "https"
		}
		c.Args["wsBase"] = "ws://" + c.Request.Request.Host + apiBasePath
		c.Args["httpBase"] = schema + "://" + c.Request.Request.Host + apiBasePath

		agentVersion := c.Request.Header.Get("X-Percona-QAN-Agent-Version")
		if agentVersion == "" {
			agentVersion = "0.0.9"
		}
		c.Args["agentVersion"] = agentVersion

		// Set common headers before Revel sets the response code and writes
		// the response body. This avoids "multiple calls to WriterHeader" errors.
		c.Response.Out.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Response.Out.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,DELETE")
		c.Response.Out.Header().Set("Access-Control-Allow-Origin", "*")

		return nil
	}
}

func afterController(c *revel.Controller) revel.Result {
	if c.Action == "Home.Options" {
		return nil
	}

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Close(); err != nil {
		revel.ERROR.Println(err)
	}

	return nil
}

func authAgent(c *revel.Controller) revel.Result {
	// We don't need a valid agent UUID for these routes.
	if c.Action == "Agent.Create" || c.Action == "Agent.List" || c.Action == "Home.Options" {
		return nil
	}

	var agentUUID string
	c.Params.Bind(&agentUUID, "uuid")

	instanceMgr := models.NewInstanceManager(c.Args["connsPool"])
	agentID, agent, err := instanceMgr.Get(agentUUID)
	if agentID == 0 || agent.Subsystem != "agent" || err != nil {
		c.Response.Status = http.StatusNotFound
		return c.RenderText("Agent not found")
	}

	c.Args["agentId"] = agentID

	return nil // success
}

func getInstanceID(c *revel.Controller) revel.Result {
	// Get the internal (auto-inc) instance ID of the UUID.
	var uuid string
	c.Params.Bind(&uuid, "uuid")

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return internalError(c, "init.getInstanceId: dbm.Open", err)
	}

	instanceID, err := instance.GetInstanceId(dbm.DB(), uuid)
	if err != nil {
		switch err {
		case shared.ErrNotFound:
			c.Response.Status = http.StatusNotFound
			return c.RenderText("")
		default:
			return internalError(c, "init.getInstanceId: ih.GetInstanceId", err)
		}
	}
	c.Args["instanceId"] = instanceID

	return nil // success
}

func getQueryID(c *revel.Controller) revel.Result {
	// Get the internal (auto-inc) query ID.
	var queryID string
	c.Params.Bind(&queryID, "id")

	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return internalError(c, "init.getQueryId: dbm.Open", err)
	}

	// 92F3B1B361FB0E5B -> 5
	classID, err := query.GetClassId(dbm.DB(), queryID)
	if err != nil {
		switch err {
		case shared.ErrNotFound:
			c.Response.Status = http.StatusNotFound
			return c.RenderText("")
		default:
			return internalError(c, "init.getQueryId: query.GetClassId", err)
		}
	}
	c.Args["classId"] = classID

	return nil // success
}

func internalError(c *revel.Controller, op string, err error) revel.Result {
	errMsg := fmt.Sprintf("%s: %s", op, err)
	revel.ERROR.Printf(errMsg)
	res := proto.Error{
		Error: errMsg,
	}
	c.Response.Status = http.StatusInternalServerError
	return c.RenderJSON(res)
}
