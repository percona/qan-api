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

	qp "github.com/percona/pmm/proto/qan"
	"github.com/percona/qan-api/app/config"
	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/models"
	"github.com/percona/qan-api/app/qan"
	"github.com/percona/qan-api/app/query"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/stats"
	"github.com/revel/revel"
)

type QAN struct {
	BackEnd
}

func (c QAN) Profile(uuid string) revel.Result {
	instanceId := c.Args["instanceId"].(uint)

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
	r := qp.RankBy{
		Metric: "Query_time",
		Stat:   "sum",
		Limit:  10,
	}

	// Get the server profile, aka query rank.
	dbm := c.Args["dbm"].(db.Manager)
	if err := dbm.Open(); err != nil {
		return c.Error(err, "QAN.Profile: dbm.Open")
	}
	qh := qan.NewReporter(dbm, stats.NullStats())
	profile, err := qh.Profile(instanceId, begin, end, r, offset, search)
	if err != nil {
		return c.Error(err, "qh.Profile")
	}

	return c.RenderJSON(profile)
}

func (c QAN) QueryReport(uuid, queryId string) revel.Result {
	instanceId := c.Args["instanceId"].(uint)

	// Convert and validate the time range.
	var beginTs, endTs string
	c.Params.Bind(&beginTs, "begin")
	c.Params.Bind(&endTs, "end")

	begin, end, err := shared.ValidateTimeRange(beginTs, endTs)
	if err != nil {
		return c.BadRequest(err, "invalid time range")
	}

	// Get the full query info: abstract, example, first/laset seen, etc.
	dbm := c.Args["dbm"].(db.Manager)
	qh := query.NewMySQLHandler(dbm, stats.NullStats())
	queries, err := qh.Get([]string{queryId})
	if err != nil {
		return c.Error(err, "qh.Get")
	}
	q, ok := queries[queryId]
	if !ok {
		return c.Error(shared.ErrNotFound, "QAN.QueryReport")
	}

	// Convert query ID to class ID so we can pull data from other tables.
	classId, err := query.GetClassId(dbm.DB(), queryId)
	if err != nil {
		return c.Error(err, "qh.GetQueryId")
	}

	s, err := qh.Example(classId, instanceId, end)
	if err != nil && err != shared.ErrNotFound {
		return c.Error(err, "qh.Example")
	}

	// Init the report. This info is a little redundant because the caller
	// already knows what query and time range it requested, but it makes
	// the report stateless in case the caller passes the data to other code.
	report := qp.QueryReport{
		InstanceId: uuid,
		Begin:      begin,
		End:        end,
		Query:      q,
		Example:    s,
	}

	metrics2, sparks2 := models.Metrics.GetClassMetrics(classId, instanceId, begin, end)
	report.Metrics2 = metrics2
	report.Sparks2 = sparks2

	return c.RenderJSON(report)
}

func (c QAN) ServerSummary(uuid string) revel.Result {
	instanceId := c.Args["instanceId"].(uint)

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
		InstanceId: uuid,
		Begin:      begin,
		End:        end,
	}

	metrics2, sparks2 := models.Metrics.GetGlobalMetrics(instanceId, begin, end)
	summary.Metrics2 = metrics2
	summary.Sparks2 = sparks2

	return c.RenderJSON(summary)
}

func (c QAN) Config(uuid string) revel.Result {
	instanceId := c.Args["instanceId"].(uint)
	dbm := c.Args["dbm"].(db.Manager)
	ch := config.NewMySQLHandler(dbm, stats.NullStats())
	configs, err := ch.GetQAN(instanceId)
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
