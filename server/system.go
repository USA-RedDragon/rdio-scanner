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
	"strconv"
	"strings"
	"sync"

	entsystem "github.com/USA-RedDragon/rdio-scanner/server/ent/system"
	enttalkgroup "github.com/USA-RedDragon/rdio-scanner/server/ent/talkgroup"
	entunit "github.com/USA-RedDragon/rdio-scanner/server/ent/unit"
)

type System struct {
	Id           uint        `json:"id"`
	AutoPopulate bool        `json:"autoPopulate"`
	Blacklists   Blacklists  `json:"blacklists"`
	Label        string      `json:"label"`
	Led          any         `json:"led"`
	Order        uint        `json:"order"`
	RowId        any         `json:"_id"`
	Talkgroups   *Talkgroups `json:"talkgroups"`
	Units        *Units      `json:"units"`
}

func NewSystem() *System {
	return &System{
		Talkgroups: NewTalkgroups(),
		Units:      NewUnits(),
	}
}

func (system *System) FromMap(m map[string]any) *System {
	switch v := m["_id"].(type) {
	case float64:
		system.RowId = uint(v)
	}

	switch v := m["id"].(type) {
	case float64:
		system.Id = uint(v)
	}

	switch v := m["autoPopulate"].(type) {
	case bool:
		system.AutoPopulate = v
	}

	switch v := m["blacklists"].(type) {
	case string:
		system.Blacklists = Blacklists(v)
	}

	switch v := m["label"].(type) {
	case string:
		system.Label = v
	}

	switch v := m["led"].(type) {
	case string:
		system.Led = v
	}

	switch v := m["order"].(type) {
	case float64:
		system.Order = uint(v)
	}

	switch v := m["talkgroups"].(type) {
	case []any:
		system.Talkgroups.FromMap(v)
	}

	switch v := m["units"].(type) {
	case []any:
		system.Units.FromMap(v)
	}

	return system
}

type SystemMap map[string]any

type Systems struct {
	List  []*System
	mutex sync.Mutex
}

func NewSystems() *Systems {
	return &Systems{
		List:  []*System{},
		mutex: sync.Mutex{},
	}
}

func (systems *Systems) FromMap(f []any) *Systems {
	systems.mutex.Lock()
	defer systems.mutex.Unlock()

	systems.List = []*System{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			system := NewSystem()
			system.FromMap(m)
			systems.List = append(systems.List, system)
		}
	}

	return systems
}

func (systems *Systems) GetNewSystemId() uint {
	systems.mutex.Lock()
	defer systems.mutex.Unlock()

NextId:
	for i := uint(1); i < 65535; i++ {
		for _, s := range systems.List {
			if s.Id == i {
				continue NextId
			}
		}
		return i
	}
	return 0
}

func (systems *Systems) GetSystem(f any) (system *System, ok bool) {
	systems.mutex.Lock()
	defer systems.mutex.Unlock()

	switch v := f.(type) {
	case uint:
		for _, system := range systems.List {
			if system.Id == v {
				return system, true
			}
		}
	case string:
		for _, system := range systems.List {
			if system.Label == v {
				return system, true
			}
		}
	}
	return nil, false
}

func (systems *Systems) GetScopedSystems(client *Client, groups *Groups, tags *Tags, sortTalkgroups bool) SystemsMap {
	var (
		rawSystems = []System{}
		systemsMap = SystemsMap{}
	)

	if client.Access == nil {
		for _, system := range systems.List {
			rawSystems = append(rawSystems, *system)
		}

	} else {
		switch v := client.Access.Systems.(type) {
		case nil:
			for _, system := range systems.List {
				rawSystems = append(rawSystems, *system)
			}

		case string:
			if v == "*" {
				for _, system := range systems.List {
					rawSystems = append(rawSystems, *system)
				}
			}

		case []any:
			for _, fSystem := range v {
				switch v := fSystem.(type) {
				case map[string]any:
					var (
						mSystemId   = v["id"]
						mTalkgroups = v["talkgroups"]
						systemId    uint
					)

					switch v := mSystemId.(type) {
					case float64:
						systemId = uint(v)
					default:
						continue
					}

					system, ok := systems.GetSystem(systemId)
					if !ok {
						continue
					}

					switch v := mTalkgroups.(type) {
					case string:
						if mTalkgroups == "*" {
							rawSystems = append(rawSystems, *system)
							continue
						}

					case []any:
						rawSystem := *system
						rawSystem.Talkgroups = NewTalkgroups()
						for _, fTalkgroupId := range v {
							switch v := fTalkgroupId.(type) {
							case float64:
								talkgroupId := uint(v)
								rawTalkgroup, ok := system.Talkgroups.GetTalkgroup(talkgroupId)
								if !ok {
									continue
								}
								rawSystem.Talkgroups.List = append(rawSystem.Talkgroups.List, rawTalkgroup)
							default:
								continue
							}
						}
						rawSystems = append(rawSystems, rawSystem)
					}
				}
			}
		}
	}

	for i, rawSystem := range rawSystems {
		talkgroupsMap := TalkgroupsMap{}

		if sortTalkgroups {
			sort.Slice(rawSystem.Talkgroups.List, func(i int, j int) bool {
				return rawSystem.Talkgroups.List[i].Label < rawSystem.Talkgroups.List[j].Label
			})
			for i := range rawSystem.Talkgroups.List {
				rawSystem.Talkgroups.List[i].Order = uint(i + 1)
			}
		}

		for j, rawTalkgroup := range rawSystem.Talkgroups.List {
			group, ok := groups.GetGroup(rawTalkgroup.GroupId)
			if !ok {
				continue
			}
			rawSystems[i].Talkgroups.List[j].group = group.Label

			tag, ok := tags.GetTag(rawTalkgroup.TagId)
			if !ok {
				continue
			}
			rawSystems[i].Talkgroups.List[j].tag = tag.Label

			talkgroupMap := TalkgroupMap{
				"id":    rawTalkgroup.Id,
				"group": group.Label,
				"label": rawTalkgroup.Label,
				"name":  rawTalkgroup.Name,
				"order": rawTalkgroup.Order,
				"tag":   tag.Label,
			}

			if rawTalkgroup.Frequency != nil {
				talkgroupMap["frequency"] = rawTalkgroup.Frequency
			}

			if rawTalkgroup.Led != nil {
				talkgroupMap["led"] = rawTalkgroup.Led
			}

			talkgroupsMap = append(talkgroupsMap, talkgroupMap)
		}

		sort.Slice(talkgroupsMap, func(i int, j int) bool {
			if a, err := strconv.Atoi(fmt.Sprintf("%v", talkgroupsMap[i]["order"])); err == nil {
				if b, err := strconv.Atoi(fmt.Sprintf("%v", talkgroupsMap[j]["order"])); err == nil {
					return a < b
				}
			}
			return false
		})

		systemMap := SystemMap{
			"id":         rawSystem.Id,
			"label":      rawSystem.Label,
			"order":      rawSystem.Order,
			"talkgroups": talkgroupsMap,
			"units":      rawSystem.Units.List,
		}

		if rawSystem.Led != nil {
			systemMap["led"] = rawSystem.Led
		}

		systemsMap = append(systemsMap, systemMap)
	}

	sort.Slice(systemsMap, func(i int, j int) bool {
		if a, err := strconv.Atoi(fmt.Sprintf("%v", systemsMap[i]["order"])); err == nil {
			if b, err := strconv.Atoi(fmt.Sprintf("%v", systemsMap[j]["order"])); err == nil {
				return a < b
			}
		}
		return false
	})

	return systemsMap
}

func (systems *Systems) Read(db *Database) error {
	systems.mutex.Lock()
	defer systems.mutex.Unlock()

	systems.List = []*System{}

	records, err := db.Ent.System.Query().All(context.Background())
	if err != nil {
		return fmt.Errorf("systems.read: %w", err)
	}

	for _, r := range records {
		system := &System{
			Id:           uint(r.SystemID),
			Label:        r.Label,
			RowId:        uint(r.ID),
			Talkgroups:   NewTalkgroups(),
			Units:        NewUnits(),
			AutoPopulate: r.AutoPopulate != nil && *r.AutoPopulate,
		}

		if len(r.Blacklists) > 0 {
			inner := strings.ReplaceAll(r.Blacklists, "[", "")
			inner = strings.ReplaceAll(inner, "]", "")
			system.Blacklists = Blacklists(inner)
		}

		if r.Led != nil && len(*r.Led) > 0 {
			system.Led = *r.Led
		}

		if r.Order != nil && *r.Order > 0 {
			system.Order = uint(*r.Order)
		}

		if err = system.Talkgroups.Read(db, system.Id); err != nil {
			return err
		}

		if err = system.Units.Read(db, system.Id); err != nil {
			return err
		}

		systems.List = append(systems.List, system)
	}

	sort.Slice(systems.List, func(i int, j int) bool {
		return systems.List[i].Order < systems.List[j].Order
	})

	return nil
}
func (systems *Systems) Write(db *Database) error {
	systems.mutex.Lock()
	defer systems.mutex.Unlock()

	ctx := context.Background()

	formatError := func(err error) error {
		return fmt.Errorf("systems.write: %w", err)
	}

	// Ids (row _id) still present, and the natural system ids being removed so
	// their talkgroups and units can be cascaded.
	keep := make([]int, 0, len(systems.List))
	for _, system := range systems.List {
		if id, ok := rowID(system.RowId); ok {
			keep = append(keep, id)
		}
	}

	existing, err := db.Ent.System.Query().All(ctx)
	if err != nil {
		return formatError(err)
	}
	keepSet := make(map[int]bool, len(keep))
	for _, id := range keep {
		keepSet[id] = true
	}
	removedSystemIds := []int{}
	for _, r := range existing {
		if !keepSet[r.ID] {
			removedSystemIds = append(removedSystemIds, r.SystemID)
		}
	}

	if len(removedSystemIds) > 0 {
		if _, err = db.Ent.Talkgroup.Delete().Where(enttalkgroup.SystemIDIn(removedSystemIds...)).Exec(ctx); err != nil {
			return formatError(err)
		}
		if _, err = db.Ent.Unit.Delete().Where(entunit.SystemIDIn(removedSystemIds...)).Exec(ctx); err != nil {
			return formatError(err)
		}
	}

	if _, err = db.Ent.System.Delete().Where(entsystem.IDNotIn(keep...)).Exec(ctx); err != nil {
		return formatError(err)
	}

	for _, system := range systems.List {
		blacklists := "[]"
		if len(system.Blacklists) > 0 {
			blacklists = "[" + system.Blacklists.String() + "]"
		}

		if id, ok := rowID(system.RowId); ok {
			upd := db.Ent.System.UpdateOneID(id).
				SetSystemID(int(system.Id)).
				SetAutoPopulate(system.AutoPopulate).
				SetBlacklists(blacklists).
				SetLabel(system.Label)
			setOrClearStr(upd.SetLed, upd.ClearLed, nillableStr(system.Led))
			setOrClearInt(upd.SetOrder, upd.ClearOrder, nillablePosInt(system.Order))
			if err = upd.Exec(ctx); err != nil {
				return formatError(err)
			}
		} else {
			if err = db.Ent.System.Create().
				SetSystemID(int(system.Id)).
				SetAutoPopulate(system.AutoPopulate).
				SetBlacklists(blacklists).
				SetLabel(system.Label).
				SetNillableLed(nillableStr(system.Led)).
				SetNillableOrder(nillablePosInt(system.Order)).
				Exec(ctx); err != nil {
				return formatError(err)
			}
		}

		if err = system.Talkgroups.Write(db, system.Id); err != nil {
			return err
		}

		if err = system.Units.Write(db, system.Id); err != nil {
			return err
		}
	}

	return nil
}

type SystemsMap []SystemMap
