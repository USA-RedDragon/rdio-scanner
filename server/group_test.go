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

func groupLabels(groups *Groups) []string {
	labels := []string{}
	for _, g := range groups.List {
		labels = append(labels, g.Label)
	}
	sort.Strings(labels)
	return labels
}

func TestGroupsReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		groups := NewGroups()
		if err := groups.Read(db); err != nil {
			t.Fatal(err)
		}
		seeded := len(groups.List)
		if seeded == 0 {
			t.Fatal("expected seeded groups")
		}

		// Add one (no id yet) and write.
		groups.List = append(groups.List, &Group{Label: "Test Group"})
		if err := groups.Write(db); err != nil {
			t.Fatal(err)
		}
		if err := groups.Read(db); err != nil {
			t.Fatal(err)
		}
		if len(groups.List) != seeded+1 {
			t.Fatalf("after add: got %d groups, want %d: %v", len(groups.List), seeded+1, groupLabels(groups))
		}
		added, ok := groups.GetGroup("Test Group")
		if !ok || added.Id == nil {
			t.Fatalf("added group missing or without id: %+v", added)
		}

		// Rename it.
		added.Label = "Renamed Group"
		if err := groups.Write(db); err != nil {
			t.Fatal(err)
		}
		if err := groups.Read(db); err != nil {
			t.Fatal(err)
		}
		if _, ok := groups.GetGroup("Renamed Group"); !ok {
			t.Fatalf("rename not persisted: %v", groupLabels(groups))
		}

		// Remove it.
		kept := []*Group{}
		for _, g := range groups.List {
			if g.Label != "Renamed Group" {
				kept = append(kept, g)
			}
		}
		groups.List = kept
		if err := groups.Write(db); err != nil {
			t.Fatal(err)
		}
		if err := groups.Read(db); err != nil {
			t.Fatal(err)
		}
		if len(groups.List) != seeded {
			t.Fatalf("after remove: got %d groups, want %d: %v", len(groups.List), seeded, groupLabels(groups))
		}
	})
}
