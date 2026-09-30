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
	"time"

	"github.com/USA-RedDragon/rdio-scanner/server/ent"
	entaccess "github.com/USA-RedDragon/rdio-scanner/server/ent/access"
)

type Access struct {
	Id         any    `json:"_id"`
	Code       string `json:"code"`
	Expiration any    `json:"expiration"`
	Ident      string `json:"ident"`
	Limit      any    `json:"limit"`
	Order      any    `json:"order"`
	Systems    any    `json:"systems"`
}

func NewAccess() *Access {
	return &Access{Systems: "*"}
}

func (access *Access) FromMap(m map[string]any) *Access {
	switch v := m["_id"].(type) {
	case float64:
		access.Id = uint(v)
	}

	switch v := m["code"].(type) {
	case string:
		access.Code = v
	}

	switch v := m["expiration"].(type) {
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			access.Expiration = t.UTC()
		}
	}

	switch v := m["ident"].(type) {
	case string:
		access.Ident = v
	}

	switch v := m["limit"].(type) {
	case float64:
		access.Limit = uint(v)
	}

	switch v := m["order"].(type) {
	case float64:
		access.Order = uint(v)
	}

	switch v := m["systems"].(type) {
	case []any:
		if b, err := json.Marshal(v); err == nil {
			access.Systems = string(b)
		}
	case string:
		access.Systems = v
	}

	return access
}

func (access *Access) HasAccess(call *Call) bool {
	if access.Systems != nil {
		switch v := access.Systems.(type) {
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
	}

	return false
}

func (access *Access) HasExpired() bool {
	switch v := access.Expiration.(type) {
	case time.Time:
		return v.Before(time.Now())
	}
	return false
}

type Accesses struct {
	List  []*Access
	mutex sync.Mutex
}

func NewAccesses() *Accesses {
	return &Accesses{
		List:  []*Access{},
		mutex: sync.Mutex{},
	}
}

func (accesses *Accesses) Add(access *Access) (*Accesses, bool) {
	accesses.mutex.Lock()
	defer accesses.mutex.Unlock()

	added := true

	for _, a := range accesses.List {
		if a.Code == access.Code {
			a.Expiration = access.Expiration
			a.Ident = access.Ident
			a.Limit = access.Limit
			a.Systems = access.Systems
			added = false
		}
	}

	if added {
		accesses.List = append(accesses.List, access)
	}

	return accesses, added
}

func (accesses *Accesses) FromMap(f []any) *Accesses {
	accesses.mutex.Lock()
	defer accesses.mutex.Unlock()

	accesses.List = []*Access{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			access := &Access{}
			access.FromMap(m)
			accesses.List = append(accesses.List, access)
		}
	}

	return accesses
}

func (accesses *Accesses) GetAccess(code string) (access *Access, ok bool) {
	accesses.mutex.Lock()
	defer accesses.mutex.Unlock()

	for _, access := range accesses.List {
		if access.Code == code {
			return access, true
		}
	}

	return nil, false
}

func (accesses *Accesses) IsRestricted() bool {
	accesses.mutex.Lock()
	defer accesses.mutex.Unlock()

	return len(accesses.List) > 0
}

func (accesses *Accesses) Read(db *Database) error {
	accesses.mutex.Lock()
	defer accesses.mutex.Unlock()

	accesses.List = []*Access{}

	records, err := db.Ent.Access.Query().All(context.Background())
	if err != nil {
		return fmt.Errorf("accesses.read: %w", err)
	}

	for _, r := range records {
		if len(r.Code) == 0 {
			continue
		}

		access := &Access{Id: uint(r.ID), Code: r.Code}

		if r.Expiration != nil {
			access.Expiration = r.Expiration.UTC()
		}

		if r.Ident != nil && len(*r.Ident) > 0 {
			access.Ident = *r.Ident
		} else {
			access.Ident = defaults.access.ident
		}

		if r.Limit != nil && *r.Limit > 0 {
			access.Limit = uint(*r.Limit)
		}

		if r.Order != nil && *r.Order > 0 {
			access.Order = uint(*r.Order)
		}

		if err = json.Unmarshal([]byte(r.Systems), &access.Systems); err != nil {
			access.Systems = []any{}
		}

		accesses.List = append(accesses.List, access)
	}

	return nil
}

func (accesses *Accesses) Remove(access *Access) (*Accesses, bool) {
	accesses.mutex.Lock()
	defer accesses.mutex.Unlock()

	removed := false

	for i, a := range accesses.List {
		if a.Ident == access.Ident {
			accesses.List = append(accesses.List[:i], accesses.List[i+1:]...)
			removed = true
		}
	}

	return accesses, removed
}

func (accesses *Accesses) Write(db *Database) error {
	accesses.mutex.Lock()
	defer accesses.mutex.Unlock()

	ctx := context.Background()

	formatError := func(err error) error {
		return fmt.Errorf("accesses.write: %w", err)
	}

	keep := make([]int, 0, len(accesses.List))
	for _, access := range accesses.List {
		if id, ok := rowID(access.Id); ok {
			keep = append(keep, id)
		}
	}

	if _, err := db.Ent.Access.Delete().Where(entaccess.IDNotIn(keep...)).Exec(ctx); err != nil {
		return formatError(err)
	}

	for _, access := range accesses.List {
		if id, ok := rowID(access.Id); ok {
			upd := db.Ent.Access.UpdateOneID(id).
				SetCode(access.Code).
				SetIdent(access.Ident).
				SetSystems(systemsText(access.Systems))
			applyAccessExpirationLimitOrder(upd, access)
			if err := upd.Exec(ctx); err != nil {
				return formatError(err)
			}
		} else {
			create := db.Ent.Access.Create().
				SetCode(access.Code).
				SetIdent(access.Ident).
				SetSystems(systemsText(access.Systems)).
				SetNillableExpiration(nillableTime(access.Expiration)).
				SetNillableLimit(nillablePosInt(access.Limit)).
				SetNillableOrder(nillablePosInt(access.Order))
			if err := create.Exec(ctx); err != nil {
				return formatError(err)
			}
		}
	}

	return nil
}

// applyAccessExpirationLimitOrder sets or clears the nullable columns on an
// update (SetNillable leaves a nil pointer unchanged, so clearing needs Clear).
func applyAccessExpirationLimitOrder(upd *ent.AccessUpdateOne, access *Access) {
	if t := nillableTime(access.Expiration); t != nil {
		upd.SetExpiration(*t)
	} else {
		upd.ClearExpiration()
	}
	if n := nillablePosInt(access.Limit); n != nil {
		upd.SetLimit(*n)
	} else {
		upd.ClearLimit()
	}
	if n := nillablePosInt(access.Order); n != nil {
		upd.SetOrder(*n)
	} else {
		upd.ClearOrder()
	}
}
