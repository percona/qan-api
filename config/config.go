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

package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/revel/config"
)

var gConfig *config.Config
var ErrNoConfig = errors.New("no config file loaded")
var ApiRootDir string
var TestDir string
var SchemaDir string

var hostname string // from Docker ADDRESS env var
var portSuffix *regexp.Regexp = regexp.MustCompile(`:\d+$`)

func init() {
	configFile := os.Getenv("PERCONA_DATASTORE_CONF")
	if configFile == "" {
		configFile = "dev.conf"
	}
	var err error
	gConfig, err = loadConfig(configFile)
	if err != nil {
		log.Panic(err)
	}
	hostname = os.Getenv("ADDRESS")
	if hostname != "" {
		if !portSuffix.Match([]byte(hostname)) {
			hostname += ":9001"
		}
	}
}

func Get(name string) string {
	if gConfig == nil {
		log.Panic("No config")
	}
	if hostname != "" {
		switch name {
		case "hostname":
			return hostname
		}
	}
	value, err := gConfig.RawStringDefault(name)
	if err != nil {
		log.Panic(err)
	}
	return value
}

func loadConfig(configFile string) (conf *config.Config, err error) {
	if configFile != "" && configFile[0] == '/' {
		log.Printf("Loading %s", configFile)
		if conf, err = config.ReadDefault(configFile); err == nil {
			setRootDir(configFile)
			return conf, nil
		}
	}
	confPaths := getConfPaths()
	for _, confPath := range confPaths {
		log.Printf("Loading %s/%s", confPath, configFile)
		conf, err := config.ReadDefault(path.Join(confPath, configFile))
		if err == nil {
			setRootDir(confPath)
			return conf, nil
		}
	}

	// If we are here then we didn't found out where is config file
	err = errors.New(fmt.Sprintf("Config %s not found in paths: %s", configFile, strings.Join(confPaths, ", ")))
	return nil, err
}

func getConfPaths() (confPaths []string) {
	// We can read config from revel.ConfPaths or from GOPATH
	// but please remember that both variables can store multiple paths
	goPaths := filepath.SplitList(os.Getenv("GOPATH"))
	for _, goPath := range goPaths {
		confPaths = append(confPaths, path.Join(goPath, "/src/github.com/percona/qan-api/conf"))
	}
	return confPaths
}

func setRootDir(confFile string) {
	ApiRootDir = os.Getenv("PERCONA_DATASTORE_BASEDIR")
	if ApiRootDir == "" {
		dir, _ := filepath.Split(confFile)
		dirs := filepath.SplitList(dir)
		ApiRootDir = dirs[0]
	}
	TestDir = path.Join(ApiRootDir, "test")
	SchemaDir = path.Join(ApiRootDir, "schema")
	log.Printf("API root dir " + ApiRootDir)
}
