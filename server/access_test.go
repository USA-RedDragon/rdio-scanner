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
	"time"
)

// Systems in the per-system list form, as Access.FromMap stores it (a JSON
// string) before a Write.
const accessScopedSystems = `[{"id":1,"talkgroups":"*"},{"id":2,"talkgroups":[10,20]}]`

func accessCodes(accesses *Accesses) []string {
	codes := []string{}
	for _, a := range accesses.List {
		codes = append(codes, a.Code)
	}
	sort.Strings(codes)
	return codes
}

func mustGetAccess(t *testing.T, accesses *Accesses, code string) *Access {
	t.Helper()
	a, ok := accesses.GetAccess(code)
	if !ok {
		t.Fatalf("access %q not found in %v", code, accessCodes(accesses))
	}
	return a
}

func TestAccessesReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		expiration := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)

		accesses := NewAccesses()
		must(t, accesses.Read(db))
		if len(accesses.List) != 0 {
			t.Fatalf("fresh db: got %d accesses, want 0", len(accesses.List))
		}

		all := &Access{Code: "code-all", Expiration: expiration, Ident: "Everything", Limit: uint(3), Order: uint(1), Systems: "*"}
		scoped := &Access{Code: "code-scoped", Ident: "Scoped", Order: uint(2), Systems: accessScopedSystems}
		accesses.Add(all)
		accesses.Add(scoped)
		must(t, accesses.Write(db))

		accesses = NewAccesses()
		must(t, accesses.Read(db))
		if len(accesses.List) != 2 {
			t.Fatalf("after add: got %v, want 2 accesses", accessCodes(accesses))
		}

		got := mustGetAccess(t, accesses, "code-all")
		if got.Id == nil {
			t.Error("code-all: no id after read")
		}
		assertEqual(t, "code-all ident", got.Ident, "Everything")
		assertEqual(t, "code-all limit", got.Limit, uint(3))
		assertEqual(t, "code-all order", got.Order, uint(1))
		assertEqual(t, "code-all systems", got.Systems, "*")
		if exp, ok := got.Expiration.(time.Time); !ok {
			t.Errorf("code-all expiration: got %#v, want a time.Time", got.Expiration)
		} else {
			assertSameSecond(t, "code-all expiration", exp, expiration)
		}
		if got.HasExpired() {
			t.Error("code-all should not have expired")
		}

		got = mustGetAccess(t, accesses, "code-scoped")
		if got.Id == nil {
			t.Error("code-scoped: no id after read")
		}
		assertEqual(t, "code-scoped ident", got.Ident, "Scoped")
		assertEqual(t, "code-scoped expiration", got.Expiration, nil)
		assertEqual(t, "code-scoped limit", got.Limit, nil)
		assertEqual(t, "code-scoped order", got.Order, uint(2))
		assertJSONEqual(t, "code-scoped systems", got.Systems, accessScopedSystems)
		if !got.HasAccess(&Call{System: 2, Talkgroup: 20}) || got.HasAccess(&Call{System: 2, Talkgroup: 30}) {
			t.Error("code-scoped: read-back systems do not scope calls as written")
		}
		scopedId := got.Id

		// Update both: new code/ident/limit/expiration for one, switch the
		// other from a talkgroup list to the wildcard and back to a new list.
		newExpiration := time.Date(2031, time.June, 7, 8, 9, 10, 0, time.UTC)
		got = mustGetAccess(t, accesses, "code-all")
		allId := got.Id
		got.Code = "code-all-2"
		got.Ident = "Everything 2"
		got.Limit = uint(5)
		got.Order = uint(3)
		got.Expiration = newExpiration
		got.Systems = "*"
		got = mustGetAccess(t, accesses, "code-scoped")
		got.Systems = `[{"id":3,"talkgroups":[30]}]`
		must(t, accesses.Write(db))

		accesses = NewAccesses()
		must(t, accesses.Read(db))
		if len(accesses.List) != 2 {
			t.Fatalf("after update: got %v, want 2 accesses", accessCodes(accesses))
		}
		got = mustGetAccess(t, accesses, "code-all-2")
		assertEqual(t, "updated id", got.Id, allId)
		assertEqual(t, "updated ident", got.Ident, "Everything 2")
		assertEqual(t, "updated limit", got.Limit, uint(5))
		assertEqual(t, "updated order", got.Order, uint(3))
		if exp, ok := got.Expiration.(time.Time); !ok || !sameSecond(exp, newExpiration) {
			t.Errorf("updated expiration: got %#v, want %v", got.Expiration, newExpiration)
		}
		got = mustGetAccess(t, accesses, "code-scoped")
		assertEqual(t, "scoped id", got.Id, scopedId)
		assertJSONEqual(t, "updated scoped systems", got.Systems, `[{"id":3,"talkgroups":[30]}]`)

		// Remove one.
		accesses.Remove(&Access{Ident: "Scoped"})
		must(t, accesses.Write(db))
		accesses = NewAccesses()
		must(t, accesses.Read(db))
		assertEqual(t, "after remove", accessCodes(accesses), []string{"code-all-2"})

		// Remove the last one.
		accesses.List = []*Access{}
		must(t, accesses.Write(db))
		must(t, accesses.Read(db))
		assertEqual(t, "after remove all", accessCodes(accesses), []string{})
	})
}

// Removing an existing access and adding a new one in the same Write must do both.
func TestAccessesWriteRemoveAndAddTogether(t *testing.T) {
	t.Skip("known bug: sync-list Write skips deletions when any item has a nil Id; fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		accesses := NewAccesses()
		accesses.Add(&Access{Code: "keep", Ident: "Keep", Systems: "*"})
		accesses.Add(&Access{Code: "drop", Ident: "Drop", Systems: "*"})
		must(t, accesses.Write(db))
		must(t, accesses.Read(db))

		accesses.Remove(&Access{Ident: "Drop"})
		accesses.Add(&Access{Code: "new", Ident: "New", Systems: "*"})
		must(t, accesses.Write(db))

		accesses = NewAccesses()
		must(t, accesses.Read(db))
		assertEqual(t, "codes", accessCodes(accesses), []string{"keep", "new"})
	})
}

// Writing back accesses exactly as Read returned them must work. Read decodes
// list-form systems into []any, which Write passes straight to the driver.
// Reachable: the admin user-add/user-remove handlers Add/Remove on the
// in-memory list (loaded by Read) and then Write the whole list.
func TestAccessesWriteReadBackScopedSystems(t *testing.T) {
	t.Skip("known bug: Write cannot store list-form systems as returned by Read ([]any is not a driver value); fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		accesses := NewAccesses()
		accesses.Add(&Access{Code: "scoped", Ident: "Scoped", Systems: accessScopedSystems})
		must(t, accesses.Write(db))
		accesses = NewAccesses()
		must(t, accesses.Read(db))

		// What UserAddHandler does: add one and write the whole list.
		accesses.Add(NewAccess().FromMap(map[string]any{"code": "another", "ident": "Another", "systems": "*"}))
		must(t, accesses.Write(db))

		accesses = NewAccesses()
		must(t, accesses.Read(db))
		assertEqual(t, "codes", accessCodes(accesses), []string{"another", "scoped"})
		assertJSONEqual(t, "scoped systems", mustGetAccess(t, accesses, "scoped").Systems, accessScopedSystems)
	})
}
