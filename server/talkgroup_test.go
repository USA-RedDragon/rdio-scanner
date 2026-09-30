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

// Talkgroups are keyed by (systemId, id): the same talkgroup id in two
// systems is two independent rows, and writing one system's list never
// touches the other's.
func TestTalkgroupsPerSystem(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		g, tg := seededGroupAndTagIds(t, db)

		a := NewTalkgroups()
		a.List = []*Talkgroup{
			{Id: 100, GroupId: g[0], Label: "A100", Name: "System A 100", Order: 1, TagId: tg[0]},
			{Id: 200, GroupId: g[0], Label: "A200", Name: "System A 200", Order: 2, TagId: tg[0]},
		}
		must(t, a.Write(db, 7))

		b := NewTalkgroups()
		b.List = []*Talkgroup{
			{Id: 100, Frequency: uint(460000000), GroupId: g[1], Label: "B100", Led: "cyan", Name: "System B 100", Order: 1, TagId: tg[1]},
		}
		must(t, b.Write(db, 8))

		a = NewTalkgroups()
		must(t, a.Read(db, 7))
		assertEqual(t, "system 7 talkgroups", talkgroupIds(a), []uint{100, 200})
		got, _ := a.GetTalkgroup(uint(100))
		assertTalkgroup(t, got, &Talkgroup{Id: 100, GroupId: g[0], Label: "A100", Name: "System A 100", Order: 1, TagId: tg[0]})

		b = NewTalkgroups()
		must(t, b.Read(db, 8))
		assertEqual(t, "system 8 talkgroups", talkgroupIds(b), []uint{100})
		got, _ = b.GetTalkgroup(uint(100))
		assertTalkgroup(t, got, &Talkgroup{Id: 100, Frequency: uint(460000000), GroupId: g[1], Label: "B100", Led: "cyan", Name: "System B 100", Order: 1, TagId: tg[1]})

		// Emptying system 8 leaves system 7 alone.
		b.List = []*Talkgroup{}
		must(t, b.Write(db, 8))
		must(t, b.Read(db, 8))
		assertEqual(t, "system 8 after clear", talkgroupIds(b), []uint{})
		must(t, a.Read(db, 7))
		assertEqual(t, "system 7 after clearing 8", talkgroupIds(a), []uint{100, 200})

		// Clearing optional fields on update stores them as absent.
		got, _ = a.GetTalkgroup(uint(200))
		got.Led = "white"
		got.Frequency = uint(155000000)
		must(t, a.Write(db, 7))
		must(t, a.Read(db, 7))
		got, _ = a.GetTalkgroup(uint(200))
		assertEqual(t, "led set", got.Led, "white")
		assertEqual(t, "frequency set", got.Frequency, uint(155000000))
		got.Led = nil
		got.Frequency = nil
		must(t, a.Write(db, 7))
		must(t, a.Read(db, 7))
		got, _ = a.GetTalkgroup(uint(200))
		assertEqual(t, "led cleared", got.Led, nil)
		assertEqual(t, "frequency cleared", got.Frequency, nil)
	})
}
