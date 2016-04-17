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

package factory

import (
	"golang.org/x/net/websocket"

	"github.com/percona/qan-api/app/ws"
)

type ConnectorFactory struct {
}

func (f ConnectorFactory) Make(url, origin string) ws.Connector {
	return ws.NewConnection(url, origin, nil)
}

func (f ConnectorFactory) Use(url, origin string, conn *websocket.Conn) ws.Connector {
	return ws.ExistingConnection(url, origin, conn)
}
