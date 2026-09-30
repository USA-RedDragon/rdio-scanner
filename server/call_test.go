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
	"bytes"
	"testing"
	"time"
)

// testAudio returns n bytes of deterministic fake audio.
func testAudio(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 7)
	}
	return b
}

// newTestCall builds a valid call at the given instant.
func newTestCall(system, talkgroup uint, at time.Time) *Call {
	call := NewCall()
	call.Audio = testAudio(64)
	call.AudioName = "call.m4a"
	call.AudioType = "audio/mp4"
	call.DateTime = at
	call.System = system
	call.Talkgroup = talkgroup
	return call
}

func mustWriteCall(t *testing.T, calls *Calls, db *Database, call *Call) uint {
	t.Helper()
	id, err := calls.WriteCall(call, db)
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("WriteCall returned id 0")
	}
	return id
}

func TestCallsWriteGetRoundTrip(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		calls := NewCalls()
		at := time.Date(2024, time.March, 10, 12, 34, 56, 0, time.UTC)

		call := newTestCall(1, 10, at)
		call.Audio = testAudio(4096)
		call.Frequencies = []map[string]any{
			{"errorCount": 0, "freq": 851012500, "len": 2.5, "pos": 0, "spikeCount": 1},
			{"errorCount": 3, "freq": 851037500, "len": 1, "pos": 2.5, "spikeCount": 0},
		}
		call.Frequency = uint(851012500)
		call.Patches = []uint{20, 30}
		call.Source = uint(1001)
		call.Sources = []map[string]any{{"pos": 0, "src": 1001}, {"pos": 2.5, "src": 1002}}

		id := mustWriteCall(t, calls, db, call)

		// A second call gets a different id.
		other := mustWriteCall(t, calls, db, newTestCall(2, 20, at.Add(time.Minute)))
		if other == id {
			t.Fatalf("two calls got the same id %d", id)
		}

		got, err := calls.GetCall(id, db)
		must(t, err)
		assertEqual(t, "id", got.Id, id)
		if !bytes.Equal(got.Audio, call.Audio) {
			t.Errorf("audio: got %d bytes, want %d identical bytes", len(got.Audio), len(call.Audio))
		}
		assertEqual(t, "audioName", got.AudioName, "call.m4a")
		assertEqual(t, "audioType", got.AudioType, "audio/mp4")
		assertSameSecond(t, "dateTime", got.DateTime, at)
		assertJSONEqual(t, "frequencies", got.Frequencies, call.Frequencies)
		assertEqual(t, "frequency", got.Frequency, uint(851012500))
		assertJSONEqual(t, "patches", got.Patches, []uint{20, 30})
		assertEqual(t, "source", got.Source, uint(1001))
		assertJSONEqual(t, "sources", got.Sources, call.Sources)
		assertEqual(t, "system", got.System, uint(1))
		assertEqual(t, "talkgroup", got.Talkgroup, uint(10))

		// A minimal call: optional fields come back absent.
		bare := NewCall()
		bare.Audio = testAudio(100)
		bare.DateTime = at
		bare.System = 3
		bare.Talkgroup = 30
		bareId := mustWriteCall(t, calls, db, bare)
		got, err = calls.GetCall(bareId, db)
		must(t, err)
		assertEqual(t, "bare audioName", got.AudioName, nil)
		assertEqual(t, "bare audioType", got.AudioType, nil)
		assertEqual(t, "bare frequency", got.Frequency, nil)
		assertEqual(t, "bare source", got.Source, nil)
		assertJSONEqual(t, "bare frequencies", got.Frequencies, []any{})
		assertJSONEqual(t, "bare patches", got.Patches, []any{})
		assertJSONEqual(t, "bare sources", got.Sources, []any{})
		assertSameSecond(t, "bare dateTime", got.DateTime, at)
	})
}

func TestCallsCheckDuplicate(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		calls := NewCalls()
		at := time.Date(2024, time.March, 10, 12, 0, 0, 0, time.UTC)
		mustWriteCall(t, calls, db, newTestCall(1, 10, at))

		cases := []struct {
			name string
			call *Call
			ms   uint
			want bool
		}{
			{"same instant", newTestCall(1, 10, at), 500, true},
			{"inside window after", newTestCall(1, 10, at.Add(2*time.Second)), 3000, true},
			{"inside window before", newTestCall(1, 10, at.Add(-2*time.Second)), 3000, true},
			{"outside window after", newTestCall(1, 10, at.Add(10*time.Second)), 3000, false},
			{"outside window before", newTestCall(1, 10, at.Add(-10*time.Second)), 3000, false},
			{"other talkgroup", newTestCall(1, 11, at), 3000, false},
			{"other system", newTestCall(2, 10, at), 3000, false},
		}
		for _, c := range cases {
			if got := calls.CheckDuplicate(c.call, c.ms, db); got != c.want {
				t.Errorf("%s: CheckDuplicate = %v, want %v", c.name, got, c.want)
			}
		}
	})
}

// callCount returns the number of stored calls, via an unfiltered Search.
func callCount(t *testing.T, calls *Calls, db *Database) uint {
	t.Helper()
	res, err := calls.Search(&CallsSearchOptions{}, testClient(db))
	must(t, err)
	return res.Count
}

func TestCallsPrune(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		calls := NewCalls()
		now := time.Now().UTC().Truncate(time.Second)

		mustWriteCall(t, calls, db, newTestCall(1, 10, now.Add(-30*24*time.Hour)))
		mustWriteCall(t, calls, db, newTestCall(1, 10, now.Add(-10*24*time.Hour)))
		recent := mustWriteCall(t, calls, db, newTestCall(1, 10, now.Add(-2*24*time.Hour)))
		fresh := mustWriteCall(t, calls, db, newTestCall(1, 10, now.Add(-time.Hour)))

		must(t, calls.Prune(db, 7))

		res, err := calls.Search(&CallsSearchOptions{}, testClient(db))
		must(t, err)
		ids := []uint{}
		for _, r := range res.Results {
			ids = append(ids, r.Id)
		}
		assertEqual(t, "remaining calls", ids, []uint{recent, fresh})
		assertEqual(t, "remaining count", res.Count, uint(2))

		// A generous window keeps everything that is left.
		must(t, calls.Prune(db, 365))
		assertEqual(t, "count after no-op prune", callCount(t, calls, db), uint(2))
	})
}

// Prune(0) means "everything older than now". The cutoff is computed from
// time.Now() in the local zone and formatted without converting to UTC, while
// calls are stored in UTC, so west of UTC it keeps hours of calls it should
// drop (and east of UTC it drops calls it should keep).
func TestCallsPruneIsTimeZoneIndependent(t *testing.T) {

	withLocalZone(t, time.FixedZone("UTC-5", -5*3600))
	forEachDatabase(t, func(t *testing.T, db *Database) {
		calls := NewCalls()
		mustWriteCall(t, calls, db, newTestCall(1, 10, time.Now().UTC().Add(-2*time.Hour)))

		must(t, calls.Prune(db, 0))
		assertEqual(t, "count after prune(0)", callCount(t, calls, db), uint(0))
	})
}

// searchFixture stores a known set of calls and returns their ids by name.
func searchFixture(t *testing.T, db *Database) (*Calls, map[string]uint) {
	t.Helper()
	calls := NewCalls()
	day1 := time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)

	ids := map[string]uint{}
	add := func(name string, system, talkgroup uint, at time.Time, patches []uint) {
		call := newTestCall(system, talkgroup, at)
		if patches != nil {
			call.Patches = patches
		}
		ids[name] = mustWriteCall(t, calls, db, call)
	}
	// Inserted out of chronological order on purpose.
	add("c3", 2, 10, day1.Add(12*time.Hour), nil)
	add("c1", 1, 10, day1.Add(10*time.Hour), nil)
	add("c4", 1, 10, day2.Add(9*time.Hour), nil)
	add("c2", 1, 20, day1.Add(11*time.Hour), nil)
	add("c5", 2, 30, day2.Add(10*time.Hour), []uint{10, 50})
	return calls, ids
}

func resultIds(res *CallsSearchResults) []uint {
	ids := []uint{}
	for _, r := range res.Results {
		ids = append(ids, r.Id)
	}
	return ids
}

func idsOf(ids map[string]uint, names ...string) []uint {
	out := []uint{}
	for _, n := range names {
		out = append(out, ids[n])
	}
	return out
}

func TestCallsSearch(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		calls, ids := searchFixture(t, db)
		client := testClient(db)
		day1 := time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC)
		day2 := day1.Add(24 * time.Hour)

		search := func(opts *CallsSearchOptions) *CallsSearchResults {
			t.Helper()
			res, err := calls.Search(opts, client)
			must(t, err)
			return res
		}

		// Basic listing: ascending by date, with the full result set's range.
		res := search(&CallsSearchOptions{})
		assertEqual(t, "all: count", res.Count, uint(5))
		assertEqual(t, "all: ids", resultIds(res), idsOf(ids, "c1", "c2", "c3", "c4", "c5"))
		assertSameSecond(t, "all: dateStart", res.DateStart, day1.Add(10*time.Hour))
		assertSameSecond(t, "all: dateStop", res.DateStop, day2.Add(10*time.Hour))
		r := res.Results[0]
		assertEqual(t, "result system", r.System, uint(1))
		assertEqual(t, "result talkgroup", r.Talkgroup, uint(10))
		assertSameSecond(t, "result dateTime", r.DateTime, day1.Add(10*time.Hour))

		// Descending.
		res = search(&CallsSearchOptions{Sort: -1})
		assertEqual(t, "desc: ids", resultIds(res), idsOf(ids, "c5", "c4", "c3", "c2", "c1"))

		// Filter by system, then by system and talkgroup.
		res = search(&CallsSearchOptions{System: uint(1)})
		assertEqual(t, "system 1: count", res.Count, uint(3))
		assertEqual(t, "system 1: ids", resultIds(res), idsOf(ids, "c1", "c2", "c4"))
		assertSameSecond(t, "system 1: dateStart", res.DateStart, day1.Add(10*time.Hour))
		assertSameSecond(t, "system 1: dateStop", res.DateStop, day2.Add(9*time.Hour))

		res = search(&CallsSearchOptions{System: uint(1), Talkgroup: uint(10)})
		assertEqual(t, "system 1 tg 10: count", res.Count, uint(2))
		assertEqual(t, "system 1 tg 10: ids", resultIds(res), idsOf(ids, "c1", "c4"))

		res = search(&CallsSearchOptions{System: uint(2)})
		assertEqual(t, "system 2: ids", resultIds(res), idsOf(ids, "c3", "c5"))

		// A talkgroup without a system is ignored.
		res = search(&CallsSearchOptions{Talkgroup: uint(20)})
		assertEqual(t, "talkgroup only: count", res.Count, uint(5))

		// Nothing matches: empty results, zero count.
		res = search(&CallsSearchOptions{System: uint(99)})
		assertEqual(t, "no match: count", res.Count, uint(0))
		assertEqual(t, "no match: ids", resultIds(res), []uint{})

		// Limit and offset page through the result; count is the full total.
		res = search(&CallsSearchOptions{Limit: uint(2), Offset: uint(1)})
		assertEqual(t, "page: count", res.Count, uint(5))
		assertEqual(t, "page: ids", resultIds(res), idsOf(ids, "c2", "c3"))
		res = search(&CallsSearchOptions{Limit: uint(2), Offset: uint(4)})
		assertEqual(t, "last page: ids", resultIds(res), idsOf(ids, "c5"))
		res = search(&CallsSearchOptions{Limit: uint(2), Offset: uint(1), Sort: -1})
		assertEqual(t, "desc page: ids", resultIds(res), idsOf(ids, "c4", "c3"))

		// Date: ascending covers the 24h from the given minute...
		res = search(&CallsSearchOptions{Date: day1})
		assertEqual(t, "date asc: count", res.Count, uint(3))
		assertEqual(t, "date asc: ids", resultIds(res), idsOf(ids, "c1", "c2", "c3"))
		res = search(&CallsSearchOptions{Date: day1.Add(10*time.Hour + 30*time.Minute)})
		assertEqual(t, "date asc mid-day: ids", resultIds(res), idsOf(ids, "c2", "c3", "c4", "c5"))
		// ...descending covers the 24h up to it.
		res = search(&CallsSearchOptions{Date: day2, Sort: -1})
		assertEqual(t, "date desc: ids", resultIds(res), idsOf(ids, "c3", "c2", "c1"))
		// Date combines with the system filter.
		res = search(&CallsSearchOptions{Date: day1, System: uint(1)})
		assertEqual(t, "date + system: ids", resultIds(res), idsOf(ids, "c1", "c2"))

		// Group and tag filters use the client's maps (label -> system -> talkgroups).
		client.GroupsMap = GroupsMap{"Fire": {1: {10}, 2: {30}}}
		client.TagsMap = TagsMap{"Law": {1: {20}}}
		res = search(&CallsSearchOptions{Group: "Fire"})
		assertEqual(t, "group: ids", resultIds(res), idsOf(ids, "c1", "c4", "c5"))
		res = search(&CallsSearchOptions{Tag: "Law"})
		assertEqual(t, "tag: ids", resultIds(res), idsOf(ids, "c2"))
		res = search(&CallsSearchOptions{Group: "Unknown"})
		assertEqual(t, "unknown group: count", res.Count, uint(5))
		client.GroupsMap, client.TagsMap = nil, nil

		// Access scoping: only system 1 talkgroup 10, and all of system 2.
		client.Access = &Access{Systems: []any{
			map[string]any{"id": float64(1), "talkgroups": []any{float64(10)}},
			map[string]any{"id": float64(2), "talkgroups": "*"},
		}}
		res = search(&CallsSearchOptions{})
		assertEqual(t, "scoped: count", res.Count, uint(4))
		assertEqual(t, "scoped: ids", resultIds(res), idsOf(ids, "c1", "c3", "c4", "c5"))
		res = search(&CallsSearchOptions{System: uint(1)})
		assertEqual(t, "scoped system 1: ids", resultIds(res), idsOf(ids, "c1", "c4"))

		// A wildcard access sees everything.
		client.Access = &Access{Systems: "*"}
		assertEqual(t, "wildcard access: count", search(&CallsSearchOptions{}).Count, uint(5))

		// Patched talkgroups: tg 10 in system 2 also finds c5 (patched with 10).
		res = search(&CallsSearchOptions{System: uint(2), Talkgroup: uint(10), searchPatchedTalkgroups: true})
		assertEqual(t, "patched: ids", resultIds(res), idsOf(ids, "c3", "c5"))
	})
}

// With patched-talkgroup search, the talkgroup/patches OR group is joined to
// the system condition without parentheses, so a call in another system whose
// patches contain the talkgroup leaks into the results.
func TestCallsSearchPatchedTalkgroupsStaysInSystem(t *testing.T) {

	forEachDatabase(t, func(t *testing.T, db *Database) {
		calls, ids := searchFixture(t, db)
		// c5 is system 2 patched with [10,50]; it must not show up for system 1.
		res, err := calls.Search(&CallsSearchOptions{System: uint(1), Talkgroup: uint(10), searchPatchedTalkgroups: true}, testClient(db))
		must(t, err)
		assertEqual(t, "ids", resultIds(res), idsOf(ids, "c1", "c4"))
		assertEqual(t, "count", res.Count, uint(2))
	})
}

// A call patched with a single talkgroup is stored as "[10]", which none of
// the patches patterns ('10', '[10,%', '%,10,%', '%,10]') match.
func TestCallsSearchPatchedTalkgroupsSinglePatch(t *testing.T) {

	forEachDatabase(t, func(t *testing.T, db *Database) {
		calls := NewCalls()
		at := time.Date(2024, time.March, 10, 12, 0, 0, 0, time.UTC)
		direct := mustWriteCall(t, calls, db, newTestCall(1, 10, at))
		single := newTestCall(1, 60, at.Add(time.Minute))
		single.Patches = []uint{10}
		patched := mustWriteCall(t, calls, db, single)

		res, err := calls.Search(&CallsSearchOptions{System: uint(1), Talkgroup: uint(10), searchPatchedTalkgroups: true}, testClient(db))
		must(t, err)
		assertEqual(t, "ids", resultIds(res), []uint{direct, patched})
	})
}
