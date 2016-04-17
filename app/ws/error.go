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
	"fmt"
	"io"
	"net"

	"github.com/percona/qan-api/app/shared"
)

var (
	ErrNotConnected = errors.New("not connected")
	ErrNotRunning   = errors.New("not running")
	ErrStopped      = errors.New("stopped")
	ErrSendTimeout  = errors.New("send timeout")
	ErrRecvTimeout  = errors.New("recv timeout")
)

type Error struct {
	Bytes []byte
	Error error
}

func isTimeout(err error) bool {
	e, ok := err.(net.Error)
	return ok && e.Timeout()
}

func sharedError(err error, msg string) error {
	switch {
	case err == nil:
		return nil
	case isTimeout(err):
		return shared.ErrTimeout
	case err == io.EOF:
		return err
	default:
		return fmt.Errorf("%s: %s", msg, err)
	}
}
