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

import (
	"fmt"
	"log"
)

type ApiService struct {
	api map[string]*Api
}

func NewApiService() *ApiService {
	return &ApiService{
		api: make(map[string]*Api),
	}
}

func (a *ApiService) GetApi(host string, port string) *Api {
	api, ok := a.api[host+":"+port]
	if ok {
		return api
	}

	api = NewApi(host, port)
	err := api.Start()
	if err != nil {
		log.Fatal(fmt.Printf("Can't start %s:%s api: %s", host, port, err))
	}
	a.api[host+":"+port] = api

	return api
}

func (a *ApiService) StopApi(host string, port string) {
	name := host + ":" + port
	api, ok := a.api[name]
	if !ok {
		return
	}
	api.Stop()
	delete(a.api, name)
}

func (a *ApiService) StopAllApi() {
	for _, api := range a.api {
		api.Stop()
	}
	a.api = make(map[string]*Api)
}

func (a *ApiService) DisconnectAllClients() {
	for _, api := range a.api {
		api.DisconnectAllClients()
	}
}
