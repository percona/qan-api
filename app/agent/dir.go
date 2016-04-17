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
	"log"
	"sync"
	"time"

	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/stats"
)

type agentInfo struct {
	comm shared.AgentCommunicator
	ts   time.Time
}

type LocalDirectory struct {
	agents map[uint]*agentInfo
	mux    *sync.RWMutex
	stats  stats.Stats // SEE NOTE BELOW
}

// When using the d.stats copy, only use stats.Metric(), do not use any of the
// stats.Stats helper funcs like System() or Org(). shared.InternalStats is
// shared only for its underlying statsd client and root prefix (e.g.
// prod.api.api02), so each component must specify fully-qualified metric names,
// e.g. for this component: agent.{metric}

func NewLocalDirectory() *LocalDirectory {
	d := &LocalDirectory{
		// --
		agents: make(map[uint]*agentInfo),
		mux:    &sync.RWMutex{},
		stats:  shared.InternalStats, // copy
	}
	// Must reset gauges at start else we'll start at the last value.
	d.stats.Gauge(d.stats.Metric("agent.comm.connected"), 0, 1)
	return d
}

// Used by agent comm controller to register local agent
func (d *LocalDirectory) Add(agentId uint, comm shared.AgentCommunicator) error {
	d.mux.Lock()
	defer d.mux.Unlock()
	defer d.updateConnected()

	// There's always a race condition we can't prevent: agent connects but
	// while we're registering it, it disconnects and reconnects to another API.
	a := d.agents[agentId]
	if a != nil {
		// Supposedly the agent is already connected to this API. If true and
		// it's alive on the existing comm, then somehow the same agent is
		// trying to connect twice to this API. Maybe the user clone the server
		// and forget to install a new agent?
		if a.comm.IsAlive() {
			d.stats.Inc(d.stats.Metric("agent.comm.dupe"), 1, 1)
			return shared.ErrDuplicateAgent
		}

		// The agent was connected to this API, but it died. This can happen
		// due to network failures and Fernando Ipar. It's ok, just stop the
		// old comm and re-register the new one.
		a.comm.Stop() // stop old comm
		a.comm = comm // register new comm
	} else {
		// First time we've seen this agent, so register it.
		a = &agentInfo{
			comm: comm,
		}
		d.agents[agentId] = a
	}
	a.ts = time.Now()
	return nil
}

func (d *LocalDirectory) Get(agentId uint) shared.AgentCommunicator {
	d.mux.RLock()
	defer d.mux.RUnlock()
	a := d.agents[agentId]
	if a == nil {
		d.stats.Inc(d.stats.Metric("agent.dir.get-miss"), 1, d.stats.SampleRate)
		return nil
	}
	d.stats.Inc(d.stats.Metric("agent.dir.get-hit"), 1, d.stats.SampleRate)
	return a.comm
}

// Used by agent comm controller to unregister local agent.
func (d *LocalDirectory) Remove(agentId uint) {
	d.mux.Lock()
	defer d.mux.Unlock()
	defer d.updateConnected()
	delete(d.agents, agentId)
}

func (d *LocalDirectory) Refresh(timeLimit time.Duration) {
	// Hold a read lock, not a write lock, while doing this so we don't block
	// the rest of the API. If there are dead agents, we'll get a write lock
	// and remove them at the end.
	d.mux.RLock()
	have := len(d.agents)
	done := 0
	remove := []uint{}
	begin := time.Now()
	for agentId, a := range d.agents {
		done++
		if !a.comm.IsAlive() {
			a.comm.Stop()
			remove = append(remove, agentId)
		}
		if time.Now().Sub(begin) >= timeLimit {
			log.Printf("NOTICE: timeout refreshing agent directory, checked %d of %d\n", done, have)
			break
		}
	}
	d.mux.RUnlock() // XXX not defered, be sure not to exit in loop ^

	if len(remove) > 0 {
		d.mux.Lock()
		defer d.mux.Unlock()
		for _, agentId := range remove {
			log.Printf("NOTICE: removing dead agent_id=%d\n", agentId)
			delete(d.agents, agentId)
		}
		d.updateConnected()
	}
}

func (d *LocalDirectory) updateConnected() {
	d.stats.Gauge(d.stats.Metric("agent.comm.connected"), int64(len(d.agents)), 1)
}
