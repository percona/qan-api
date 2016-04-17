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

package test

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/percona/pmm/proto"
)

var RootDir string

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

	_, filename, _, _ := runtime.Caller(1)
	dir := filepath.Dir(filename)

	for i := 0; i < 3; i++ {
		dir = dir + "/../"
		if FileExists(dir + ".git") {
			RootDir = filepath.Clean(dir + "test")
			break
		}
	}
	if RootDir == "" {
		log.Panic("Cannot find repo root dir")
	}
}

func FileExists(file string) bool {
	_, err := os.Stat(file)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func Dump(v interface{}) {
	bytes, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(bytes))
}

func CopyFile(src, dst string) error {
	cmd := exec.Command("cp", src, dst)
	return cmd.Run()
}

func DrainCmdChan(c chan *proto.Cmd) {
DRAIN:
	for {
		select {
		case _ = <-c:
		default:
			break DRAIN
		}
	}
}
