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

package query

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

	queryProto "github.com/percona/pmm/proto/query"
	"github.com/youtube/vitess/go/vt/sqlparser"
)

type QueryInfo struct {
	Fingerprint string
	Abstract    string
	Tables      []queryProto.Table
}

type parseTry struct {
	query     string
	q         QueryInfo
	s         sqlparser.Statement
	queryChan chan QueryInfo
	crashChan chan bool
}

type protoTables []queryProto.Table

func (t protoTables) String() string {
	s := ""
	sep := ""
	for _, table := range t {
		s += sep + table.String()
		sep = " "
	}
	return s
}

const (
	MAX_JOIN_DEPTH = 100
)

var (
	ErrNotSupported = errors.New("SQL parser does not support the query")
)

type Mini struct {
	Debug      bool
	cwd        string
	queryIn    chan string
	miniOut    chan string
	parseChan  chan parseTry
	onlyTables bool
	stopChan   chan struct{}
}

func NewMini(cwd string) *Mini {
	m := &Mini{
		cwd:        cwd,
		onlyTables: cwd == "",         // only tables if no path to mini.pl given
		queryIn:    make(chan string), // XXX see note below
		miniOut:    make(chan string), // XXX see note below
		parseChan:  make(chan parseTry, 1),
		stopChan:   make(chan struct{}),
	}
	return m
	/// XXX DO NOT BUFFER queryIn or miniOut, else everything will break!
	//      There's only 1 mini.pl proc per Mini instance, and the Mini instance
	//      can be shared (e.g. processing QAN data for mulitple agents).
	//      Unbuffered chans serialize access to mini.pl in usePerl(). If either
	//      one of the chans is buffered, a race condition is created which
	//      results in goroutines receiving the wrong data. -- parseChan is a
	///     different approach; it can be buffered.
}

func (m *Mini) Stop() {
	close(m.stopChan)
}

func (m *Mini) Run() {
	// Go-based SQL parsing
	go m.parse()

	// Perl-based SQL parsing
	if !m.onlyTables {
		cmd := exec.Command(m.cwd + "/mini.pl")

		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Fatal(err)
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}

		r := bufio.NewReader(stdout)

		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}

		for {
			select {
			case query := <-m.queryIn:
				// Do not use buffered IO so input/output is immediate.
				// Do not forget "\n" because mini.pl is reading lines.
				if _, err := io.WriteString(stdin, query+"\n"); err != nil {
					log.Fatal(err)
				}
				q, err := r.ReadString('\n')
				if err != nil {
					log.Fatal(err)
				}
				m.miniOut <- q
			case <-m.stopChan:
				return
			}
		}
	}
}

func (m *Mini) Parse(fingerprint, example, defaultDb string) (QueryInfo, error) {
	fingerprint = strings.TrimSpace(fingerprint)
	example = strings.TrimSpace(example)
	q := QueryInfo{
		Fingerprint: fingerprint,
		Tables:      []queryProto.Table{},
	}
	defer func() {
		q.Abstract = strings.TrimSpace(q.Abstract)
	}()

	if m.Debug {
		fmt.Printf("\n\nexample: %s\n", example)
		fmt.Printf("\n\nfingerprint: %s\n", fingerprint)
	}

	query := fingerprint
	// If we have a query example, that's better to parse than a fingerprint.
	if example != "" {
		query = example
	}

	// Fingerprints replace IN (1, 2) -> in (?+) but "?+" is not valid SQL so
	// it breaks sqlparser/.
	query = strings.Replace(query, "?+", "? ", -1)

	// Internal newlines break everything.
	query = strings.Replace(query, "\n", " ", -1)

	s, err := sqlparser.Parse(query)
	if err != nil {
		if m.Debug {
			fmt.Println("ERROR:", err)
		}
		return m.usePerl(query, q, err)
	}

	// Parse the SQL structure. The sqlparser is rather terrible, incomplete code,
	// so it's prone to crash. If that happens, fall back to using the Perl code
	// which only gets the abstract. Be sure to re-run the parse() goroutine for
	// other callers and queries.
	try := parseTry{
		query:     query,
		q:         q,
		s:         s,
		queryChan: make(chan QueryInfo, 1),
		crashChan: make(chan bool, 1),
	}
	m.parseChan <- try
	select {
	case q = <-try.queryChan:
	case <-try.crashChan:
		fmt.Printf("WARN: query crashes sqlparser: %s\n", query)
		go m.parse()
		return m.usePerl(query, q, err)
	}

	if defaultDb != "" {
		for n, t := range q.Tables {
			if t.Db == "" {
				q.Tables[n].Db = defaultDb
			}
		}
	}

	return q, nil
}

func (m *Mini) parse() {
	var crashChan chan bool
	defer func() {
		if err := recover(); err != nil {
			crashChan <- true
		}
	}()
	for {
		select {
		case p := <-m.parseChan:
			q := p.q
			crashChan = p.crashChan
			switch s := p.s.(type) {
			case *sqlparser.Select:
				q.Abstract = "SELECT"
				if m.Debug {
					fmt.Printf("struct: %#v\n", s)
				}
				tables := getTablesFromTableExprs(s.From)
				if len(tables) > 0 {
					q.Tables = append(q.Tables, tables...)
					q.Abstract += " " + tables.String()
				}
			case *sqlparser.Insert:
				// REPLACEs will be recognized by sqlparser as INSERTs and the Action field
				// will have the real command
				q.Abstract = strings.ToUpper(s.Action)
				if m.Debug {
					fmt.Printf("struct: %#v\n", s)
				}
				table := queryProto.Table{
					Db:    s.Table.Qualifier.String(),
					Table: s.Table.Name.String(),
				}
				q.Tables = append(q.Tables, table)
				q.Abstract += " " + table.String()
			case *sqlparser.Update:
				q.Abstract = "UPDATE"
				if m.Debug {
					fmt.Printf("struct: %#v\n", s)
				}
				tables := getTablesFromTableExprs(s.TableExprs)
				if len(tables) > 0 {
					q.Tables = append(q.Tables, tables...)
					q.Abstract += " " + tables.String()
				}
			case *sqlparser.Delete:
				q.Abstract = "DELETE"
				if m.Debug {
					fmt.Printf("struct: %#v\n", s)
				}
				tables := getTablesFromTableExprs(s.TableExprs)
				if len(tables) > 0 {
					q.Tables = append(q.Tables, tables...)
					q.Abstract += " " + tables.String()
				}
			case *sqlparser.Use:
				q.Abstract = "USE"
			case *sqlparser.Show:
				sql := sqlparser.NewTrackedBuffer(nil)
				s.Format(sql)
				q.Abstract = strings.ToUpper(sql.String())
			default:
				if m.Debug {
					fmt.Printf("unsupported type: %#v\n", p.s)
				}
				q, _ = m.usePerl(p.query, q, ErrNotSupported)
				switch use := p.s.(type) {
				case *sqlparser.DDL:
					table := queryProto.Table{
						Db:    use.NewName.Qualifier.String(),
						Table: use.NewName.Name.String(),
					}
					q.Tables = append(q.Tables, table)
				}
			}
			p.queryChan <- q
		case <-m.stopChan:
			return
		}
	}
}

func (m *Mini) usePerl(query string, q QueryInfo, originalErr error) (QueryInfo, error) {
	if m.onlyTables {
		// Caller wants only tables but we can't get them because sqlparser
		// failed for this query.
		return q, originalErr
	}
	m.queryIn <- query
	abstract := <-m.miniOut
	q.Abstract = strings.Replace(abstract, "\n", "", -1)
	return q, nil
}

func getTablesFromTableExprs(tes sqlparser.TableExprs) (tables protoTables) {
	for _, te := range tes {
		tables = append(tables, getTablesFromTableExpr(te, 0)...)
	}
	return tables
}

func getTablesFromTableExpr(te sqlparser.TableExpr, depth uint) (tables protoTables) {
	if depth > MAX_JOIN_DEPTH {
		return nil
	}
	depth++
	switch a := te.(type) {
	case *sqlparser.AliasedTableExpr:
		n := a.Expr.(sqlparser.TableName)
		db := n.Qualifier.String()
		tbl := parseTableName(n.Name.String())
		if db != "" || tbl != "" {
			table := queryProto.Table{
				Db:    db,
				Table: tbl,
			}
			tables = append(tables, table)
		}
	case *sqlparser.JoinTableExpr:
		// This case happens for JOIN clauses. It recurses to the bottom
		// of the tree via the left expressions, then it unwinds. E.g. with
		// "a JOIN b JOIN c" the tree is:
		//
		//  Left			Right
		//  a     b      c	AliasedTableExpr (case above)
		//  |     |      |
		//  +--+--+      |
		//     |         |
		//    t2----+----+	JoinTableExpr
		//          |
		//        var t (t @ depth=1) JoinTableExpr
		//
		// Code will go left twice to arrive at "a". Then it will unwind and
		// store the right-side values: "b" then "c". Because of this, if
		// MAX_JOIN_DEPTH is reached, we lose the whole tree because if we take
		// the existing right-side tables, we'll generate a misleading partial
		// list of tables, e.g. "SELECT b c".
		tables = append(tables, getTablesFromTableExpr(a.LeftExpr, depth)...)
		tables = append(tables, getTablesFromTableExpr(a.RightExpr, depth)...)
	}

	return tables
}

func parseTableName(tableName string) string {
	// https://dev.mysql.com/doc/refman/5.7/en/select.html#idm140358784149168
	// You are permitted to specify DUAL as a dummy table name in situations where no tables are referenced:
	//
	// ```
	// mysql> SELECT 1 + 1 FROM DUAL;
	//         -> 2
	// ```
	// DUAL is purely for the convenience of people who require that all SELECT statements
	// should have FROM and possibly other clauses. MySQL may ignore the clauses.
	// MySQL does not require FROM DUAL if no tables are referenced.
	if tableName == "dual" {
		tableName = ""
	}
	return tableName
}
