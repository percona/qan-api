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
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
)

const (
	MYSQL_DATETIME_LAYOUT = "2006-01-02 15:04:05"
)

var vre = regexp.MustCompile("-.*$")

func AtLeastVersion(v1, v2 string) (bool, error) {
	v1 = vre.ReplaceAllString(v1, "") // Strip everything after the first dash
	v2 = vre.ReplaceAllString(v2, "") // Strip everything after the first dash
	v, err := version.NewVersion(v1)
	if err != nil {
		return false, err
	}
	c, err := version.NewConstraint(">= " + v2)
	if err != nil {
		return false, err
	}
	return c.Check(v), nil
}

func Placeholders(length int) string {
	return strings.Join(strings.Split(strings.Repeat("?", length), ""), ",")
}

func GenericStringList(s []string) []interface{} {
	v := make([]interface{}, len(s))
	for i := range s {
		v[i] = s[i]
	}
	return v
}

func ValidateTimeRange(beginTs, endTs string) (time.Time, time.Time, error) {
	var err error
	var begin, end time.Time
	begin, err = time.Parse("2006-01-02T15:04:05", beginTs)
	if err != nil {
		return begin, end, fmt.Errorf("invalid begin: '%s': %s", beginTs, err)
	}
	end, err = time.Parse("2006-01-02T15:04:05", endTs)
	if err != nil {
		return begin, end, fmt.Errorf("invalid end: '%s': %s", beginTs, err)
	}
	return begin, end, nil
}
