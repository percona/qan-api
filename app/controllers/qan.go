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
	"encoding/base64"
	"fmt"
	"time"

	qp "github.com/percona/pmm/proto/qan"
	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/app/shared"
	"github.com/revel/revel"
)

// QueryReport returns report of query classes
type QueryReport struct {
	InstanceId string                  // UUID of MySQL instance
	Begin      time.Time               // time range [Begin, End)
	End        time.Time               // time range [Being, End)
	Query      models.Query            // id, abstract, fingerprint, etc.
	Metrics    map[string]models.Stats // keyed on metric name, e.g. Query_time
	Example    *models.Example         // query example
	Sparks     []interface{}           `json:",omitempty"`
	Metrics2   interface{}             `json:",omitempty"`
	Sparks2    interface{}             `json:",omitempty"`
}

// QAN is base for query analytics related endpoints.
type QAN struct {
	BackEnd
}

// Profile is endpoint to get query analitics for given instance.
// TODO: looks like UUID is not used
func (c QAN) Profile(UUID string) revel.Result {
	instanceID := c.Args["instanceId"].(uint)

	// Convert and validate the time range.
	var beginTs, endTs, search, searchB64 string
	var offset int
	c.Params.Bind(&beginTs, "begin")
	c.Params.Bind(&endTs, "end")
	c.Params.Bind(&searchB64, "search")
	c.Params.Bind(&offset, "offset")
	searchB, err := base64.StdEncoding.DecodeString(searchB64)
	if err != nil {
		fmt.Println("error decoding base64 search :", err)
	}
	search = string(searchB)

	begin, end, err := shared.ValidateTimeRange(beginTs, endTs)
	if err != nil {
		return c.BadRequest(err, "invalid time range")
	}

	// todo: let caller specify rank by args via URL params
	r := models.RankBy{
		Metric: "Query_time",
		Stat:   "sum",
		Limit:  10,
	}

	// qh := qan.NewReporter(dbm, stats.NullStats())
	// profile, err := qh.Profile(instanceID, begin, end, r, offset, search)

	queryReportMgr := models.NewQueryReportManager(c.Args["connsPool"])
	profile, err := queryReportMgr.Profile(instanceID, begin, end, r, offset, search)
	if err != nil {
		return c.Error(err, "qh.Profile")
	}

	return c.RenderJSON(profile)
}

// QueryReport is endpoint to get metrics for given instance and query class
func (c QAN) QueryReport(UUID, queryID string) revel.Result {
	instanceID := c.Args["instanceId"].(uint)

	// Convert and validate the time range.
	var beginTs, endTs string
	c.Params.Bind(&beginTs, "begin")
	c.Params.Bind(&endTs, "end")

	begin, end, err := shared.ValidateTimeRange(beginTs, endTs)
	if err != nil {
		return c.BadRequest(err, "invalid time range")
	}

	// Get the full query info: abstract, example, first/laset seen, etc.
	queryMgr := models.NewQueryManager(c.Args["connsPool"])
	queries, err := queryMgr.Get([]string{queryID})

	if err != nil {
		return c.Error(err, "qh.Get")
	}
	q, ok := queries[queryID]
	if !ok {
		return c.Error(shared.ErrNotFound, "QAN.QueryReport")
	}

	// Convert query ID to class ID so we can pull data from other tables.
	classID, err := queryMgr.GetClassID(queryID)

	if err != nil {
		return c.Error(err, "qh.GetQueryId")
	}

	s, err := queryMgr.Example(classID, instanceID, end)
	if err != nil && err != shared.ErrNotFound {
		return c.Error(err, "qh.Example")
	}

	// Init the report. This info is a little redundant because the caller
	// already knows what query and time range it requested, but it makes
	// the report stateless in case the caller passes the data to other code.
	report := QueryReport{
		InstanceId: UUID,
		Begin:      begin,
		End:        end,
		Query:      q,
		Example:    s,
	}

	metricsMgr := models.NewMetricsManager(c.Args["connsPool"])
	metrics2, sparks2 := metricsMgr.GetClassMetrics(classID, instanceID, begin, end)
	report.Metrics2 = metrics2
	report.Sparks2 = sparks2

	return c.RenderJSON(report)
}

// ServerSummary is endpoint to get metrics for given instance over all query classes
func (c QAN) ServerSummary(UUID string) revel.Result {
	instanceID := c.Args["instanceId"].(uint)

	// Convert and validate the time range.
	var beginTs, endTs string
	c.Params.Bind(&beginTs, "begin")
	c.Params.Bind(&endTs, "end")

	begin, end, err := shared.ValidateTimeRange(beginTs, endTs)
	if err != nil {
		return c.BadRequest(err, "invalid time range")
	}

	// Init the report. This info is a little redundant because the caller
	// already knows what query and time range it requested, but it makes
	// the report stateless in case the caller passes the data to other code.
	summary := qp.Summary{
		InstanceId: UUID,
		Begin:      begin,
		End:        end,
	}

	metricsMgr := models.NewMetricsManager(c.Args["connsPool"])
	metrics2, sparks2 := metricsMgr.GetGlobalMetrics(instanceID, begin, end)
	summary.Metrics2 = metrics2
	summary.Sparks2 = sparks2

	return c.RenderJSON(summary)
}

// Config is endpoint to get configuration of query analytics
// TODO: looks like UUID is not used
func (c QAN) Config(UUID string) revel.Result {
	instanceID := c.Args["instanceId"].(uint)

	agentConfigMgr := models.NewAgentConfigManager(c.Args["connsPool"])
	configs, err := agentConfigMgr.GetQAN(instanceID)
	if err != nil {
		return c.Error(err, "config.MySQLHandler.GetQAN")
	}
	if len(configs) == 0 {
		return c.NotFound("")
	}
	if len(configs) > 1 {
		return c.Error(fmt.Errorf("got %d QAN configs, expected 1", len(configs)), "QAN.Config")
	}
	return c.RenderJSON(configs[0])
}
