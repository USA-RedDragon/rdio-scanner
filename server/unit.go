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
	"sort"
	"sync"

	entunit "github.com/USA-RedDragon/rdio-scanner/server/ent/unit"
)

type Unit struct {
	Id    uint   `json:"id"`
	Label string `json:"label"`
	Order uint   `json:"order"`
}

func (unit *Unit) FromMap(m map[string]any) *Unit {
	switch v := m["id"].(type) {
	case float64:
		unit.Id = uint(v)
	}

	switch v := m["label"].(type) {
	case string:
		unit.Label = v
	}

	switch v := m["order"].(type) {
	case float64:
		unit.Order = uint(v)
	}

	return unit
}

type Units struct {
	List  []*Unit
	mutex sync.Mutex
}

func NewUnits() *Units {
	return &Units{
		List:  []*Unit{},
		mutex: sync.Mutex{},
	}
}

func (units *Units) Add(id uint, label string) (*Units, bool) {
	added := true

	for _, u := range units.List {
		if u.Id == id {
			added = false
			break
		}
	}

	if added {
		units.List = append(units.List, &Unit{Id: id, Label: label})
	}

	return units, added
}

func (units *Units) FromMap(f []any) *Units {
	units.mutex.Lock()
	defer units.mutex.Unlock()

	units.List = []*Unit{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			unit := &Unit{}
			unit.FromMap(m)
			units.List = append(units.List, unit)
		}
	}

	return units
}

func (u *Units) Merge(units *Units) bool {
	merged := false

	if units != nil {
		u.mutex.Lock()
		defer u.mutex.Unlock()

		for _, v := range units.List {
			if _, added := u.Add(v.Id, v.Label); added {
				merged = added
			}
		}
	}

	return merged
}

func (units *Units) Read(db *Database, systemId uint) error {
	units.mutex.Lock()
	defer units.mutex.Unlock()

	units.List = []*Unit{}

	records, err := db.Ent.Unit.Query().Where(entunit.SystemID(int(systemId))).All(context.Background())
	if err != nil {
		return fmt.Errorf("units.read: %w", err)
	}

	for _, r := range records {
		unit := &Unit{Id: uint(r.UnitID), Label: r.Label}
		if r.Order != nil && *r.Order > 0 {
			unit.Order = uint(*r.Order)
		}
		units.List = append(units.List, unit)
	}

	sort.Slice(units.List, func(i int, j int) bool {
		return units.List[i].Order < units.List[j].Order
	})

	return nil
}
func (units *Units) Write(db *Database, systemId uint) error {
	units.mutex.Lock()
	defer units.mutex.Unlock()

	ctx := context.Background()
	sysId := int(systemId)

	formatError := func(err error) error {
		return fmt.Errorf("units.write: %w", err)
	}

	keep := make([]int, 0, len(units.List))
	for _, unit := range units.List {
		keep = append(keep, int(unit.Id))
	}

	if _, err := db.Ent.Unit.Delete().Where(entunit.SystemID(sysId), entunit.UnitIDNotIn(keep...)).Exec(ctx); err != nil {
		return formatError(err)
	}

	for _, unit := range units.List {
		exists, err := db.Ent.Unit.Query().Where(entunit.SystemID(sysId), entunit.UnitID(int(unit.Id))).Exist(ctx)
		if err != nil {
			return formatError(err)
		}
		if exists {
			upd := db.Ent.Unit.Update().Where(entunit.SystemID(sysId), entunit.UnitID(int(unit.Id))).SetLabel(unit.Label)
			setOrClearInt(upd.SetOrder, upd.ClearOrder, nillablePosInt(unit.Order))
			if _, err := upd.Save(ctx); err != nil {
				return formatError(err)
			}
		} else {
			if err := db.Ent.Unit.Create().
				SetUnitID(int(unit.Id)).
				SetLabel(unit.Label).
				SetSystemID(sysId).
				SetNillableOrder(nillablePosInt(unit.Order)).
				Exec(ctx); err != nil {
				return formatError(err)
			}
		}
	}

	return nil
}
