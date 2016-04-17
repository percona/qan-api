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

package shared

import (
	"errors"
	"net"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrNoMetric          = errors.New("metric not found")
	ErrNoData            = errors.New("no data in time range")
	ErrNoService         = errors.New("service not found")
	ErrInvalidService    = errors.New("invalid service")
	ErrNoInstance        = errors.New("instance not found")
	ErrReadOnlyDb        = errors.New("database is read-only")
	ErrDuplicateEntry    = errors.New("duplicate entry")
	ErrGoroutineCrash    = errors.New("goroutine crashed")
	ErrUnexpectedReturn  = errors.New("unexpected return")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrDuplicateAgent    = errors.New("duplicate agent")
	ErrAgentNotConnected = errors.New("agent not connected")
	ErrLinkClosed        = errors.New("link closed")
	ErrNotImplemented    = errors.New("not implemented")
	ErrTimeout           = errors.New("timeout")
	ErrAgentReplyError   = errors.New("agent reply error")
)

func IsNetworkError(err error) bool {
	switch err.(type) {
	case *net.OpError:
		return true
	}
	return false
}
