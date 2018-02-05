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

package controllers

import (
	"github.com/percona/pmm/proto"
	"github.com/revel/revel"
)

type Home struct {
	BackEnd
}

func (c Home) Links() revel.Result {
	httpBase := c.Args["httpBase"].(string)
	links := &proto.Links{
		Links: map[string]string{
			"agents":    httpBase + "/agents",
			"instances": httpBase + "/instances",
		},
	}
	return c.RenderJSON(links)
}

func (c Home) Ping() revel.Result {
	revel.TRACE.Println("Home.Ping")
	c.Response.Status = 200
	return c.RenderText("")
}

func (c Home) Options() revel.Result {
	c.Response.Out.Header().Set("Access-Control-Allow-Origin", "*")
	c.Response.Out.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,DELETE")
	c.Response.Out.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
	c.Response.Out.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
	c.Response.Status = 200
	return c.RenderText("")
}
