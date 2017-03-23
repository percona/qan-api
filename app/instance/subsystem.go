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

package instance

import (
	"errors"

	"github.com/percona/pmm/proto"
)

var subsysName map[uint]string = map[uint]string{
	1: "os",
	2: "agent",
	3: "mysql",
	4: "mongo",
}

var subsys map[string]proto.Subsystem = map[string]proto.Subsystem{
	"os": {
		Id:       1,
		ParentId: 0,
		Name:     "os",
		Label:    "OS",
	},
	"agent": {
		Id:       2,
		ParentId: 1,
		Name:     "agent",
		Label:    "Agent",
	},
	"mysql": {
		Id:       3,
		ParentId: 1,
		Name:     "mysql",
		Label:    "MySQL",
	},
	"mongo": {
		Id:       4,
		ParentId: 1,
		Name:     "mongo",
		Label:    "MongoDB",
	},
}

var ErrNotFound = errors.New("subsystem not found")

func GetSubsystemById(id uint) (proto.Subsystem, error) {
	name, ok := subsysName[id]
	if !ok {
		return proto.Subsystem{}, ErrNotFound
	}
	return subsys[name], nil
}

func GetSubsystemByName(name string) (proto.Subsystem, error) {
	s, ok := subsys[name]
	if !ok {
		return proto.Subsystem{}, ErrNotFound
	}
	return s, nil
}
