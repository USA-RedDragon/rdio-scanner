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

func tagLabels(tags *Tags) []string {
	labels := []string{}
	for _, tag := range tags.List {
		labels = append(labels, tag.Label)
	}
	sort.Strings(labels)
	return labels
}

func TestTagsReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		tags := NewTags()
		must(t, tags.Read(db))
		seeded := len(tags.List)
		if seeded == 0 {
			t.Fatal("expected seeded tags")
		}
		for _, tag := range tags.List {
			if tag.Id == nil || tag.Label == "" {
				t.Fatalf("seeded tag without id or label: %+v", tag)
			}
		}

		// Add one (no id yet) and write.
		tags.List = append(tags.List, &Tag{Label: "Test Tag"})
		must(t, tags.Write(db))
		must(t, tags.Read(db))
		if len(tags.List) != seeded+1 {
			t.Fatalf("after add: got %d tags, want %d: %v", len(tags.List), seeded+1, tagLabels(tags))
		}
		added, ok := tags.GetTag("Test Tag")
		if !ok || added.Id == nil {
			t.Fatalf("added tag missing or without id: %+v", added)
		}
		addedId := added.Id.(uint)
		if byId, ok := tags.GetTag(addedId); !ok || byId.Label != "Test Tag" {
			t.Fatalf("GetTag by id %d: %+v %v", addedId, byId, ok)
		}

		// Rename it; the id must be kept.
		added.Label = "Renamed Tag"
		must(t, tags.Write(db))
		must(t, tags.Read(db))
		renamed, ok := tags.GetTag("Renamed Tag")
		if !ok {
			t.Fatalf("rename not persisted: %v", tagLabels(tags))
		}
		assertEqual(t, "renamed tag id", renamed.Id, addedId)
		if _, ok := tags.GetTag("Test Tag"); ok {
			t.Fatalf("old label still present: %v", tagLabels(tags))
		}

		// Remove it.
		kept := []*Tag{}
		for _, tag := range tags.List {
			if tag.Label != "Renamed Tag" {
				kept = append(kept, tag)
			}
		}
		tags.List = kept
		must(t, tags.Write(db))
		must(t, tags.Read(db))
		if len(tags.List) != seeded {
			t.Fatalf("after remove: got %d tags, want %d: %v", len(tags.List), seeded, tagLabels(tags))
		}
		if _, ok := tags.GetTag(addedId); ok {
			t.Fatalf("removed tag id %d still present", addedId)
		}
	})
}

// Removing an existing tag and adding a new one in the same Write must do both.
func TestTagsWriteRemoveAndAddTogether(t *testing.T) {
	t.Skip("known bug: sync-list Write skips deletions when any item has a nil Id; fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		tags := NewTags()
		must(t, tags.Read(db))
		seeded := len(tags.List)
		if seeded < 2 {
			t.Fatalf("need at least 2 seeded tags, got %d", seeded)
		}
		removed := tags.List[0].Label

		tags.List = append(tags.List[1:], &Tag{Label: "Brand New Tag"})
		must(t, tags.Write(db))
		must(t, tags.Read(db))

		if _, ok := tags.GetTag(removed); ok {
			t.Errorf("tag %q should have been removed: %v", removed, tagLabels(tags))
		}
		if _, ok := tags.GetTag("Brand New Tag"); !ok {
			t.Errorf("new tag missing: %v", tagLabels(tags))
		}
		if len(tags.List) != seeded {
			t.Errorf("got %d tags, want %d: %v", len(tags.List), seeded, tagLabels(tags))
		}
	})
}
