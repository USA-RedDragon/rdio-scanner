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

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/USA-RedDragon/rdio-scanner/server/ent"
	entlogentry "github.com/USA-RedDragon/rdio-scanner/server/ent/logentry"
	"github.com/USA-RedDragon/rdio-scanner/server/ent/predicate"
)

const (
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

type Log struct {
	Id       any       `json:"_id"`
	DateTime time.Time `json:"dateTime"`
	Level    string    `json:"level"`
	Message  string    `json:"message"`
}

type Logs struct {
	database *Database
	mutex    sync.Mutex
	daemon   *Daemon
}

func NewLogs() *Logs {
	return &Logs{
		mutex: sync.Mutex{},
	}
}

func (logs *Logs) LogEvent(level string, message string) error {
	logs.mutex.Lock()
	defer logs.mutex.Unlock()

	if logs.daemon != nil {
		switch level {
		case LogLevelError:
			logs.daemon.Logger.Error(message)
		case LogLevelWarn:
			logs.daemon.Logger.Warning(message)
		case LogLevelInfo:
			logs.daemon.Logger.Info(message)
		}

	} else {
		log.Println(message)
	}

	if logs.database != nil {
		l := Log{
			DateTime: time.Now().UTC(),
			Level:    level,
			Message:  message,
		}

		if err := logs.database.Ent.LogEntry.Create().SetDateTime(l.DateTime).SetLevel(l.Level).SetMessage(l.Message).Exec(context.Background()); err != nil {
			return fmt.Errorf("logs.logevent: %w", err)
		}
	}

	return nil
}

func (logs *Logs) Prune(db *Database, pruneDays uint) error {
	logs.mutex.Lock()
	defer logs.mutex.Unlock()

	cutoff := time.Now().UTC().Add(-24 * time.Hour * time.Duration(pruneDays))
	_, err := db.Ent.LogEntry.Delete().Where(entlogentry.DateTimeLT(cutoff)).Exec(context.Background())

	return err
}

func (logs *Logs) Search(searchOptions *LogsSearchOptions, db *Database) (*LogsSearchResults, error) {
	const (
		ascOrder  = 1
		descOrder = -1
	)

	ctx := context.Background()

	logs.mutex.Lock()
	defer logs.mutex.Unlock()

	formatError := func(err error) error {
		return fmt.Errorf("logs.search: %w", err)
	}

	logResults := &LogsSearchResults{
		Options: searchOptions,
		Logs:    []Log{},
	}

	sort := ascOrder
	if v, ok := searchOptions.Sort.(int); ok && v < 0 {
		sort = descOrder
	}

	preds := []predicate.LogEntry{}

	if level, ok := searchOptions.Level.(string); ok {
		preds = append(preds, entlogentry.Level(level))
	}

	if v, ok := searchOptions.Date.(time.Time); ok {
		minute := v.UTC().Truncate(time.Minute)
		var start, stop time.Time
		if sort == ascOrder {
			// 24h window starting at the selected minute.
			start = minute
			stop = minute.Add(24 * time.Hour)
		} else {
			// 24h window ending at the end of the selected minute.
			stop = minute.Add(time.Minute)
			start = stop.Add(-24 * time.Hour)
		}
		preds = append(preds, entlogentry.DateTimeGTE(start), entlogentry.DateTimeLT(stop))
	}

	limit := 200
	if v, ok := searchOptions.Limit.(uint); ok {
		if v > 500 {
			limit = 500
		} else {
			limit = int(v)
		}
	}

	offset := 0
	if v, ok := searchOptions.Offset.(uint); ok {
		offset = int(v)
	}

	count, err := db.Ent.LogEntry.Query().Where(preds...).Count(ctx)
	if err != nil {
		return nil, formatError(err)
	}
	logResults.Count = uint(count)

	if first, err := db.Ent.LogEntry.Query().Where(preds...).Order(ent.Asc(entlogentry.FieldDateTime)).First(ctx); err == nil {
		logResults.DateStart = first.DateTime.UTC()
	} else if !ent.IsNotFound(err) {
		return nil, formatError(err)
	}

	if last, err := db.Ent.LogEntry.Query().Where(preds...).Order(ent.Desc(entlogentry.FieldDateTime)).First(ctx); err == nil {
		logResults.DateStop = last.DateTime.UTC()
	} else if !ent.IsNotFound(err) {
		return nil, formatError(err)
	}

	order := ent.Asc(entlogentry.FieldDateTime)
	if sort == descOrder {
		order = ent.Desc(entlogentry.FieldDateTime)
	}

	records, err := db.Ent.LogEntry.Query().Where(preds...).Order(order).Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, formatError(err)
	}

	for _, r := range records {
		logResults.Logs = append(logResults.Logs, Log{
			Id:       uint(r.ID),
			DateTime: r.DateTime.UTC(),
			Level:    r.Level,
			Message:  r.Message,
		})
	}

	return logResults, nil
}

func (logs *Logs) setDaemon(d *Daemon) {
	logs.daemon = d
}

func (logs *Logs) setDatabase(d *Database) {
	logs.database = d
}

type LogsSearchOptions struct {
	Date   any `json:"date,omitempty"`
	Level  any `json:"level,omitempty"`
	Limit  any `json:"limit,omitempty"`
	Offset any `json:"offset,omitempty"`
	Sort   any `json:"sort,omitempty"`
}

func NewLogSearchOptions() *LogsSearchOptions {
	return &LogsSearchOptions{}
}

func (searchOptions *LogsSearchOptions) FromMap(m map[string]any) *LogsSearchOptions {
	switch v := m["date"].(type) {
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			searchOptions.Date = t
		}
	}

	switch v := m["level"].(type) {
	case string:
		searchOptions.Level = v
	}

	switch v := m["limit"].(type) {
	case float64:
		searchOptions.Limit = uint(v)
	}

	switch v := m["offset"].(type) {
	case float64:
		searchOptions.Offset = uint(v)
	}

	switch v := m["sort"].(type) {
	case float64:
		searchOptions.Sort = int(v)
	}

	return searchOptions
}

type LogsSearchResults struct {
	Count     uint               `json:"count"`
	DateStart time.Time          `json:"dateStart"`
	DateStop  time.Time          `json:"dateStop"`
	Options   *LogsSearchOptions `json:"options"`
	Logs      []Log              `json:"logs"`
}
