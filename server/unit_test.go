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

import "testing"

func unitSummaries(units *Units) []Unit {
	out := []Unit{}
	for _, u := range units.List {
		out = append(out, *u)
	}
	return out
}

// writeSystemsWithUnits stores system 1 with two units and system 2 with no
// units (so later writes of system 2 never update a unit row).
func writeSystemsWithUnits(t *testing.T, db *Database) *Systems {
	t.Helper()
	g, tg := seededGroupAndTagIds(t, db)

	withUnits := NewSystem()
	withUnits.Id = 1
	withUnits.Label = "Trunked"
	withUnits.Order = 1
	withUnits.Talkgroups.List = []*Talkgroup{{Id: 10, GroupId: g[0], Label: "TG10", Name: "Talkgroup 10", Order: 1, TagId: tg[0]}}
	withUnits.Units.List = []*Unit{
		{Id: 1002, Label: "Ladder 2", Order: 2},
		{Id: 1001, Label: "Engine 1", Order: 1},
	}

	noUnits := NewSystem()
	noUnits.Id = 2
	noUnits.Label = "Conventional"
	noUnits.Order = 2

	systems := NewSystems()
	systems.List = []*System{withUnits, noUnits}
	must(t, systems.Write(db))

	systems = NewSystems()
	must(t, systems.Read(db))
	return systems
}

func TestUnitsReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		systems := writeSystemsWithUnits(t, db)

		// Read sorts units by order.
		assertEqual(t, "system 1 units", unitSummaries(mustGetSystem(t, systems, 1).Units), []Unit{
			{Id: 1001, Label: "Engine 1", Order: 1},
			{Id: 1002, Label: "Ladder 2", Order: 2},
		})
		assertEqual(t, "system 2 units", unitSummaries(mustGetSystem(t, systems, 2).Units), []Unit{})

		direct := NewUnits()
		must(t, direct.Read(db, 1))
		assertEqual(t, "units read directly", len(direct.List), 2)

		// Remove the whole system with units: its units and talkgroups go too.
		systems.List = []*System{mustGetSystem(t, systems, 2)}
		must(t, systems.Write(db))
		systems = NewSystems()
		must(t, systems.Read(db))
		assertEqual(t, "systems after remove", systemIds(systems), []uint{2})
		orphans := NewUnits()
		must(t, orphans.Read(db, 1))
		assertEqual(t, "units of removed system", unitSummaries(orphans), []Unit{})
		orphanTgs := NewTalkgroups()
		must(t, orphanTgs.Read(db, 1))
		assertEqual(t, "talkgroups of removed system", talkgroupIds(orphanTgs), []uint{})
	})
}

// Updating existing units (label/order), removing one and adding one.
func TestUnitsUpdate(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		if db.Config.DbType == DbTypePostgresql {
		}

		systems := writeSystemsWithUnits(t, db)
		system := mustGetSystem(t, systems, 1)

		var engine *Unit
		for _, u := range system.Units.List {
			if u.Id == 1001 {
				engine = u
			}
		}
		engine.Label = "Engine 1 (reserve)"
		engine.Order = 5
		system.Units.List = []*Unit{engine, {Id: 1003, Label: "Medic 3", Order: 3}}
		must(t, systems.Write(db))

		systems = NewSystems()
		must(t, systems.Read(db))
		assertEqual(t, "units after update", unitSummaries(mustGetSystem(t, systems, 1).Units), []Unit{
			{Id: 1003, Label: "Medic 3", Order: 3},
			{Id: 1001, Label: "Engine 1 (reserve)", Order: 5},
		})
	})
}

// Units.Merge (used by auto-populate) followed by a Write persists new units
// without disturbing existing ones.
func TestUnitsMergeThenWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		if db.Config.DbType == DbTypePostgresql {
		}

		systems := writeSystemsWithUnits(t, db)
		system := mustGetSystem(t, systems, 1)

		incoming := NewUnits()
		incoming.List = []*Unit{{Id: 1001, Label: "ignored, already known"}, {Id: 1004, Label: "Chief 4"}}
		if !system.Units.Merge(incoming) {
			t.Fatal("Merge reported nothing merged")
		}
		must(t, systems.Write(db))

		units := NewUnits()
		must(t, units.Read(db, 1))
		got := map[uint]string{}
		for _, u := range units.List {
			got[u.Id] = u.Label
		}
		assertEqual(t, "units after merge", got, map[uint]string{1001: "Engine 1", 1002: "Ladder 2", 1004: "Chief 4"})
	})
}
