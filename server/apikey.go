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
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Apikey struct {
	Id       any    `json:"_id"`
	Disabled bool   `json:"disabled"`
	Ident    string `json:"ident"`
	Key      string `json:"key"`
	Order    any    `json:"order"`
	Systems  any    `json:"systems"`
}

func (apikey *Apikey) FromMap(m map[string]any) *Apikey {
	switch v := m["_id"].(type) {
	case float64:
		apikey.Id = uint(v)
	}

	switch v := m["disabled"].(type) {
	case bool:
		apikey.Disabled = v
	}

	switch v := m["ident"].(type) {
	case string:
		apikey.Ident = v
	}

	switch v := m["key"].(type) {
	case string:
		apikey.Key = v
	}

	switch v := m["order"].(type) {
	case float64:
		apikey.Order = uint(v)
	}

	switch v := m["systems"].(type) {
	case []any:
		if b, err := json.Marshal(v); err == nil {
			apikey.Systems = string(b)
		}
	case string:
		apikey.Systems = v
	}

	return apikey
}

func (apikey *Apikey) HasAccess(call *Call) bool {
	switch v := apikey.Systems.(type) {
	case []any:
		for _, f := range v {
			switch v := f.(type) {
			case map[string]any:
				switch id := v["id"].(type) {
				case float64:
					if id == float64(call.System) {
						switch tg := v["talkgroups"].(type) {
						case string:
							if tg == "*" {
								return true
							}
						case []any:
							for _, f := range tg {
								switch tg := f.(type) {
								case float64:
									if tg == float64(call.Talkgroup) {
										return true
									}
								}
							}
						}
					}
				}
			}
		}

	case string:
		if v == "*" {
			return true
		}
	}

	return false
}

type Apikeys struct {
	List  []*Apikey
	mutex sync.Mutex
}

func NewApikeys() *Apikeys {
	return &Apikeys{
		List:  []*Apikey{},
		mutex: sync.Mutex{},
	}
}

func (apikeys *Apikeys) FromMap(f []any) *Apikeys {
	apikeys.mutex.Lock()
	defer apikeys.mutex.Unlock()

	apikeys.List = []*Apikey{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			apikey := &Apikey{}
			apikey.FromMap(m)
			apikeys.List = append(apikeys.List, apikey)
		}
	}

	return apikeys
}

func (apikeys *Apikeys) GetApikey(key string) (apikey *Apikey, ok bool) {
	apikeys.mutex.Lock()
	defer apikeys.mutex.Unlock()

	for _, apikey := range apikeys.List {
		if apikey.Key == key && !apikey.Disabled {
			return apikey, true
		}
	}
	return nil, false
}

func (apikeys *Apikeys) Read(db *Database) error {
	var (
		err     error
		id      sql.NullFloat64
		order   sql.NullFloat64
		rows    *sql.Rows
		systems string
	)

	apikeys.mutex.Lock()
	defer apikeys.mutex.Unlock()

	apikeys.List = []*Apikey{}

	formatError := func(err error) error {
		return fmt.Errorf("apikeys.read: %v", err)
	}

	q := "select `_id`, `disabled`, `ident`, `key`, `order`, `systems` from `rdioScannerApiKeys`"
	if db.Config.DbType == DbTypePostgresql {
		q = "select _id, disabled, ident, key, \"order\", systems from rdioScannerApiKeys"
	}
	if rows, err = db.Sql.Query(q); err != nil {
		return formatError(err)
	}

	for rows.Next() {
		apikey := &Apikey{}

		if err = rows.Scan(&id, &apikey.Disabled, &apikey.Ident, &apikey.Key, &order, &systems); err != nil {
			break
		}

		if id.Valid && id.Float64 > 0 {
			apikey.Id = uint(id.Float64)
		}

		if len(apikey.Ident) == 0 {
			apikey.Ident = defaults.apikey.ident
		}

		if len(apikey.Key) == 0 {
			apikey.Key = uuid.New().String()
		}

		if order.Valid && order.Float64 > 0 {
			apikey.Order = uint(order.Float64)
		}

		if err = json.Unmarshal([]byte(systems), &apikey.Systems); err != nil {
			apikey.Systems = []any{}
		}

		apikeys.List = append(apikeys.List, apikey)
	}

	rows.Close()

	if err != nil {
		return formatError(err)
	}

	return nil
}

func (apikeys *Apikeys) Write(db *Database) error {
	var (
		count   uint
		err     error
		rows    *sql.Rows
		rowIds  = []uint{}
		systems any
	)

	apikeys.mutex.Lock()
	defer apikeys.mutex.Unlock()

	formatError := func(err error) error {
		return fmt.Errorf("apikeys.write %v", err)
	}

	q := "select `_id` from `rdioScannerApiKeys`"
	if db.Config.DbType == DbTypePostgresql {
		q = "select _id from rdioScannerApiKeys"
	}
	if rows, err = db.Sql.Query(q); err != nil {
		return formatError(err)
	}

	for rows.Next() {
		var id uint
		if err = rows.Scan(&id); err != nil {
			break
		}
		remove := true
		for _, apikey := range apikeys.List {
			if apikey.Id == nil || apikey.Id == id {
				remove = false
				break
			}
		}
		if remove {
			rowIds = append(rowIds, id)
		}
	}

	rows.Close()

	if err != nil {
		return formatError(err)
	}

	if len(rowIds) > 0 {
		if b, err := json.Marshal(rowIds); err == nil {
			s := string(b)
			s = strings.ReplaceAll(s, "[", "(")
			s = strings.ReplaceAll(s, "]", ")")
			q := fmt.Sprintf("delete from `rdioScannerApikeys` where `_id` in %v", s)
			if db.Config.DbType == DbTypePostgresql {
				q = fmt.Sprintf("delete from rdioScannerApikeys where _id in %v", s)
			}
			if _, err = db.Sql.Exec(q); err != nil {
				return formatError(err)
			}
		}
	}

	for _, apikey := range apikeys.List {
		switch apikey.Systems {
		case "*":
			systems = `"*"`
		default:
			systems = apikey.Systems
		}

		q := "select count(*) from `rdioScannerApiKeys` where `_id` = ?"
		if db.Config.DbType == DbTypePostgresql {
			q = "select count(*) from rdioScannerApiKeys where _id = $1"
		}
		if err = db.Sql.QueryRow(q, apikey.Id).Scan(&count); err != nil {
			break
		}

		if count == 0 {
			if db.Config.DbType == DbTypePostgresql {
				q = "insert into rdioScannerApiKeys (disabled, ident, key, \"order\", systems) values ($1, $2, $3, $4, $5)"
				if _, err = db.Sql.Exec(q, apikey.Disabled, apikey.Ident, apikey.Key, apikey.Order, systems); err != nil {
					break
				}
			} else {
				q = "insert into `rdioScannerApiKeys` (`_id`, `disabled`, `ident`, `key`, `order`, `systems`) values (?, ?, ?, ?, ?, ?)"
				if _, err = db.Sql.Exec(q, apikey.Id, apikey.Disabled, apikey.Ident, apikey.Key, apikey.Order, systems); err != nil {
					break
				}
			}
		} else {
			q := "update `rdioScannerApiKeys` set `_id` = ?, `disabled` = ?, `ident` = ?, `key` = ?, `order` = ?, `systems` = ? where `_id` = ?"
			if db.Config.DbType == DbTypePostgresql {
				q = "update rdioScannerApiKeys set _id = $1, disabled = $2, ident = $3, key = $4, \"order\" = $5, systems = $6 where _id = $7"
			}
			if _, err = db.Sql.Exec(q, apikey.Id, apikey.Disabled, apikey.Ident, apikey.Key, apikey.Order, systems, apikey.Id); err != nil {
				break
			}
		}
	}

	if err != nil {
		return formatError(err)
	}

	return nil
}
