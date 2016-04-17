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

package main

import (
	"github.com/revel/revel"
	"github.com/revel/revel/harness"
)

// This is used to build routes.go before running tests
// routes.go is needed in some tests for reverse routing.
// IMO routes.go should be included in repo and updated whenever routes are changed.
// However revel doesn't cooperate here and a) every time it generates routes
// it does it in "random" order and b) it's not properly formatted (no go fmt)
// so git recognises this as changes to routes.go file.
func main() {
	revel.Init("dev", "github.com/percona/qan-api", "")
	_, err := harness.Build()
	if err != nil {
		panic(err)
	}
}
