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
	"sort"
	"testing"
)

// seededGroupAndTagIds returns the ids of two seeded groups and two seeded
// tags, for talkgroups to point at.
func seededGroupAndTagIds(t *testing.T, db *Database) (groupIds, tagIds [2]uint) {
	t.Helper()
	groups := NewGroups()
	must(t, groups.Read(db))
	tags := NewTags()
	must(t, tags.Read(db))
	if len(groups.List) < 2 || len(tags.List) < 2 {
		t.Fatalf("need 2 seeded groups and tags, got %d and %d", len(groups.List), len(tags.List))
	}
	for i := 0; i < 2; i++ {
		groupIds[i] = groups.List[i].Id.(uint)
		tagIds[i] = tags.List[i].Id.(uint)
	}
	return groupIds, tagIds
}

func systemIds(systems *Systems) []uint {
	ids := []uint{}
	for _, s := range systems.List {
		ids = append(ids, s.Id)
	}
	return ids
}

func mustGetSystem(t *testing.T, systems *Systems, id uint) *System {
	t.Helper()
	s, ok := systems.GetSystem(id)
	if !ok {
		t.Fatalf("system %d not found in %v", id, systemIds(systems))
	}
	return s
}

func mustGetTalkgroup(t *testing.T, system *System, id uint) *Talkgroup {
	t.Helper()
	tg, ok := system.Talkgroups.GetTalkgroup(id)
	if !ok {
		t.Fatalf("system %d: talkgroup %d not found in %v", system.Id, id, talkgroupIds(system.Talkgroups))
	}
	return tg
}

func talkgroupIds(talkgroups *Talkgroups) []uint {
	ids := []uint{}
	for _, tg := range talkgroups.List {
		ids = append(ids, tg.Id)
	}
	return ids
}

func sortedTalkgroupIds(talkgroups *Talkgroups) []uint {
	ids := talkgroupIds(talkgroups)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// assertTalkgroup compares the exported, persisted fields of a talkgroup.
func assertTalkgroup(t *testing.T, got, want *Talkgroup) {
	t.Helper()
	assertEqual(t, "talkgroup id", got.Id, want.Id)
	assertEqual(t, "talkgroup frequency", got.Frequency, want.Frequency)
	assertEqual(t, "talkgroup groupId", got.GroupId, want.GroupId)
	assertEqual(t, "talkgroup label", got.Label, want.Label)
	assertEqual(t, "talkgroup led", got.Led, want.Led)
	assertEqual(t, "talkgroup name", got.Name, want.Name)
	assertEqual(t, "talkgroup order", got.Order, want.Order)
	assertEqual(t, "talkgroup tagId", got.TagId, want.TagId)
}

// writeTwoSystems stores system 1 (two talkgroups, every optional field set)
// and system 2 (one bare talkgroup) and returns them as read back.
func writeTwoSystems(t *testing.T, db *Database) (*Systems, [2]uint, [2]uint) {
	t.Helper()
	g, tg := seededGroupAndTagIds(t, db)

	county := NewSystem()
	county.Id = 1
	county.AutoPopulate = true
	county.Blacklists = "100,200"
	county.Label = "County"
	county.Led = "red"
	county.Order = 2
	county.Talkgroups.List = []*Talkgroup{
		{Id: 10, Frequency: uint(851012500), GroupId: g[0], Label: "FD", Led: "blue", Name: "Fire Dispatch", Order: 2, TagId: tg[0]},
		{Id: 20, GroupId: g[1], Label: "PD", Name: "Police Dispatch", Order: 1, TagId: tg[1]},
	}

	state := NewSystem()
	state.Id = 2
	state.Label = "State"
	state.Order = 1
	state.Talkgroups.List = []*Talkgroup{
		{Id: 30, GroupId: g[0], Label: "SP", Name: "State Police", Order: 1, TagId: tg[0]},
	}

	systems := NewSystems()
	systems.List = []*System{county, state}
	must(t, systems.Write(db))

	systems = NewSystems()
	must(t, systems.Read(db))
	return systems, g, tg
}

func TestSystemsReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		systems := NewSystems()
		must(t, systems.Read(db))
		if len(systems.List) != 0 {
			t.Fatalf("fresh db: got %d systems, want 0", len(systems.List))
		}

		systems, g, tg := writeTwoSystems(t, db)

		// Read sorts systems by order.
		assertEqual(t, "system ids in order", systemIds(systems), []uint{2, 1})

		county := mustGetSystem(t, systems, 1)
		if county.RowId == nil {
			t.Error("county: no row id after read")
		}
		assertEqual(t, "county autoPopulate", county.AutoPopulate, true)
		assertEqual(t, "county blacklists", county.Blacklists, Blacklists("100,200"))
		if !county.Blacklists.IsBlacklisted(200) || county.Blacklists.IsBlacklisted(300) {
			t.Errorf("county blacklists do not match: %q", county.Blacklists)
		}
		assertEqual(t, "county label", county.Label, "County")
		assertEqual(t, "county led", county.Led, "red")
		assertEqual(t, "county order", county.Order, uint(2))
		// Read sorts talkgroups by order.
		assertEqual(t, "county talkgroups in order", talkgroupIds(county.Talkgroups), []uint{20, 10})
		assertTalkgroup(t, mustGetTalkgroup(t, county, 10), &Talkgroup{Id: 10, Frequency: uint(851012500), GroupId: g[0], Label: "FD", Led: "blue", Name: "Fire Dispatch", Order: 2, TagId: tg[0]})
		assertTalkgroup(t, mustGetTalkgroup(t, county, 20), &Talkgroup{Id: 20, GroupId: g[1], Label: "PD", Name: "Police Dispatch", Order: 1, TagId: tg[1]})
		if byLabel, ok := county.Talkgroups.GetTalkgroup("PD"); !ok || byLabel.Id != 20 {
			t.Errorf("GetTalkgroup by label: %+v %v", byLabel, ok)
		}

		state := mustGetSystem(t, systems, 2)
		if state.RowId == nil {
			t.Error("state: no row id after read")
		}
		assertEqual(t, "state autoPopulate", state.AutoPopulate, false)
		assertEqual(t, "state blacklists", state.Blacklists, Blacklists(""))
		assertEqual(t, "state label", state.Label, "State")
		assertEqual(t, "state led", state.Led, nil)
		assertEqual(t, "state order", state.Order, uint(1))
		assertTalkgroup(t, mustGetTalkgroup(t, state, 30), &Talkgroup{Id: 30, GroupId: g[0], Label: "SP", Name: "State Police", Order: 1, TagId: tg[0]})
		if s, ok := systems.GetSystem("State"); !ok || s.Id != 2 {
			t.Errorf("GetSystem by label: %+v %v", s, ok)
		}

		// Modify the county system and its talkgroups: change every system
		// field, change talkgroup 10, drop talkgroup 20, add talkgroup 40.
		countyRowId := county.RowId
		county.AutoPopulate = false
		county.Blacklists = "300"
		county.Label = "County 2"
		county.Led = "green"
		county.Order = 3
		fd := mustGetTalkgroup(t, county, 10)
		fd.Frequency = uint(852000000)
		fd.GroupId = g[1]
		fd.Label = "FD Main"
		fd.Led = "orange"
		fd.Name = "Fire Main"
		fd.Order = 5
		fd.TagId = tg[1]
		county.Talkgroups.List = []*Talkgroup{fd, {Id: 40, GroupId: g[0], Label: "EMS", Name: "EMS Dispatch", Order: 4, TagId: tg[0]}}
		must(t, systems.Write(db))

		systems = NewSystems()
		must(t, systems.Read(db))
		assertEqual(t, "system ids after update", systemIds(systems), []uint{2, 1})
		county = mustGetSystem(t, systems, 1)
		assertEqual(t, "county row id", county.RowId, countyRowId)
		assertEqual(t, "county autoPopulate after update", county.AutoPopulate, false)
		assertEqual(t, "county blacklists after update", county.Blacklists, Blacklists("300"))
		assertEqual(t, "county label after update", county.Label, "County 2")
		assertEqual(t, "county led after update", county.Led, "green")
		assertEqual(t, "county order after update", county.Order, uint(3))
		assertEqual(t, "county talkgroups after update", talkgroupIds(county.Talkgroups), []uint{40, 10})
		assertTalkgroup(t, mustGetTalkgroup(t, county, 10), &Talkgroup{Id: 10, Frequency: uint(852000000), GroupId: g[1], Label: "FD Main", Led: "orange", Name: "Fire Main", Order: 5, TagId: tg[1]})
		assertTalkgroup(t, mustGetTalkgroup(t, county, 40), &Talkgroup{Id: 40, GroupId: g[0], Label: "EMS", Name: "EMS Dispatch", Order: 4, TagId: tg[0]})
		state = mustGetSystem(t, systems, 2)
		assertEqual(t, "state talkgroups untouched", talkgroupIds(state.Talkgroups), []uint{30})

		// Remove the whole state system: its talkgroups must go with it.
		systems.List = []*System{county}
		must(t, systems.Write(db))
		systems = NewSystems()
		must(t, systems.Read(db))
		assertEqual(t, "system ids after remove", systemIds(systems), []uint{1})
		orphans := NewTalkgroups()
		must(t, orphans.Read(db, 2))
		assertEqual(t, "talkgroups of removed system", talkgroupIds(orphans), []uint{})

		// Re-creating a system with the same id starts with no talkgroups.
		reborn := NewSystem()
		reborn.Id = 2
		reborn.Label = "State Reborn"
		systems.List = append(systems.List, reborn)
		must(t, systems.Write(db))
		systems = NewSystems()
		must(t, systems.Read(db))
		assertEqual(t, "reborn talkgroups", talkgroupIds(mustGetSystem(t, systems, 2).Talkgroups), []uint{})
		assertEqual(t, "county talkgroups kept", sortedTalkgroupIds(mustGetSystem(t, systems, 1).Talkgroups), []uint{10, 40})
	})
}

// Removing an existing system and adding a new one in the same Write must do
// both, including dropping the removed system's talkgroups.
func TestSystemsWriteRemoveAndAddTogether(t *testing.T) {

	forEachDatabase(t, func(t *testing.T, db *Database) {
		systems, g, tg := writeTwoSystems(t, db)

		added := NewSystem()
		added.Id = 3
		added.Label = "New"
		added.Talkgroups.List = []*Talkgroup{{Id: 50, GroupId: g[0], Label: "N", Name: "New TG", TagId: tg[0]}}
		systems.List = []*System{mustGetSystem(t, systems, 1), added}
		must(t, systems.Write(db))

		systems = NewSystems()
		must(t, systems.Read(db))
		ids := systemIds(systems)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		assertEqual(t, "system ids", ids, []uint{1, 3})
		orphans := NewTalkgroups()
		must(t, orphans.Read(db, 2))
		assertEqual(t, "talkgroups of removed system", talkgroupIds(orphans), []uint{})
	})
}
