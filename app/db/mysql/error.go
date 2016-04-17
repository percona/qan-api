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

package mysql

import (
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/percona/qan-api/app/shared"
)

// Map MySQL error to shared app error and return, if any, because db handler
// callers don't know or care about the underlying database.
func Error(err error, msg string) error {
	switch {
	case err == nil:
		return nil
	case ErrorCode(err) == ER_OPTION_PREVENTS_STATEMENT:
		return shared.ErrReadOnlyDb
	case ErrorCode(err) == ER_DUP_ENTRY:
		return shared.ErrDuplicateEntry
	case err == sql.ErrNoRows:
		return shared.ErrNotFound
	default:
		return fmt.Errorf("%s: %s", msg, err)
	}
}

func ErrorCode(err error) uint16 {
	if val, ok := err.(*mysql.MySQLError); ok {
		return val.Number
	}

	return 0 // not a mysql error
}

// MySQL error codes
const (
	ER_DUP_ENTRY                 = 1062
	ER_OPTION_PREVENTS_STATEMENT = 1290
)
