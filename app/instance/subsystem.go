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

const (
	SubsystemOS = iota + 1
	SubsystemAgent
	SubsystemMySQL
	SubsystemMongo
)

const (
	SubsystemNameOS    = "os"
	SubsystemNameAgent = "agent"
	SubsystemNameMySQL = "mysql"
	SubsystemNameMongo = "mongo"
)

var subsysName map[uint]string = map[uint]string{
	SubsystemOS:    SubsystemNameOS,
	SubsystemAgent: SubsystemNameAgent,
	SubsystemMySQL: SubsystemNameMySQL,
	SubsystemMongo: SubsystemNameMongo,
}

var subsys map[string]proto.Subsystem = map[string]proto.Subsystem{
	SubsystemNameOS: {
		Id:       1,
		ParentId: 0,
		Name:     SubsystemNameOS,
		Label:    "OS",
	},
	SubsystemNameAgent: {
		Id:       2,
		ParentId: 1,
		Name:     SubsystemNameAgent,
		Label:    "Agent",
	},
	SubsystemNameMySQL: {
		Id:       3,
		ParentId: 1,
		Name:     SubsystemNameMySQL,
		Label:    "MySQL",
	},
	SubsystemNameMongo: {
		Id:       4,
		ParentId: 1,
		Name:     SubsystemNameMongo,
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
