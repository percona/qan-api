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
	"log"
	"net/http"

	"golang.org/x/net/websocket"
)

type WebsocketClient struct {
	sendChan chan interface{}
	recvChan chan []byte
	// --
	SendErr   error
	RecvErr   error
	closeChan chan bool
	// --
	conn *websocket.Conn
}

func NewWebsocketClient(sendChan chan interface{}, recvChan chan []byte) *WebsocketClient {
	return &WebsocketClient{
		sendChan:  sendChan,
		recvChan:  recvChan,
		closeChan: make(chan bool, 2),
	}
}

func (w *WebsocketClient) Connect(url string, origin string, header http.Header) error {
	config, err := websocket.NewConfig(url, origin)
	config.Header = header
	if err != nil {
		return err
	}

	// Make websocket connection.
	// If this fails, either the other end is down or the ws address is wrong.
	conn, err := websocket.DialConfig(config)
	if err != nil {
		return err
	}
	w.conn = conn
	w.start()
	return nil
}

func (w *WebsocketClient) Close() {
	w.conn.Close()
}

func (w *WebsocketClient) start() {
	go func() {
		defer func() { w.closeChan <- true }()
		for data := range w.sendChan {
			err := websocket.JSON.Send(w.conn, data)
			if err != nil {
				log.Printf("ERROR: websocket.JSON.Send: %s\n", err)
				w.SendErr = err
				return
			}
		}
	}()

	go func() {
		defer func() { w.closeChan <- true }()
		for {
			var data []byte
			if err := websocket.Message.Receive(w.conn, &data); err != nil {
				// It seems like EOF is only error that happens here.
				w.RecvErr = err
				return
			}
			w.recvChan <- data
		}
	}()
}

func (w *WebsocketClient) SendChan() chan interface{} {
	return w.sendChan
}

func (w *WebsocketClient) RecvChan() chan []byte {
	return w.recvChan
}

func (w *WebsocketClient) CloseChan() chan bool {
	return w.closeChan
}
