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

package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sort"
	"time"

	"github.com/percona/qan-api/app/db"
	"github.com/percona/qan-api/app/db/mysql"
	"github.com/percona/qan-api/app/shared"
	"github.com/percona/qan-api/app/ws"
	"github.com/percona/qan-api/stats"
	"github.com/percona/pmm/proto"
)

const (
	MIN_LOG_LEVEL = proto.LOG_ERROR
	MAX_LOG_LEVEL = proto.LOG_DEBUG
)

type ByTs []proto.LogEntry

func (s ByTs) Len() int           { return len(s) }
func (s ByTs) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ByTs) Less(i, j int) bool { return s[i].Ts.After(s[j].Ts) } // descending

type LogFilter struct {
	Begin       time.Time
	End         time.Time
	MinLevel    byte // inclusive
	MaxLevel    byte // inclusive
	ServiceLike string
}

// --------------------------------------------------------------------------
// Log handler
// --------------------------------------------------------------------------

type LogHandler struct {
	dbm   db.Manager
	stats *stats.Stats
}

func NewLogHandler(dbm db.Manager, stats *stats.Stats) *LogHandler {
	h := &LogHandler{
		dbm:   dbm,
		stats: stats,
	}
	return h
}

func (lh *LogHandler) WriteLog(agentId uint, logEntries []proto.LogEntry) error {
	tx, err := lh.dbm.DB().Begin()
	if err != nil {
		return err
	}
	defer tx.Commit()

	stmt, err := tx.Prepare("INSERT INTO agent_log (instance_id, sec, nsec, level, service, msg) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return mysql.Error(err, "Prepare INSERT agent_log")
	}
	defer stmt.Close()

	lh.stats.SetComponent("db")

	for _, logEntry := range logEntries {
		_, err := stmt.Exec(
			agentId,
			logEntry.Ts.Unix(),
			logEntry.Ts.Nanosecond(),
			logEntry.Level,
			logEntry.Service,
			logEntry.Msg,
		)
		if err != nil {
			log.Printf("WARN: agent_id=%d: WriteLog: %s\n", agentId, err)
			continue
		}
	}

	return nil
}

func (lh *LogHandler) GetLog(agentId uint, f LogFilter) ([]proto.LogEntry, error) {
	serviceLike := ""
	if f.ServiceLike != "" {
		serviceLike = " AND (service LIKE ?)"
	}
	query := "SELECT sec, nsec, level, service, msg FROM agent_log" +
		" WHERE instance_id = ?" +
		" AND (sec >= ? AND sec < ?)" +
		" AND (level BETWEEN ? AND ?)" +
		serviceLike

	rows, err := lh.dbm.DB().Query(query, agentId, f.Begin.Unix(), f.End.Unix(), f.MinLevel, f.MaxLevel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []proto.LogEntry
	for rows.Next() {
		var sec, nsec int64
		logEntry := proto.LogEntry{}
		err := rows.Scan(
			&sec,
			&nsec,
			&logEntry.Level,
			&logEntry.Service,
			&logEntry.Msg,
		)
		if err != nil {
			return nil, err
		}
		logEntry.Ts = time.Unix(sec, nsec).UTC()
		logs = append(logs, logEntry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort descending because this data is used primarily in QAN app, so latest
	// entries appear at top of list, and user can scroll down to scroll back in
	// time. This also saves MySQL the trouble of doing a filesort.
	if len(logs) > 1 {
		sort.Sort(ByTs(logs))
	}

	return logs, nil
}

// --------------------------------------------------------------------------
// Log entry buffer-writer
// --------------------------------------------------------------------------

const LOG_BUF_SIZE = 10

func SaveLog(wsConn ws.Connector, agentId uint, tickChan <-chan time.Time, logHandler *LogHandler, stats *stats.Stats) error {
	prefix := fmt.Sprintf("[SaveLog] agent_id=%d", agentId)

	stats.SetComponent("log.msg")

	n := 0
	buf := make([]proto.LogEntry, LOG_BUF_SIZE) // circular, 0:n
	levels := make([]int64, len(proto.LogLevelNumber))

	for {
		// Receive the proto.LogEntry as bytes so we can measure the number of
		// incoming bytes below.
		bytes, err := wsConn.RecvBytes(0)
		if err != nil {
			if err == io.EOF {
				// Agent done sending, closed websocket. Data controller ignores this
				// error so don't change it with fmt.Errorf().
				return err
			} else {
				return fmt.Errorf("ww.RecvBytes:%s", err)
			}
		}

		// Record the number of incoming bytes so we can find orgs/agents tha
		// are saturating our network.
		stats.Inc(stats.System("bytes"), int64(len(bytes)), stats.SampleRate)

		// Decode the bytes as a proto.LogEntry.
		var logEntry proto.LogEntry
		if err := json.Unmarshal(bytes, &logEntry); err != nil {
			fmt.Printf("%s ERROR: json.Unmarshal: %s\n", prefix, err)
			continue
		}
		levels[logEntry.Level]++

		if n < LOG_BUF_SIZE {
			buf[n] = logEntry
			n++
		}

		haveTick := false
		select {
		case <-tickChan:
			haveTick = true
		default:
		}
		if !haveTick && n < LOG_BUF_SIZE { // no tick and buffer not full
			continue
		}
		if len(buf) == 0 { // tick but buffer empty
			continue
		}

		// Tick or buffer full, save all log entrires.

		stats.Inc(stats.System("flush"), int64(n), stats.SampleRate)

		for level, cnt := range levels {
			if cnt == 0 {
				continue
			}
			stats.Inc(stats.System("level-"+proto.LogLevelName[level]), cnt, stats.SampleRate)
			levels[level] = 0
		}

		// Insert all log entries in a single transaction.
		t := time.Now()
		err = logHandler.WriteLog(agentId, buf[0:n])
		stats.TimingDuration(stats.System("db"), time.Now().Sub(t), stats.SampleRate)
		if err != nil {
			switch {
			case shared.IsNetworkError(err):
				log.Printf("%s WARN: network error: %s\n", prefix, err)
				break
			case err == shared.ErrReadOnlyDb:
				log.Printf("%s WARN: read-only db\n", prefix)
				time.Sleep(3 * time.Second)
			default:
				log.Printf("%s ERROR: %s\n", prefix, err)
				break
			}
		}

		n = 0 // reset circular buffer
	}
}
