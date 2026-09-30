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

func apikeyKeys(apikeys *Apikeys) []string {
	keys := []string{}
	for _, a := range apikeys.List {
		keys = append(keys, a.Key)
	}
	sort.Strings(keys)
	return keys
}

// findApikey looks an api key up by key, including disabled ones (GetApikey
// hides those).
func findApikey(t *testing.T, apikeys *Apikeys, key string) *Apikey {
	t.Helper()
	for _, a := range apikeys.List {
		if a.Key == key {
			return a
		}
	}
	t.Fatalf("api key %q not found in %v", key, apikeyKeys(apikeys))
	return nil
}

// writeTwoApikeys stores an enabled wildcard key and a disabled scoped key.
func writeTwoApikeys(t *testing.T, db *Database) *Apikeys {
	t.Helper()
	apikeys := NewApikeys()
	apikeys.List = []*Apikey{
		{Ident: "Uploader", Key: "key-all", Order: uint(1), Systems: "*"},
		{Disabled: true, Ident: "Scoped", Key: "key-scoped", Order: uint(2), Systems: `[{"id":1,"talkgroups":[10,20]},{"id":2,"talkgroups":"*"}]`},
	}
	must(t, apikeys.Write(db))
	apikeys = NewApikeys()
	must(t, apikeys.Read(db))
	return apikeys
}

func TestApikeysReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		apikeys := NewApikeys()
		must(t, apikeys.Read(db))
		if len(apikeys.List) != 0 {
			t.Fatalf("fresh db: got %d api keys, want 0", len(apikeys.List))
		}

		apikeys = writeTwoApikeys(t, db)
		assertEqual(t, "keys", apikeyKeys(apikeys), []string{"key-all", "key-scoped"})

		all := findApikey(t, apikeys, "key-all")
		if all.Id == nil {
			t.Error("key-all: no id after read")
		}
		assertEqual(t, "key-all disabled", all.Disabled, false)
		assertEqual(t, "key-all ident", all.Ident, "Uploader")
		assertEqual(t, "key-all order", all.Order, uint(1))
		assertEqual(t, "key-all systems", all.Systems, "*")

		scoped := findApikey(t, apikeys, "key-scoped")
		if scoped.Id == nil {
			t.Error("key-scoped: no id after read")
		}
		assertEqual(t, "key-scoped disabled", scoped.Disabled, true)
		assertEqual(t, "key-scoped ident", scoped.Ident, "Scoped")
		assertEqual(t, "key-scoped order", scoped.Order, uint(2))
		assertJSONEqual(t, "key-scoped systems", scoped.Systems, `[{"id":1,"talkgroups":[10,20]},{"id":2,"talkgroups":"*"}]`)

		if _, ok := apikeys.GetApikey("key-scoped"); ok {
			t.Error("GetApikey returned a disabled key")
		}
		if _, ok := apikeys.GetApikey("key-all"); !ok {
			t.Error("GetApikey did not return the enabled key")
		}

		// Update: rename/re-key, flip disabled, change order and systems.
		allId, scopedId := all.Id, scoped.Id
		all.Key = "key-all-2"
		all.Ident = "Uploader 2"
		all.Disabled = true
		all.Order = uint(5)
		all.Systems = `[{"id":7,"talkgroups":"*"}]`
		scoped.Disabled = false
		scoped.Systems = "*"
		must(t, apikeys.Write(db))

		apikeys = NewApikeys()
		must(t, apikeys.Read(db))
		assertEqual(t, "keys after update", apikeyKeys(apikeys), []string{"key-all-2", "key-scoped"})
		all = findApikey(t, apikeys, "key-all-2")
		assertEqual(t, "updated id", all.Id, allId)
		assertEqual(t, "updated ident", all.Ident, "Uploader 2")
		assertEqual(t, "updated disabled", all.Disabled, true)
		assertEqual(t, "updated order", all.Order, uint(5))
		assertJSONEqual(t, "updated systems", all.Systems, `[{"id":7,"talkgroups":"*"}]`)
		scoped = findApikey(t, apikeys, "key-scoped")
		assertEqual(t, "scoped id", scoped.Id, scopedId)
		assertEqual(t, "scoped disabled", scoped.Disabled, false)
		assertEqual(t, "scoped systems", scoped.Systems, "*")
	})
}

func TestApikeysRemove(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		if db.Config.DbType == DbTypeMysql || db.Config.DbType == DbTypeMariadb {
			t.Skip("known bug: api key delete uses table rdioScannerApikeys (wrong case) and fails on mysql/mariadb; fixed during ent migration")
		}

		apikeys := writeTwoApikeys(t, db)

		kept := []*Apikey{}
		for _, a := range apikeys.List {
			if a.Key != "key-scoped" {
				kept = append(kept, a)
			}
		}
		apikeys.List = kept
		must(t, apikeys.Write(db))

		apikeys = NewApikeys()
		must(t, apikeys.Read(db))
		assertEqual(t, "keys after remove", apikeyKeys(apikeys), []string{"key-all"})

		apikeys.List = []*Apikey{}
		must(t, apikeys.Write(db))
		must(t, apikeys.Read(db))
		assertEqual(t, "keys after remove all", apikeyKeys(apikeys), []string{})
	})
}

// Removing an existing api key and adding a new one in the same Write must do both.
func TestApikeysWriteRemoveAndAddTogether(t *testing.T) {
	t.Skip("known bug: sync-list Write skips deletions when any item has a nil Id; fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		apikeys := writeTwoApikeys(t, db)

		kept := []*Apikey{}
		for _, a := range apikeys.List {
			if a.Key != "key-scoped" {
				kept = append(kept, a)
			}
		}
		apikeys.List = append(kept, &Apikey{Ident: "New", Key: "key-new", Systems: "*"})
		must(t, apikeys.Write(db))

		apikeys = NewApikeys()
		must(t, apikeys.Read(db))
		assertEqual(t, "keys", apikeyKeys(apikeys), []string{"key-all", "key-new"})
	})
}

// Writing back api keys exactly as Read returned them must work. Read decodes
// list-form systems into []any, which Write passes straight to the driver.
func TestApikeysWriteReadBackScopedSystems(t *testing.T) {
	t.Skip("known bug: Write cannot store list-form systems as returned by Read ([]any is not a driver value); fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		apikeys := writeTwoApikeys(t, db)
		must(t, apikeys.Write(db))

		apikeys = NewApikeys()
		must(t, apikeys.Read(db))
		assertJSONEqual(t, "scoped systems", findApikey(t, apikeys, "key-scoped").Systems, `[{"id":1,"talkgroups":[10,20]},{"id":2,"talkgroups":"*"}]`)
	})
}
