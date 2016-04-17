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

package mock

type Multiplexer struct {
	sendChan chan interface{}
	recvChan chan interface{}
	// --
	doneChan chan bool
}

func NewMultiplexer(sendChan chan interface{}, recvChan chan interface{}) *Multiplexer {
	m := &Multiplexer{
		sendChan: sendChan,
		recvChan: recvChan,
		// --
		doneChan: make(chan bool),
	}
	return m
}

func (m *Multiplexer) Start() error {
	return nil
}

func (m *Multiplexer) Stop() {
}

func (m *Multiplexer) Done() chan bool {
	return m.doneChan
}

func (m *Multiplexer) Send(msg interface{}) (interface{}, error) {
	// SUT: comm.Send(msg) > here > test: msg<-recvChan > test: sendChan<-msg > here > SUT
	m.recvChan <- msg
	v := <-m.sendChan
	return v, nil
}
