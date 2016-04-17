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

package db

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	errInvalidDSNUnescaped = errors.New("Invalid DSN: Did you forget to escape a param value?")
	errInvalidDSNAddr      = errors.New("Invalid DSN: Network Address not terminated (missing closing brace)")
	errInvalidDSNNoSlash   = errors.New("Invalid DSN: Missing the slash separating the database name")
)

type DSN struct {
	user              string
	passwd            string
	net               string
	addr              string
	port              string
	dbname            string
	params            map[string]string
	loc               *time.Location
	timeout           time.Duration
	allowAllFiles     bool
	allowOldPasswords bool
	clientFoundRows   bool
	dsn               string
}

// parseDSN parses the DSN string to a config
func NewDSN(dsn string) (cfg *DSN, err error) {
	cfg = &DSN{
		dsn: dsn,
	}

	// TODO: use strings.IndexByte when we can depend on Go 1.2

	// [user[:password]@][net[(addr)]]/dbname[?param1=value1&paramN=valueN]
	// Find the last '/' (since the password or the net addr might contain a '/')
	foundSlash := false
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			foundSlash = true
			var j, k int

			// left part is empty if i <= 0
			if i > 0 {
				// [username[:password]@][protocol[(address)]]
				// Find the last '@' in dsn[:i]
				for j = i; j >= 0; j-- {
					if dsn[j] == '@' {
						// username[:password]
						// Find the first ':' in dsn[:j]
						for k = 0; k < j; k++ {
							if dsn[k] == ':' {
								cfg.passwd = dsn[k+1 : j]
								break
							}
						}
						cfg.user = dsn[:k]

						break
					}
				}

				// [protocol[(address)]]
				// Find the first '(' in dsn[j+1:i]
				for k = j + 1; k < i; k++ {
					if dsn[k] == '(' {
						// dsn[i-1] must be == ')' if an address is specified
						if dsn[i-1] != ')' {
							if strings.ContainsRune(dsn[k+1:i], ')') {
								return nil, errInvalidDSNUnescaped
							}
							return nil, errInvalidDSNAddr
						}
						cfg.addr = dsn[k+1 : i-1]
						break
					}
				}
				cfg.net = dsn[j+1 : k]
			}

			// dbname[?param1=value1&...&paramN=valueN]
			// Find the first '?' in dsn[i+1:]
			for j = i + 1; j < len(dsn); j++ {
				if dsn[j] == '?' {
					if err = parseDSNParams(cfg, dsn[j+1:]); err != nil {
						return
					}
					break
				}
			}
			cfg.dbname = dsn[i+1 : j]

			break
		}
	}

	if !foundSlash && len(dsn) > 0 {
		return nil, errInvalidDSNNoSlash
	}

	// Set default network if empty
	if cfg.net == "" {
		cfg.net = "tcp"
	}

	// Set default address if empty
	if cfg.addr == "" {
		switch cfg.net {
		case "tcp":
			cfg.addr = "127.0.0.1:3306"
		case "unix":
			cfg.addr = "/tmp/mysql.sock"
		default:
			return nil, errors.New("Default addr for network '" + cfg.net + "' unknown")
		}

	}

	if strings.Contains(cfg.addr, ":") {
		parts := strings.Split(cfg.addr, ":")
		cfg.addr = parts[0]
		cfg.port = parts[1]
	}

	// Set default location if empty
	if cfg.loc == nil {
		cfg.loc = time.UTC
	}

	return cfg, nil
}

// parseDSNParams parses the DSN "query string"
// Values must be url.QueryEscape'ed
func parseDSNParams(cfg *DSN, params string) (err error) {
	for _, v := range strings.Split(params, "&") {
		param := strings.SplitN(v, "=", 2)
		if len(param) != 2 {
			continue
		}

		// cfg params
		switch value := param[1]; param[0] {

		// Disable INFILE whitelist / enable all files
		case "allowAllFiles":
			var isBool bool
			cfg.allowAllFiles, isBool = readBool(value)
			if !isBool {
				return fmt.Errorf("Invalid Bool value: %s", value)
			}

			// Switch "rowsAffected" mode
		case "clientFoundRows":
			var isBool bool
			cfg.clientFoundRows, isBool = readBool(value)
			if !isBool {
				return fmt.Errorf("Invalid Bool value: %s", value)
			}

			// Use old authentication mode (pre MySQL 4.1)
		case "allowOldPasswords":
			var isBool bool
			cfg.allowOldPasswords, isBool = readBool(value)
			if !isBool {
				return fmt.Errorf("Invalid Bool value: %s", value)
			}

			// Time Location
		case "loc":
			if value, err = url.QueryUnescape(value); err != nil {
				return
			}
			cfg.loc, err = time.LoadLocation(value)
			if err != nil {
				return
			}

			// Dial Timeout
		case "timeout":
			cfg.timeout, err = time.ParseDuration(value)
			if err != nil {
				return
			}

		default:
			// lazy init
			if cfg.params == nil {
				cfg.params = make(map[string]string)
			}

			if cfg.params[param[0]], err = url.QueryUnescape(value); err != nil {
				return
			}
		}
	}

	return
}

// Returns the bool value of the input.
// The 2nd return value indicates if the input was a valid bool value
func readBool(input string) (value bool, valid bool) {
	switch input {
	case "1", "true", "TRUE", "True":
		return true, true
	case "0", "false", "FALSE", "False":
		return false, true
	}

	// Not a valid bool value
	return
}
