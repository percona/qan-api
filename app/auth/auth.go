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

// deprecated
package auth

import (
	"net/http"
	"time"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/stats"
)

type AuthHandler interface {
	Agent(agentUuid string) (uint, *proto.AuthResponse, error)
}

type DbHandler interface {
	GetAgentId(agentId string) (uint, error)
}

// --------------------------------------------------------------------------

type AuthDb struct {
	dbh   DbHandler
	stats stats.Stats
}

func NewAuthDb(dbh DbHandler) *AuthDb {
	a := &AuthDb{
		dbh:   dbh,
		stats: shared.InternalStats, // copy
	}
	return a
}

// Agent exists and is active?
func (a *AuthDb) Agent(agentUuid string) (uint, *proto.AuthResponse, error) {
	t := time.Now()
	defer func() {
		a.stats.TimingDuration(a.stats.Metric("auth.agent.t"), time.Now().Sub(t), a.stats.SampleRate)
	}()

	res := &proto.AuthResponse{Code: http.StatusOK}

	// agentId and err are mutually exclusive: if auth is ok, agentId is non-zero
	// and err is nil, else agentId is zero and err is not nil.
	agentId, err := a.dbh.GetAgentId(agentUuid)
	if err != nil {
		switch err {
		case shared.ErrNotFound:
			a.stats.Inc(a.stats.Metric("auth.agent.not-found"), 1, 1)
			res.Code = http.StatusNotFound
			res.Error = "Agent not found"
		default:
			a.stats.Inc(a.stats.Metric("auth.agent.err"), 1, 1)
			res.Code = http.StatusInternalServerError
			res.Error = "Internal error"
		}
		return 0, res, err
	}

	// Auth OK
	a.stats.Inc(a.stats.Metric("auth.agent.ok"), 1, a.stats.SampleRate)
	return agentId, res, nil
}
