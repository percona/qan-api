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

import (
	"reflect"

	"github.com/percona/qan-api/app/ws"
	"golang.org/x/net/websocket"
)

type WebsocketConnector struct {
	sendDataChan  chan interface{}
	recvDataChan  chan interface{}
	sendBytesChan chan []byte
	recvBytesChan chan []byte
	// --
	sendErrChan chan ws.Error
	recvErrChan chan ws.Error
	recvErr     error
	connectChan chan bool
}

func NewWebsocketConnector(sendDataChan chan interface{}, recvDataChan chan interface{}, sendBytesChan chan []byte, recvBytesChan chan []byte) *WebsocketConnector {
	w := &WebsocketConnector{
		sendDataChan:  sendDataChan,
		recvDataChan:  recvDataChan,
		sendBytesChan: sendBytesChan,
		recvBytesChan: recvBytesChan,
		// --
		sendErrChan: make(chan ws.Error),
		recvErrChan: make(chan ws.Error),
		connectChan: make(chan bool, 1),
	}
	return w
}

func (c *WebsocketConnector) Start() {
}

func (c *WebsocketConnector) Stop() {
}

func (c *WebsocketConnector) Connect() {
}

func (c *WebsocketConnector) ConnectOnce(timeout uint) error {
	return nil
}

func (c *WebsocketConnector) ConnectChan() chan bool {
	return c.connectChan
}

func (c *WebsocketConnector) Disconnect() error {
	return nil
}

func (c *WebsocketConnector) Send(data interface{}, timeout uint) error {
	// SUT: conn.Send(data) > here > test: data<-recvDataChan
	c.recvDataChan <- data
	return nil
}

func (c *WebsocketConnector) Recv(data interface{}, timeout uint) error {
	// SUT: conn.Recv(data) > here > test: sendDataChan<-data
	dataFromTest := <-c.sendDataChan
	/**
	 * Yes, we need reflection because "everything in Go is passed by value"
	 * (http://golang.org/doc/faq#Pointers).  When the caller passes a pointer
	 * to a struct (*T) as an interface{} arg, the function receives a new
	 * interface that contains a pointer to the struct.  Therefore, setting
	 * data = dataFromTest only sets the new interface, not the underlying
	 * struct.  The only way to access and change the underlying struct of an
	 * interface is with reflection.  websocket.JSON.Receive() uses reflection
	 * to solve the same problem (and because it's actually decoding raw data
	 * into the underlying struct; we're just copying data).  The nex two lines
	 * might not make any sense until you grok reflection; I leave that to you.
	 */
	if dataFromTest != nil {
		dataVal := reflect.ValueOf(data).Elem()
		dataVal.Set(reflect.ValueOf(dataFromTest).Elem())
	}
	return nil
}

func (c *WebsocketConnector) SendBytes(bytes []byte, timeout uint) error {
	// SUT: conn.SendBytes(data) > here > test: data<-recvBytesChan
	c.recvBytesChan <- bytes
	return nil
}

func (c *WebsocketConnector) RecvBytes(timeout uint) ([]byte, error) {
	// SUT: data=conn.RecvBytes() > here > test: sendBytesChan<-data
	bytes := <-c.sendBytesChan
	var err error
	if c.recvErr != nil {
		err = c.recvErr
	}
	return bytes, err
}

func (c *WebsocketConnector) SendChan() chan []byte {
	// The SUT's send chan is the test's recv chan: SUT send -> test recv
	return c.recvBytesChan
}

func (c *WebsocketConnector) RecvChan() chan []byte {
	// The SUT's recv chan is the test's send chan: SUT recv <- test send
	return c.sendBytesChan
}

func (c *WebsocketConnector) SendErrorChan() chan ws.Error {
	return c.sendErrChan
}

func (c *WebsocketConnector) RecvErrorChan() chan ws.Error {
	return c.recvErrChan
}

func (c *WebsocketConnector) Conn(conn *websocket.Conn) *websocket.Conn {
	return nil
}

func (c *WebsocketConnector) RecvError(err error) {
	c.recvErr = err
}
