// Copyright (C) 2019-2022 Chrystian Huot <chrystian.huot@saubeo.solutions>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>

package main

// LogEvent always stamps time.Now(), so these tests work relative to the
// moment the events were logged. MySQL/MariaDB store whole seconds, hence the
// short sleeps to make the first and last events distinguishable.

import (
	"testing"
	"time"
)

type logFixture struct {
	logs  *Logs
	first time.Time // just before the first event
	last  time.Time // just after the last event
}

// logFourEvents logs "first info", then (a second later) a warning, an error
// and "second info".
func logFourEvents(t *testing.T, db *Database) *logFixture {
	t.Helper()
	logs := NewLogs()
	logs.setDatabase(db)

	f := &logFixture{logs: logs, first: time.Now().UTC()}
	must(t, logs.LogEvent(LogLevelInfo, "first info"))
	time.Sleep(1100 * time.Millisecond)
	must(t, logs.LogEvent(LogLevelWarn, "a warning"))
	must(t, logs.LogEvent(LogLevelError, "an error"))
	must(t, logs.LogEvent(LogLevelInfo, "second info"))
	f.last = time.Now().UTC()
	return f
}

func (f *logFixture) search(t *testing.T, db *Database, opts *LogsSearchOptions) *LogsSearchResults {
	t.Helper()
	res, err := f.logs.Search(opts, db)
	must(t, err)
	return res
}

func logMessages(res *LogsSearchResults) []string {
	out := []string{}
	for _, l := range res.Logs {
		out = append(out, l.Message)
	}
	return out
}

// within reports whether ts lies in [from, to] allowing for whole-second storage.
func within(ts, from, to time.Time) bool {
	return !ts.Before(from.Add(-time.Second)) && !ts.After(to.Add(time.Second))
}

func TestLogsSearch(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		f := logFourEvents(t, db)

		// Everything, ascending.
		res := f.search(t, db, NewLogSearchOptions())
		assertEqual(t, "all: count", res.Count, uint(4))
		if len(res.Logs) != 4 {
			t.Fatalf("all: got %d logs, want 4", len(res.Logs))
		}
		assertEqual(t, "all: first message", res.Logs[0].Message, "first info")
		assertEqual(t, "all: first level", res.Logs[0].Level, LogLevelInfo)
		levels := map[string]string{}
		for i, l := range res.Logs {
			if l.Id == nil {
				t.Errorf("log %d has no id", i)
			}
			if !within(l.DateTime, f.first, f.last) {
				t.Errorf("log %q: dateTime %v outside [%v, %v]", l.Message, l.DateTime, f.first, f.last)
			}
			if i > 0 && l.DateTime.Before(res.Logs[i-1].DateTime) {
				t.Errorf("ascending order broken at %d", i)
			}
			levels[l.Message] = l.Level
		}
		assertEqual(t, "levels", levels, map[string]string{
			"first info": LogLevelInfo, "a warning": LogLevelWarn, "an error": LogLevelError, "second info": LogLevelInfo,
		})
		assertSameSecond(t, "all: dateStart", res.DateStart, res.Logs[0].DateTime)

		// Descending.
		res = f.search(t, db, &LogsSearchOptions{Sort: -1})
		assertEqual(t, "desc: count", res.Count, uint(4))
		assertEqual(t, "desc: last message", res.Logs[len(res.Logs)-1].Message, "first info")
		for i := 1; i < len(res.Logs); i++ {
			if res.Logs[i].DateTime.After(res.Logs[i-1].DateTime) {
				t.Errorf("descending order broken at %d", i)
			}
		}

		// Level filter.
		res = f.search(t, db, &LogsSearchOptions{Level: LogLevelInfo})
		assertEqual(t, "info: count", res.Count, uint(2))
		assertEqual(t, "info: messages", logMessages(res), []string{"first info", "second info"})
		res = f.search(t, db, &LogsSearchOptions{Level: LogLevelInfo, Sort: -1})
		assertEqual(t, "info desc: messages", logMessages(res), []string{"second info", "first info"})
		res = f.search(t, db, &LogsSearchOptions{Level: LogLevelError})
		assertEqual(t, "error: messages", logMessages(res), []string{"an error"})
		res = f.search(t, db, &LogsSearchOptions{Level: "debug"})
		assertEqual(t, "unknown level: count", res.Count, uint(0))
		assertEqual(t, "unknown level: messages", logMessages(res), []string{})

		// Limit and offset; count is the full total.
		res = f.search(t, db, &LogsSearchOptions{Limit: uint(1)})
		assertEqual(t, "limit: count", res.Count, uint(4))
		assertEqual(t, "limit: messages", logMessages(res), []string{"first info"})
		res = f.search(t, db, &LogsSearchOptions{Limit: uint(2), Offset: uint(1), Level: LogLevelInfo})
		assertEqual(t, "offset: messages", logMessages(res), []string{"second info"})

		// Date (ascending): the 24h starting at the given minute.
		res = f.search(t, db, &LogsSearchOptions{Date: f.first.Add(-time.Hour)})
		assertEqual(t, "date before: count", res.Count, uint(4))
		res = f.search(t, db, &LogsSearchOptions{Date: f.last.Add(time.Hour)})
		assertEqual(t, "date after: count", res.Count, uint(0))
		res = f.search(t, db, &LogsSearchOptions{Date: f.first.Add(-25 * time.Hour)})
		assertEqual(t, "date a day before: count", res.Count, uint(0))
		res = f.search(t, db, &LogsSearchOptions{Date: f.first.Add(-time.Hour), Level: LogLevelWarn})
		assertEqual(t, "date + level: messages", logMessages(res), []string{"a warning"})
	})
}

// DateStop must be the newest matching log; the "max" query sorts ascending,
// so it always equals DateStart.
func TestLogsSearchDateStop(t *testing.T) {

	forEachDatabase(t, func(t *testing.T, db *Database) {
		f := logFourEvents(t, db)
		res := f.search(t, db, NewLogSearchOptions())
		if len(res.Logs) != 4 {
			t.Fatalf("got %d logs, want 4", len(res.Logs))
		}
		assertSameSecond(t, "dateStart", res.DateStart, res.Logs[0].DateTime)
		assertSameSecond(t, "dateStop", res.DateStop, res.Logs[3].DateTime)
		if sameSecond(res.DateStart, res.DateStop) {
			t.Errorf("dateStop %v equals dateStart", res.DateStop)
		}
	})
}

// A descending date search covers the 24h up to the given minute (as calls
// do). Logs subtracts the minute-of-hour a second time, so the window ends
// 2*minute minutes early and misses the most recent entries.
func TestLogsSearchDateDescending(t *testing.T) {

	forEachDatabase(t, func(t *testing.T, db *Database) {
		f := logFourEvents(t, db)
		date := f.last.Truncate(time.Minute).Add(time.Minute)
		res := f.search(t, db, &LogsSearchOptions{Date: date, Sort: -1})
		assertEqual(t, "count", res.Count, uint(4))
		if n := len(res.Logs); n == 0 || res.Logs[n-1].Message != "first info" {
			t.Errorf("messages: got %v, want the oldest (\"first info\") last", logMessages(res))
		}
	})
}

func TestLogsPrune(t *testing.T) {
	// Pin the zone to UTC: see TestLogsPruneIsTimeZoneIndependent.
	withLocalZone(t, time.UTC)
	forEachDatabase(t, func(t *testing.T, db *Database) {
		f := logFourEvents(t, db)

		must(t, f.logs.Prune(db, 1))
		assertEqual(t, "after prune(1)", f.search(t, db, NewLogSearchOptions()).Count, uint(4))

		// Let every event fall at least a whole second into the past.
		time.Sleep(2 * time.Second)
		must(t, f.logs.Prune(db, 0))
		assertEqual(t, "after prune(0)", f.search(t, db, NewLogSearchOptions()).Count, uint(0))
	})
}

// Same as TestCallsPruneIsTimeZoneIndependent, for logs.
func TestLogsPruneIsTimeZoneIndependent(t *testing.T) {

	withLocalZone(t, time.FixedZone("UTC-5", -5*3600))
	forEachDatabase(t, func(t *testing.T, db *Database) {
		logs := NewLogs()
		logs.setDatabase(db)
		must(t, logs.LogEvent(LogLevelInfo, "to be pruned"))
		time.Sleep(2 * time.Second)

		must(t, logs.Prune(db, 0))
		res, err := logs.Search(NewLogSearchOptions(), db)
		must(t, err)
		assertEqual(t, "count after prune(0)", res.Count, uint(0))
	})
}
