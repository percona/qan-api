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
	"github.com/percona/qan-api/test/mock"
	"net/http"
)

func ConnectWs(client *mock.WebsocketClient, url string, extraHeaders map[string]string) error {
	headers := http.Header{
		"X-Percona-API-Key": []string{"1"},
	}
	if extraHeaders != nil {
		for k, v := range extraHeaders {
			headers[k] = []string{v}
		}
	}
	if err := client.Connect(url, "http://localhost", headers); err != nil {
		return err
	}

	return nil // success
}
