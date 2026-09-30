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
	"encoding/json"
	"fmt"
	"sync"

	entapikey "github.com/USA-RedDragon/rdio-scanner/server/ent/apikey"
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
	apikeys.mutex.Lock()
	defer apikeys.mutex.Unlock()

	apikeys.List = []*Apikey{}

	records, err := db.Ent.Apikey.Query().All(context.Background())
	if err != nil {
		return fmt.Errorf("apikeys.read: %w", err)
	}

	for _, r := range records {
		apikey := &Apikey{Id: uint(r.ID), Key: r.Key}

		apikey.Disabled = r.Disabled != nil && *r.Disabled

		if r.Ident != nil && len(*r.Ident) > 0 {
			apikey.Ident = *r.Ident
		} else {
			apikey.Ident = defaults.apikey.ident
		}

		if len(apikey.Key) == 0 {
			apikey.Key = uuid.New().String()
		}

		if r.Order != nil && *r.Order > 0 {
			apikey.Order = uint(*r.Order)
		}

		if err = json.Unmarshal([]byte(r.Systems), &apikey.Systems); err != nil {
			apikey.Systems = []any{}
		}

		apikeys.List = append(apikeys.List, apikey)
	}

	return nil
}

func (apikeys *Apikeys) Write(db *Database) error {
	apikeys.mutex.Lock()
	defer apikeys.mutex.Unlock()

	ctx := context.Background()

	formatError := func(err error) error {
		return fmt.Errorf("apikeys.write: %w", err)
	}

	keep := make([]int, 0, len(apikeys.List))
	for _, apikey := range apikeys.List {
		if id, ok := rowID(apikey.Id); ok {
			keep = append(keep, id)
		}
	}

	if _, err := db.Ent.Apikey.Delete().Where(entapikey.IDNotIn(keep...)).Exec(ctx); err != nil {
		return formatError(err)
	}

	for _, apikey := range apikeys.List {
		if id, ok := rowID(apikey.Id); ok {
			upd := db.Ent.Apikey.UpdateOneID(id).
				SetDisabled(apikey.Disabled).
				SetIdent(apikey.Ident).
				SetKey(apikey.Key).
				SetSystems(systemsText(apikey.Systems))
			if n := nillablePosInt(apikey.Order); n != nil {
				upd.SetOrder(*n)
			} else {
				upd.ClearOrder()
			}
			if err := upd.Exec(ctx); err != nil {
				return formatError(err)
			}
		} else {
			if err := db.Ent.Apikey.Create().
				SetDisabled(apikey.Disabled).
				SetIdent(apikey.Ident).
				SetKey(apikey.Key).
				SetSystems(systemsText(apikey.Systems)).
				SetNillableOrder(nillablePosInt(apikey.Order)).
				Exec(ctx); err != nil {
				return formatError(err)
			}
		}
	}

	return nil
}
