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

package ws

type Multiplexer interface {
	Start() error
	Stop()
	Done() chan bool
	Send(msg interface{}) (interface{}, error)
}

// A Processor lets the API see and react to messages as they're sent and
// received. For agent communicators, we update oN tables for certain commands.
// For example, when a tool is stopped, we set oN.agent_config.running=false.
// For API communicators, we handle requests from the remote API. For example,
// a remote API routes an agent status request to us because we have the agent.
// A comm processor is also responsible for uniquely identifying messages and
// serializing them to []byte for sending and receiving.
type Processor interface {
	BeforeSend(interface{}) (id string, bytes []byte, err error)
	AfterRecv([]byte) (id string, data interface{}, err error)
	Timeout(id string)
}

type ProcessorFactory interface {
	Make() Processor
}
