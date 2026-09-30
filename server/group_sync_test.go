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

// Removing an existing group and adding a new one in the same Write must do
// both. (Kept out of group_test.go, which is the reference example.)
func TestGroupsWriteRemoveAndAddTogether(t *testing.T) {

	forEachDatabase(t, func(t *testing.T, db *Database) {
		groups := NewGroups()
		must(t, groups.Read(db))
		seeded := len(groups.List)
		if seeded < 2 {
			t.Fatalf("need at least 2 seeded groups, got %d", seeded)
		}
		removed := groups.List[0].Label

		groups.List = append(groups.List[1:], &Group{Label: "Brand New Group"})
		must(t, groups.Write(db))
		must(t, groups.Read(db))

		if _, ok := groups.GetGroup(removed); ok {
			t.Errorf("group %q should have been removed: %v", removed, groupLabels(groups))
		}
		if _, ok := groups.GetGroup("Brand New Group"); !ok {
			t.Errorf("new group missing: %v", groupLabels(groups))
		}
		if len(groups.List) != seeded {
			t.Errorf("got %d groups, want %d: %v", len(groups.List), seeded, groupLabels(groups))
		}
	})
}
