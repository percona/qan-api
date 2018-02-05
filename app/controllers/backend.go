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
	"fmt"
	"net/http"

	"github.com/percona/pmm/proto"
	"github.com/percona/qan-api/app/shared"
	"github.com/revel/revel"
)

type BackEnd struct {
	*revel.Controller
}

func (c BackEnd) Error(err error, op string) revel.Result {
	switch err {
	// //////////////////////////////////////////////////////////////////////
	// Not really an error, no content
	// //////////////////////////////////////////////////////////////////////
	case shared.ErrNotFound:
		c.Response.Status = http.StatusNotFound // 404
		return c.RenderText("")
	case shared.ErrDuplicateEntry:
		c.Response.Status = http.StatusConflict // 409
		return c.RenderText("")
	case shared.ErrNotImplemented:
		c.Response.Status = http.StatusNotImplemented // 501
		return c.Render("")

	// //////////////////////////////////////////////////////////////////////
	// Kind of an error but not 500, error response
	// //////////////////////////////////////////////////////////////////////
	case shared.ErrAgentReplyError:
		res := proto.Error{
			Error: op,
		}
		c.Response.Status = http.StatusNonAuthoritativeInfo // 203
		return c.RenderJSON(res)
	case shared.ErrAgentNotConnected:
		res := proto.Error{
			Error: shared.ErrAgentNotConnected.Error(),
		}
		c.Response.Status = http.StatusNotFound // 404
		return c.RenderJSON(res)
	case shared.ErrReadOnlyDb:
		res := proto.Error{
			Error: "database is in read-only mode",
		}
		c.Response.Status = http.StatusServiceUnavailable // 503
		return c.RenderJSON(res)

	// //////////////////////////////////////////////////////////////////////
	// 500 error, something blew up
	// //////////////////////////////////////////////////////////////////////
	default:
		errMsg := fmt.Sprintf("%s: %s", op, err)
		revel.ERROR.Printf(errMsg)
		res := proto.Error{
			Error: errMsg,
		}
		c.Response.Status = http.StatusInternalServerError
		return c.RenderJSON(res)
	}
	return nil
}

func (c BackEnd) BadRequest(err error, msg string) revel.Result {
	if err != nil {
		msg += ": " + err.Error()
	}
	res := proto.Error{
		Error: msg,
	}
	c.Response.Status = http.StatusBadRequest // 400
	return c.RenderJSON(res)
}

type NoContent struct {
}

func (c BackEnd) RenderNoContent() revel.Result {
	return &NoContent{}
}

func (j *NoContent) Apply(req *revel.Request, resp *revel.Response) {
	resp.Out.WriteHeader(http.StatusNoContent) // 204
}

type Created struct {
	Location string
}

func (c BackEnd) RenderCreated(location string) revel.Result {
	return &Created{
		Location: location,
	}
}

func (j *Created) Apply(req *revel.Request, resp *revel.Response) {
	resp.Out.Header().Set("Location", j.Location)
	resp.Out.WriteHeader(http.StatusCreated) // 201
}
