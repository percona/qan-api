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

package ws_test

import (
	"github.com/percona/qan-api/app/ws"
	"github.com/percona/qan-api/test/mock"
	. "gopkg.in/check.v1"
)

type MxTestSuite struct {
	sendDataChan  chan interface{}
	recvDataChan  chan interface{}
	sendBytesChan chan []byte
	recvBytesChan chan []byte
	gotData       interface{}
	expectData    interface{}
	gotBytes      []byte
	expectBytes   []byte
	expectId      string
	expectErr     error
	mockConn      *mock.WebsocketConnector
	mockCommProc  *mock.CommProcessor
	mx            *ws.ConcurrentMultiplexer
}

var _ = Suite(&MxTestSuite{})

func (s *MxTestSuite) SetUpSuite(t *C) {
	s.sendDataChan = make(chan interface{}, 1)
	s.recvDataChan = make(chan interface{}, 1)
	s.sendBytesChan = make(chan []byte, 1)
	s.recvBytesChan = make(chan []byte, 1)
}

func (s *MxTestSuite) SetUpTest(t *C) {
	s.mockConn = mock.NewWebsocketConnector(s.sendDataChan, s.recvDataChan, s.sendBytesChan, s.recvBytesChan)
	s.gotData = nil
	s.expectData = nil
	s.gotBytes = []byte("gotBytes not set")
	s.expectBytes = []byte("gotData not set")
	s.expectId = "not set"
	s.expectErr = nil
	s.mockCommProc = &mock.CommProcessor{
		BeforeSendFunc: func(data interface{}) (id string, bytes []byte, err error) {
			s.gotData = data
			return s.expectId, s.expectBytes, s.expectErr
		},
		AfterRecvFunc: func(bytes []byte) (id string, data interface{}, err error) {
			s.gotBytes = bytes
			return s.expectId, s.expectData, s.expectErr
		},
	}
	s.mx = ws.NewConcurrentMultiplexer("test-mx", s.mockConn, s.mockCommProc, 0)
}

func (s *MxTestSuite) TearDownTest(t *C) {
	s.mx.Stop() // stop its goroutines else they'll leak
}

// --------------------------------------------------------------------------

func (s *MxTestSuite) TestSendLocal(t *C) {
	_, err := s.mx.Send(nil)
	t.Check(err, Equals, ws.ErrNotRunning)

	err = s.mx.Start()
	t.Assert(err, IsNil)

	s.expectId = "13579"
	s.expectData = "hello"
	s.expectBytes = []byte("world")

	go func() {
		<-s.recvBytesChan                // 2. recv req
		s.sendBytesChan <- s.expectBytes // 3. send resp
	}()

	v, err := s.mx.Send(s.expectData) // 1. send req
	t.Check(err, IsNil)
	t.Assert(v, NotNil)

	t.Check(s.gotData, Equals, s.expectData)
	t.Check(string(s.gotBytes), DeepEquals, string(s.expectBytes))
}

func (s *MxTestSuite) TestSendRemote(t *C) {
	_, err := s.mx.Send(nil)
	t.Check(err, Equals, ws.ErrNotRunning)

	err = s.mx.Start()
	t.Assert(err, IsNil)

	s.expectId = "13579"
	s.expectData = "hello"
	s.expectBytes = []byte("world")
	s.sendBytesChan <- s.expectBytes

	s.gotData = <-s.recvDataChan
	t.Check(string(s.gotBytes), Equals, string(s.expectBytes))
	t.Check(s.gotData, Equals, s.expectData)
}
