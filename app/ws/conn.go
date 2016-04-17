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

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

const (
	BUFSIZE = 10
)

type Connector interface {
	Start()
	Stop()
	Connect()
	ConnectOnce(timeout uint) error
	ConnectChan() chan bool
	Disconnect() error
	Send(data interface{}, timeout uint) error
	Recv(data interface{}, timeout uint) error
	SendBytes(bytes []byte, timeout uint) error
	RecvBytes(timeout uint) ([]byte, error)
	SendChan() chan []byte
	RecvChan() chan []byte
	SendErrorChan() chan Error
	RecvErrorChan() chan Error
	Conn(conn *websocket.Conn) *websocket.Conn
}

type ConnectorFactory interface {
	Make(url, origin string) Connector
	Use(url, origin string, conn *websocket.Conn) Connector
}

type Connection struct {
	url     string
	origin  string
	headers map[string]string
	// --
	conn      *websocket.Conn
	connected bool
	mux       *sync.RWMutex // guard conn and connected
	// --
	started     bool
	recvChan    chan []byte
	sendChan    chan []byte
	connectChan chan bool
	sendErrChan chan Error
	recvErrChan chan Error
	stopChan    chan bool
}

func NewConnection(url, origin string, headers map[string]string) *Connection {
	c := &Connection{
		url:     url,
		origin:  origin,
		headers: headers,
		// --
		mux:  &sync.RWMutex{},
		conn: nil,
		// --
		recvChan:    make(chan []byte, BUFSIZE),
		sendChan:    make(chan []byte, BUFSIZE),
		connectChan: make(chan bool, 1),
		sendErrChan: make(chan Error, 1),
		recvErrChan: make(chan Error, 1),
		stopChan:    make(chan bool),
	}
	return c
}

func ExistingConnection(url, origin string, conn *websocket.Conn) *Connection {
	c := NewConnection(url, origin, nil)
	c.conn = conn
	c.connected = true
	return c
}

func (c *Connection) Start() {
	c.mux.Lock()
	defer c.mux.Unlock()
	// Start send() and recv() goroutines, but they wait for successful Connect().
	if c.started {
		return
	}
	go c.send()
	go c.recv()
	c.started = true
}

func (c *Connection) Stop() {
	c.mux.Lock()
	defer c.mux.Unlock()
	if !c.started {
		return
	}
	close(c.stopChan)
	c.started = false
}

func (c *Connection) Connect() {
	for {
		if err := c.ConnectOnce(10); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		c.notifyConnect(true)
		return // success
	}
}

func (c *Connection) ConnectOnce(timeout uint) error {
	// Make websocket connection.  If this fails, either API is down or the ws
	// address is wrong.
	config, err := websocket.NewConfig(c.url, c.origin)
	if err != nil {
		return err
	}
	if c.headers != nil {
		for k, v := range c.headers {
			config.Header.Add(k, v)
		}
	}

	conn, err := c.dialTimeout(config, timeout)
	if err != nil {
		return err
	}

	c.mux.Lock()
	defer c.mux.Unlock()
	c.conn = conn
	c.connected = true

	return nil
}

func (c *Connection) Send(data interface{}, timeout uint) error {
	c.mux.RLock()
	connected := c.connected
	c.mux.RUnlock()
	if !connected {
		return ErrNotConnected
	}
	if timeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
		defer c.conn.SetWriteDeadline(time.Time{})
	} else {
		c.conn.SetWriteDeadline(time.Time{})
	}
	return sharedError(websocket.JSON.Send(c.conn, data), "Send")
}

func (c *Connection) Recv(data interface{}, timeout uint) error {
	c.mux.RLock()
	connected := c.connected
	c.mux.RUnlock()
	if !connected {
		return ErrNotConnected
	}
	if timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
		defer c.conn.SetReadDeadline(time.Time{})
	} else {
		c.conn.SetReadDeadline(time.Time{})
	}
	return sharedError(websocket.JSON.Receive(c.conn, data), "Recv")
}

func (c *Connection) SendBytes(bytes []byte, timeout uint) error {
	c.mux.RLock()
	connected := c.connected
	c.mux.RUnlock()
	if !connected {
		return ErrNotConnected
	}
	if timeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
		defer c.conn.SetWriteDeadline(time.Time{})
	} else {
		c.conn.SetWriteDeadline(time.Time{})
	}
	return sharedError(websocket.Message.Send(c.conn, bytes), "SendBytes")
}

func (c *Connection) RecvBytes(timeout uint) ([]byte, error) {
	c.mux.RLock()
	connected := c.connected
	c.mux.RUnlock()
	if !connected {
		return nil, ErrNotConnected
	}
	if timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
		defer c.conn.SetReadDeadline(time.Time{})
	} else {
		c.conn.SetReadDeadline(time.Time{})
	}
	var bytes []byte
	err := websocket.Message.Receive(c.conn, &bytes)
	return bytes, sharedError(err, "RecvBytes")
}

func (c *Connection) ConnectChan() chan bool {
	return c.connectChan
}

func (c *Connection) Disconnect() error {
	/**
	 * Must guard c.conn here to prevent duplicate notifyConnect() because Close()
	 * causes recv() to error which calls Disconnect(), and normally we want this:
	 * to call Disconnect() on recv error so that notifyConnect(false) is called
	 * to let user know that remote end hung up.  However, when user hangs up
	 * the Disconnect() call from recv() is duplicate and not needed.
	 */
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.connected {
		return nil
	}

	// Close() causes a write, therefore it's affected by the write timeout.
	// Since Send() also sets the write timeout, we must reset it here else
	// Close() can fail immediately due to previous timeout set for Send()
	// already having passed.
	// https://jira.percona.com/browse/PCT-1045
	c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	defer c.conn.SetWriteDeadline(time.Time{})

	var err error
	if err = c.conn.Close(); err != nil {
		// Example: write tcp 127.0.0.1:8000: i/o timeout
		// That ^ can happen if remote end hangs up, then we call Close(),
		// or if there's a timeout (shouldn't happen afaik).
		// Since there's nothing we can do about errors here, we ignore them.
	}

	/**
	 * Do not set c.conn = nil to indicate that connection is closed because
	 * unless we also guard c.conn in Send() and Recv() c.conn.Set*Deadline()
	 * will panic.  If the underlying websocket.Conn is closed, then
	 * Set*Deadline() will do nothing and websocket.JSON.Send/Receive() will
	 * just return an error, which is a lot better than a panic.
	 */
	c.connected = false
	c.notifyConnect(false)
	return err
}

func (c *Connection) SendErrorChan() chan Error {
	return c.sendErrChan
}

func (c *Connection) RecvErrorChan() chan Error {
	return c.recvErrChan
}

func (c *Connection) Conn(conn *websocket.Conn) *websocket.Conn {
	if conn != nil {
		c.conn = conn
	}
	return c.conn
}

// --------------------------------------------------------------------------

func (c *Connection) dialTimeout(config *websocket.Config, timeout uint) (ws *websocket.Conn, err error) {
	// websocket.Dial() does not handle timeouts, so we use lower-level net package
	// to create connection with timeout, then create ws client with the net connection.

	if config.Location == nil {
		return nil, websocket.ErrBadWebSocketLocation
	}
	if config.Origin == nil {
		return nil, websocket.ErrBadWebSocketOrigin
	}

	var conn net.Conn
	switch config.Location.Scheme {
	case "ws":
		conn, err = net.DialTimeout("tcp", config.Location.Host, time.Duration(timeout)*time.Second)
	case "wss":
		dialer := &net.Dialer{
			Timeout: time.Duration(timeout) * time.Second,
		}
		if config.Location.Host == "localhost:8443" {
			// Test uses mock ws server which uses self-signed cert which causes Go to throw
			// an error like "x509: certificate signed by unknown authority".  This disables
			// the cert verification for testing.
			config.TlsConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", config.Location.Host, config.TlsConfig)
	default:
		err = websocket.ErrBadScheme
	}
	if err != nil {
		return nil, &websocket.DialError{config, err}
	}

	ws, err = websocket.NewClient(config, conn)
	if err != nil {
		return nil, err
	}

	return ws, nil
}

func (c *Connection) send() {
	defer c.Disconnect()
	for {
		select {
		case bytes := <-c.sendChan:
			if err := c.SendBytes(bytes, 5); err != nil {
				select {
				case c.sendErrChan <- Error{bytes, err}:
				default:
				}
				return
			}
		case <-c.stopChan:
			return
		}
	}
}

func (c *Connection) recv() {
	defer c.Disconnect()
	for {
		// Before blocking on Recv, see if we're supposed to stop.
		select {
		case <-c.stopChan:
			return
		default:
		}

		bytes, err := c.RecvBytes(0)
		if err != nil {
			select {
			case c.recvErrChan <- Error{bytes, err}:
			default:
			}
			return
		}

		select {
		case c.recvChan <- bytes:
		case <-c.stopChan:
		}
	}
}

func (c *Connection) SendChan() chan []byte {
	return c.sendChan
}

func (c *Connection) RecvChan() chan []byte {
	return c.recvChan
}

func (c *Connection) notifyConnect(state bool) {
	select {
	case c.connectChan <- state:
	case <-time.After(5 * time.Second):
	}
}
