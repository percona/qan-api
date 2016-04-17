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
	"errors"
	"io"
	"log"
	"sync"
	"time"
)

type ConcurrentMultiplexer struct {
	name     string
	conn     Connector // agent or remote API
	commProc Processor
	concur   uint
	// --
	clients         map[string]chan *response // users and remote APIs
	clientsMux      *sync.Mutex
	fromLocalClient chan *request  // in from local clients
	toRemoteClient  chan *response // out to remote agent or API
	recvBytes       chan []byte
	stopChan        chan bool
	doneChan        chan bool
	running         bool
	runningMux      *sync.RWMutex
}

type request struct {
	id       string
	data     interface{}
	respChan chan *response
	ts       time.Time
}

type response struct {
	id        string
	data      interface{}
	err       error
	errString string
}

func NewConcurrentMultiplexer(name string, conn Connector, commProc Processor, concurrency uint) *ConcurrentMultiplexer {
	if concurrency == 0 {
		concurrency = 1
	}
	a := &ConcurrentMultiplexer{
		name:     name,
		conn:     conn,
		commProc: commProc,
		concur:   concurrency,
		// --
		clients:         make(map[string]chan *response),
		clientsMux:      &sync.Mutex{},
		fromLocalClient: make(chan *request, BUFSIZE),
		toRemoteClient:  make(chan *response, BUFSIZE),
		recvBytes:       make(chan []byte, BUFSIZE),
		stopChan:        make(chan bool),
		doneChan:        make(chan bool, 1),
		runningMux:      &sync.RWMutex{},
	}
	return a
}

func (m *ConcurrentMultiplexer) Start() error {
	m.runningMux.Lock()
	defer m.runningMux.Unlock()
	if m.running {
		return nil
	}
	m.conn.Start()
	for i := uint(0); i < m.concur; i++ {
		go m.sendResponse()
	}
	go m.recv()
	go m.send()
	m.running = true
	return nil
}

func (m *ConcurrentMultiplexer) Stop() {
	m.stop()
}

func (m *ConcurrentMultiplexer) Done() chan bool {
	return m.doneChan
}

func (m *ConcurrentMultiplexer) Send(data interface{}) (interface{}, error) {
	m.runningMux.RLock()
	running := m.running
	m.runningMux.RUnlock()
	if !running {
		return nil, ErrNotRunning
	}
	respChan := make(chan *response, 1) // must buffer so sendResponse() doesn't block
	req := &request{data: data, respChan: respChan, ts: time.Now()}

	select {
	case m.fromLocalClient <- req:
	case <-time.After(2 * time.Second):
		return nil, ErrSendTimeout
	}

	select {
	case res := <-respChan:
		return res.data, res.err
	case <-time.After(20 * time.Second):
		return nil, ErrRecvTimeout
	}

	return nil, errors.New("reached end of mx.Send")
}

// --------------------------------------------------------------------------

func (m *ConcurrentMultiplexer) send() {
	defer m.stop()
	for {
		select {
		case req := <-m.fromLocalClient: // in from Send()
			// Process the data just before sending it. The comm proc should
			// return an ID for it so we can route the server's response back
			// to the client.
			id, bytes, err := m.commProc.BeforeSend(req.data)
			if err != nil {
				// todo: handle better
				log.Printf(m.name+" ERROR: ws.ConcurrentMultiplexer.send: m.commProc.BeforeSend: %s\n", err)
				continue
			}

			// Save the request ID so when we receive a response with the same
			// ID we know which local client to return it to.
			m.clientsMux.Lock()
			_, ok := m.clients[id]
			if ok {
				// todo: handle better
				log.Printf(m.name+" ERROR: ws.ConcurrentMultiplexer.send: duplicate id: %s\n", id)
				m.clientsMux.Unlock()
				continue
			}
			m.clients[id] = req.respChan
			m.clientsMux.Unlock()
			req.id = id

			// Send the client's request to the server. If this fails, the error
			// will be sent via SendErrorChan().
			select {
			case m.conn.SendChan() <- bytes: // local Send() -> local agent or remote API
			default:
				// chan full
			}
		case err := <-m.conn.SendErrorChan(): // err from websocket on send
			switch err.Error {
			case io.EOF:
			default:
				log.Printf(m.name+" WARN: ws.ConcurrentMultiplexer.send: %s\n", err.Error)
			}
			return
		case res := <-m.toRemoteClient: // in from sendResponse()
			if res.err != nil {
				// json pkg can't marshal type error.
				res.errString = res.err.Error()
			}
			// remote Send() -> local sendResponse() -> remote API
			if err := m.conn.Send(res.data, 5); err != nil {
				log.Printf(m.name+" WARN: m.conn.Send: %s\n", err)
				return
			}
		case connected := <-m.conn.ConnectChan():
			if !connected {
				return
			}
		case <-m.stopChan:
			return
		}
	}
}

func (m *ConcurrentMultiplexer) recv() {
	defer m.stop()
	for {
		select {
		case bytes := <-m.conn.RecvChan(): // from server
			// todo: handle timeout
			m.recvBytes <- bytes
		case err := <-m.conn.RecvErrorChan(): // err from websocket on recv
			switch err.Error {
			case io.EOF:
			default:
				log.Printf(m.name+" WARN: ws.ConcurrentMultiplexer.recv: %s\n", err.Error)
			}
			return
		case connected := <-m.conn.ConnectChan():
			if !connected {
				return
			}
		case <-m.stopChan:
			return
		}
	}
}

func (m *ConcurrentMultiplexer) sendResponse() {
	for {
		select {
		case bytes := <-m.recvBytes:
			// Process the response just before returning it to the client. This
			// also lets the comm proc tell us the ID of the original request so
			// we can check if it originated from a local client (via a call to
			// Send()) or via a remote client via the full-duplex websocket.
			id, data, err := m.commProc.AfterRecv(bytes)
			res := &response{id: id, data: data, err: err}

			m.clientsMux.Lock()
			toLocalClient, local := m.clients[id]
			delete(m.clients, id) // null op if not local
			m.clientsMux.Unlock()

			if local {
				select {
				case toLocalClient <- res: // respChan <- res
				default:
					log.Println(m.name + " ERROR: local sender respChan not buffered, response dropped")
				}
			} else if id != "" {
				m.toRemoteClient <- res
			} else if err != nil {
				log.Printf(m.name+" ERROR: commProc.AfterRecv: %s\n", err)
			}
			// If no id and no err, the comm processor wants to ignore the response.
		case <-m.stopChan:
			return
		}
	}
}

func (m *ConcurrentMultiplexer) stop() {
	m.runningMux.Lock()
	defer m.runningMux.Unlock()
	if !m.running {
		return
	}

	// Stop the ws connection async chans.
	m.conn.Stop()

	// Stop the send() and recv() goroutines.
	close(m.stopChan)

	// Notify all clients that the agent is no longer available.
	m.clientsMux.Lock()
	defer m.clientsMux.Unlock()
	for c, _ := range m.clients {
		m.clients[c] <- &response{err: ErrStopped}
		delete(m.clients, c)
	}

	// Notify agent comm controller that we're done.
	select {
	case m.doneChan <- true:
	default:
	}

	m.running = false
}
