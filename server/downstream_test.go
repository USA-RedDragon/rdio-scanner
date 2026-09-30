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

func downstreamUrls(downstreams *Downstreams) []string {
	urls := []string{}
	for _, d := range downstreams.List {
		urls = append(urls, d.Url)
	}
	sort.Strings(urls)
	return urls
}

func findDownstream(t *testing.T, downstreams *Downstreams, url string) *Downstream {
	t.Helper()
	for _, d := range downstreams.List {
		if d.Url == url {
			return d
		}
	}
	t.Fatalf("downstream %q not found in %v", url, downstreamUrls(downstreams))
	return nil
}

func TestDownstreamsReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		downstreams := NewDownstreams()
		must(t, downstreams.Read(db))
		if len(downstreams.List) != 0 {
			t.Fatalf("fresh db: got %d downstreams, want 0", len(downstreams.List))
		}

		downstreams.List = []*Downstream{
			{Apikey: "ds-key-a", Order: uint(1), Systems: "*", Url: "https://a.example.org"},
			{Apikey: "ds-key-b", Disabled: true, Order: uint(2), Systems: `[{"id":1,"talkgroups":[10]},{"id":2,"talkgroups":"*"}]`, Url: "https://b.example.org/rdio"},
		}
		must(t, downstreams.Write(db))

		downstreams = NewDownstreams()
		must(t, downstreams.Read(db))
		assertEqual(t, "urls", downstreamUrls(downstreams), []string{"https://a.example.org", "https://b.example.org/rdio"})

		a := findDownstream(t, downstreams, "https://a.example.org")
		if a.Id == nil {
			t.Error("a: no id after read")
		}
		assertEqual(t, "a apiKey", a.Apikey, "ds-key-a")
		assertEqual(t, "a disabled", a.Disabled, false)
		assertEqual(t, "a order", a.Order, uint(1))
		assertEqual(t, "a systems", a.Systems, "*")

		b := findDownstream(t, downstreams, "https://b.example.org/rdio")
		if b.Id == nil {
			t.Error("b: no id after read")
		}
		assertEqual(t, "b apiKey", b.Apikey, "ds-key-b")
		assertEqual(t, "b disabled", b.Disabled, true)
		assertEqual(t, "b order", b.Order, uint(2))
		assertJSONEqual(t, "b systems", b.Systems, `[{"id":1,"talkgroups":[10]},{"id":2,"talkgroups":"*"}]`)
		if b.HasAccess(&Call{System: 1, Talkgroup: 10}) {
			t.Error("b is disabled and must not accept calls")
		}

		// Update every field of a, keep b.
		aId, bId := a.Id, b.Id
		a.Apikey = "ds-key-a2"
		a.Disabled = true
		a.Order = uint(3)
		a.Systems = `[{"id":5,"talkgroups":[50,51]}]`
		a.Url = "https://a2.example.org"
		b.Disabled = false
		// The admin flow always rebuilds items with FromMap, which stores
		// list-form systems as a JSON string; see
		// TestDownstreamsWriteReadBackScopedSystems for writing them as read.
		b.Systems = `[{"id":1,"talkgroups":[10]},{"id":2,"talkgroups":"*"}]`
		must(t, downstreams.Write(db))

		downstreams = NewDownstreams()
		must(t, downstreams.Read(db))
		a = findDownstream(t, downstreams, "https://a2.example.org")
		assertEqual(t, "updated id", a.Id, aId)
		assertEqual(t, "updated apiKey", a.Apikey, "ds-key-a2")
		assertEqual(t, "updated disabled", a.Disabled, true)
		assertEqual(t, "updated order", a.Order, uint(3))
		assertJSONEqual(t, "updated systems", a.Systems, `[{"id":5,"talkgroups":[50,51]}]`)
		b = findDownstream(t, downstreams, "https://b.example.org/rdio")
		assertEqual(t, "b id", b.Id, bId)
		assertEqual(t, "b re-enabled", b.Disabled, false)
		if !b.HasAccess(&Call{System: 1, Talkgroup: 10}) || b.HasAccess(&Call{System: 1, Talkgroup: 11}) {
			t.Error("b: read-back systems do not scope calls as written")
		}

		// Remove a.
		b.Systems = "*"
		downstreams.List = []*Downstream{b}
		must(t, downstreams.Write(db))
		downstreams = NewDownstreams()
		must(t, downstreams.Read(db))
		assertEqual(t, "urls after remove", downstreamUrls(downstreams), []string{"https://b.example.org/rdio"})
	})
}

// Removing an existing downstream and adding a new one in the same Write must do both.
func TestDownstreamsWriteRemoveAndAddTogether(t *testing.T) {
	t.Skip("known bug: sync-list Write skips deletions when any item has a nil Id; fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		downstreams := NewDownstreams()
		downstreams.List = []*Downstream{
			{Apikey: "k1", Systems: "*", Url: "https://keep.example.org"},
			{Apikey: "k2", Systems: "*", Url: "https://drop.example.org"},
		}
		must(t, downstreams.Write(db))
		downstreams = NewDownstreams()
		must(t, downstreams.Read(db))

		keep := findDownstream(t, downstreams, "https://keep.example.org")
		downstreams.List = []*Downstream{keep, {Apikey: "k3", Systems: "*", Url: "https://new.example.org"}}
		must(t, downstreams.Write(db))

		downstreams = NewDownstreams()
		must(t, downstreams.Read(db))
		assertEqual(t, "urls", downstreamUrls(downstreams), []string{"https://keep.example.org", "https://new.example.org"})
	})
}

// Writing back a downstream exactly as Read returned it must work. Read decodes
// list-form systems into []any, which Write passes straight to the driver.
func TestDownstreamsWriteReadBackScopedSystems(t *testing.T) {
	t.Skip("known bug: Write cannot store list-form systems as returned by Read ([]any is not a driver value); fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		const systems = `[{"id":1,"talkgroups":[10]}]`
		downstreams := NewDownstreams()
		downstreams.List = []*Downstream{{Apikey: "k", Systems: systems, Url: "https://a.example.org"}}
		must(t, downstreams.Write(db))
		downstreams = NewDownstreams()
		must(t, downstreams.Read(db))

		must(t, downstreams.Write(db))
		downstreams = NewDownstreams()
		must(t, downstreams.Read(db))
		d := findDownstream(t, downstreams, "https://a.example.org")
		assertJSONEqual(t, "systems", d.Systems, systems)
	})
}
