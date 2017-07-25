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

package api

/**
 * Utilities to start/stop API (revel) during tests.
 */

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/test/mock"
	"golang.org/x/net/websocket"
)

var ErrStartFailed = errors.New("failed to start API")

type Api struct {
	host       string
	port       string
	api        *exec.Cmd
	httpClient *http.Client
	wsClients  []*mock.WebsocketClient
}

type Response struct {
	Resp    *http.Response
	Content []byte
	Err     error
}

func NewApi(host string, port string) *Api {
	return &Api{
		host: host,
		port: port,
		httpClient: &http.Client{
			// @todo it's not obvious if this will work for subsequent requests
			// or I should create new http.Client{} for every request
			Timeout: 1 * time.Second,
		},
	}
}

func (a *Api) Start() (err error) {
	hostname := a.host + ":" + a.port
	a.api = exec.Command("revel", "run", "github.com/percona/qan-api", "test", a.port)
	a.api.Env = os.Environ()
	a.api.Env = append(a.api.Env, "CLOUD_API_HOSTNAME="+hostname)
	stdout, err := a.api.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := a.api.StderrPipe()
	if err != nil {
		return err
	}
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)
	err = a.api.Start()
	if err != nil {
		return err
	}

	connectTimeout := time.After(3 * time.Second)
	for {
		select {
		case <-connectTimeout:
			return ErrStartFailed
		default:
		}
		log.Printf("Testing websocket connection to API %s:%s...", a.host, a.port)
		conn, err := websocket.Dial("ws://"+hostname+"/", "", "http://"+a.host)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func (a *Api) Stop() {
	// Don't try to run here `a.api.Process.Kill()`,
	// it terminates process immediately without notifying it.
	// Revel dies and can't cleanup after itself so we are left
	// with datastore processes (`ps -A | grep percona-datastore`) still running
	syscall.Kill(a.api.Process.Pid, syscall.SIGINT)

	a.DisconnectAllClients()
}

func (a *Api) DisconnectAllClients() {
	for _, wsClient := range a.wsClients {
		wsClient.Close()
	}
	a.wsClients = nil
}

func (a *Api) GetHost() string {
	return a.host
}

func (a *Api) GetPort() string {
	return a.port
}

func (a *Api) Connect(path string, header http.Header) (ws *mock.WebsocketClient) {
	sendChan := make(chan interface{}, 3)
	recvChan := make(chan []byte, 3)
	ws = mock.NewWebsocketClient(sendChan, recvChan)

	if header == nil {
		header = http.Header{}
		header.Set("X-Percona-API-Key", "00000000000000000000000000000001")
	}

	// Connect to API
	if err := ws.Connect("ws://"+a.getHostname()+path, "http://"+a.getHostname(), header); err != nil {
		log.Println(err)
	}

	// Register wsClient so we can close it automatically in TearDownTest
	a.wsClients = append(a.wsClients, ws)

	return ws
}

func (a *Api) ConnectAgent(path string, header http.Header, configs ...proto.AgentConfig) (ws *mock.WebsocketClient) {
	ws = a.Connect(path, header)

	// Respond to gatekeeper cmd with empty set of configs to be updated
	// This way we still interact with gatekeeper in tests without actually updating anything
	// You can also pass in parameter list of configs for update - useful if you test gatekeeper itself
	cmd := proto.Cmd{}
	select {
	case data := <-ws.RecvChan():
		err := json.Unmarshal(data, &cmd)
		if err != nil {
			panic("Unable to unmarshal gatekeeper cmd")
		}
		// Version.Running must be 0.0.9 else app/agent/comm.go will send back
		// a Stop cmd because we only support >= 1.0.11, but 0.0.9 is a special
		// value for testing.
		reply := cmd.Reply(&proto.Version{Running: "0.0.9"})
		ws.SendChan() <- reply
	case <-ws.CloseChan():
		// Connection was closed
		ws.CloseChan() <- true
	case <-time.After(1 * time.Second):
		panic("Timeout. Didn't get cmd from gatekeeper")
	}

	select {
	case data := <-ws.RecvChan():
		err := json.Unmarshal(data, &cmd)
		if err != nil {
			panic("Unable to unmarshal gatekeeper cmd")
		}
		reply := cmd.Reply(configs)
		ws.SendChan() <- reply
	case <-ws.CloseChan():
		// Connection was closed
		ws.CloseChan() <- true
	case <-time.After(1 * time.Second):
		panic("Timeout. Didn't get cmd from gatekeeper")
	}

	// @todo better detection if agent was registered instead of just waiting 200 miliseconds
	// I run into interesting problem.
	// Even though we start ws connection then it doesn't mean code inside of /agents/"+agentUuid+"/cmd" was run
	// So I end up with test passing randomly, because sometimes api was able to register agent before test sent cmd to api
	// and sometimes cmd was sent first, and then agent was registered by api
	time.Sleep(200 * time.Millisecond)

	return ws
}

func (a *Api) Get(path string, header http.Header) (*http.Response, []byte, error) {
	prefix := "http://" + a.getHostname()
	path = strings.TrimPrefix(path, prefix)
	req, err := http.NewRequest("GET", prefix+path, nil)
	if header == nil {
		header = http.Header{}
		header.Set("X-Percona-API-Key", "00000000000000000000000000000001")
	}
	req.Header = header

	resp, err := a.httpClient.Do(req)
	if err != nil {
		log.Println("xxxxx return 1", err)
		return resp, nil, err
	}
	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Println("xxxxx return 2")
		return resp, nil, err
	}
	return resp, content, nil
}

func (a *Api) Post(path string, header http.Header, data []byte) (*http.Response, []byte, error) {
	return a.send("POST", path, header, data)
}

func (a *Api) Put(path string, header http.Header, data []byte) (*http.Response, []byte, error) {
	return a.send("PUT", path, header, data)
}

func (a *Api) StartPost(path string, header http.Header, data []byte) chan Response {
	return a.startSend("POST", path, header, data)
}

func (a *Api) StartPut(path string, header http.Header, data []byte) chan Response {
	return a.startSend("PUT", path, header, data)
}

func (a *Api) send(method string, path string, header http.Header, data []byte) (*http.Response, []byte, error) {
	prefix := "http://" + a.getHostname()
	path = strings.TrimPrefix(path, prefix)
	req, err := http.NewRequest(method, prefix+path, bytes.NewReader(data))
	if header == nil {
		header = http.Header{}
		header.Set("X-Percona-API-Key", "00000000000000000000000000000001")
	}
	req.Header = header

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return resp, nil, err
	}
	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp, nil, err
	}
	return resp, content, nil
}

func (a *Api) startSend(method string, path string, header http.Header, data []byte) chan Response {
	respChan := make(chan Response, 10)
	go func() {
		resp, content, err := a.send(method, path, header, data)
		select {
		case respChan <- Response{resp, content, err}:
		default:
			panic("Unable to send response to respChan")
		}
	}()

	return respChan
}

func (a *Api) Delete(path string, header http.Header) (*http.Response, []byte, error) {
	prefix := "http://" + a.getHostname()
	path = strings.TrimPrefix(path, prefix)
	req, err := http.NewRequest("DELETE", prefix+path, nil)
	if header == nil {
		header = http.Header{}
		header.Set("X-Percona-API-Key", "00000000000000000000000000000001")
	}
	req.Header = header

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return resp, nil, err
	}
	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp, nil, err
	}
	return resp, content, nil
}

func (a *Api) getHostname() string {
	return a.GetHost() + ":" + a.GetPort()
}
