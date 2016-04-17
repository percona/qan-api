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

package stats

import (
	"fmt"
	"strconv"

	"github.com/cactus/go-statsd-client/statsd"
)

type Stats struct {
	statsd.Statter
	env        string
	service    string
	SampleRate float32
	// --
	systemPrefix string
	agentPrefix  string
	component    string
}

func NewStats(client statsd.Statter, env, server, service string, sampleRate string) Stats {
	// Return a structure not a poniter/ref so the caller can copy it if there
	// are multiple components. However, the caller need to pass &Stats to
	// instrumented code so SetComponent() works. See bin/mm-consumer/main.go.
	var rate float32
	if sampleRate != "" {
		rate64, _ := strconv.ParseFloat(sampleRate, 32)
		rate = float32(rate64)
	}
	systemPrefix := env + ".api." + server
	if service != "" {
		systemPrefix += "." + service
	}
	s := Stats{
		client,
		env,
		service,
		rate,
		// --
		systemPrefix,
		"", // agentPrefix
		"", // component
	}
	return s
}

func (s *Stats) SetComponent(component string) {
	s.component = component
}

func (s *Stats) SetAgent(agentId uint) {
	s.agentPrefix = fmt.Sprintf("%s.agent.%d.%s", s.env, agentId, s.service)
}

func (s *Stats) System(stat string) string {
	return s.systemPrefix + "." + s.component + "." + stat
}

func (s *Stats) Agent(stat string) string {
	return s.agentPrefix + "." + s.component + "." + stat
}

func (s Stats) Metric(metric string) string {
	return s.systemPrefix + "." + metric
}

func NullStats() *Stats {
	stats := NewStats(&statsd.NoopClient{}, "", "", "", "0")
	return &stats
}
