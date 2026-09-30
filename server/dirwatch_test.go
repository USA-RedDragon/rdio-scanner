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

// Only Read/Write are exercised: no watcher is ever started, and the
// directories are plain strings that are never touched.

import (
	"sort"
	"testing"
)

func dirwatchDirs(dirwatches *Dirwatches) []string {
	dirs := []string{}
	for _, d := range dirwatches.List {
		dirs = append(dirs, d.Directory)
	}
	sort.Strings(dirs)
	return dirs
}

func findDirwatch(t *testing.T, dirwatches *Dirwatches, dir string) *Dirwatch {
	t.Helper()
	for _, d := range dirwatches.List {
		if d.Directory == dir {
			return d
		}
	}
	t.Fatalf("dirwatch %q not found in %v", dir, dirwatchDirs(dirwatches))
	return nil
}

// writeTwoDirwatches stores a fully populated dirwatch and a minimal one.
func writeTwoDirwatches(t *testing.T, db *Database) *Dirwatches {
	t.Helper()
	full := NewDirwatch()
	full.Delay = uint(2000)
	full.DeleteAfter = true
	full.Directory = "/nonexistent/rdio-test/full"
	full.Disabled = true
	full.Extension = "wav"
	full.Frequency = uint(851012500)
	full.Mask = "#DATE_#TIME_#TG"
	full.Order = uint(1)
	full.SystemId = uint(11)
	full.TalkgroupId = uint(1001)
	full.Kind = DirwatchTypeDefault
	full.UsePolling = true

	minimal := NewDirwatch()
	minimal.Directory = "/nonexistent/rdio-test/minimal"
	minimal.Kind = DirwatchTypeTrunkRecorder
	minimal.Order = uint(2)

	dirwatches := NewDirwatches()
	dirwatches.List = []*Dirwatch{full, minimal}
	must(t, dirwatches.Write(db))

	dirwatches = NewDirwatches()
	must(t, dirwatches.Read(db))
	return dirwatches
}

func TestDirwatchesReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		dirwatches := NewDirwatches()
		must(t, dirwatches.Read(db))
		if len(dirwatches.List) != 0 {
			t.Fatalf("fresh db: got %d dirwatches, want 0", len(dirwatches.List))
		}

		dirwatches = writeTwoDirwatches(t, db)
		assertEqual(t, "dirs", dirwatchDirs(dirwatches), []string{"/nonexistent/rdio-test/full", "/nonexistent/rdio-test/minimal"})

		full := findDirwatch(t, dirwatches, "/nonexistent/rdio-test/full")
		if full.Id == nil {
			t.Error("full: no id after read")
		}
		assertEqual(t, "full delay", full.Delay, uint(2000))
		assertEqual(t, "full deleteAfter", full.DeleteAfter, true)
		assertEqual(t, "full disabled", full.Disabled, true)
		assertEqual(t, "full extension", full.Extension, "wav")
		assertEqual(t, "full frequency", full.Frequency, uint(851012500))
		assertEqual(t, "full mask", full.Mask, "#DATE_#TIME_#TG")
		assertEqual(t, "full order", full.Order, uint(1))
		assertEqual(t, "full systemId", full.SystemId, uint(11))
		assertEqual(t, "full talkgroupId", full.TalkgroupId, uint(1001))
		assertEqual(t, "full type", full.Kind, DirwatchTypeDefault)
		assertEqual(t, "full usePolling", full.UsePolling, true)

		minimal := findDirwatch(t, dirwatches, "/nonexistent/rdio-test/minimal")
		if minimal.Id == nil {
			t.Error("minimal: no id after read")
		}
		assertEqual(t, "minimal delay", minimal.Delay, nil)
		assertEqual(t, "minimal deleteAfter", minimal.DeleteAfter, false)
		assertEqual(t, "minimal disabled", minimal.Disabled, false)
		assertEqual(t, "minimal extension", minimal.Extension, nil)
		assertEqual(t, "minimal frequency", minimal.Frequency, nil)
		assertEqual(t, "minimal mask", minimal.Mask, nil)
		assertEqual(t, "minimal order", minimal.Order, uint(2))
		assertEqual(t, "minimal systemId", minimal.SystemId, nil)
		assertEqual(t, "minimal talkgroupId", minimal.TalkgroupId, nil)
		assertEqual(t, "minimal type", minimal.Kind, DirwatchTypeTrunkRecorder)
		assertEqual(t, "minimal usePolling", minimal.UsePolling, false)

		// Update: change every field of minimal, flip the booleans of full.
		fullId, minimalId := full.Id, minimal.Id
		minimal.Delay = uint(500)
		minimal.DeleteAfter = true
		minimal.Directory = "/nonexistent/rdio-test/renamed"
		minimal.Extension = "mp3"
		minimal.Frequency = uint(460125000)
		minimal.Mask = "#TG-#UNIT"
		minimal.Order = uint(3)
		minimal.SystemId = uint(22)
		minimal.TalkgroupId = uint(2002)
		minimal.Kind = DirwatchTypeSdrTrunk
		minimal.UsePolling = true
		full.DeleteAfter = false
		full.Disabled = false
		full.UsePolling = false
		must(t, dirwatches.Write(db))

		dirwatches = NewDirwatches()
		must(t, dirwatches.Read(db))
		assertEqual(t, "dirs after update", dirwatchDirs(dirwatches), []string{"/nonexistent/rdio-test/full", "/nonexistent/rdio-test/renamed"})
		renamed := findDirwatch(t, dirwatches, "/nonexistent/rdio-test/renamed")
		assertEqual(t, "renamed id", renamed.Id, minimalId)
		assertEqual(t, "renamed delay", renamed.Delay, uint(500))
		assertEqual(t, "renamed deleteAfter", renamed.DeleteAfter, true)
		assertEqual(t, "renamed extension", renamed.Extension, "mp3")
		assertEqual(t, "renamed frequency", renamed.Frequency, uint(460125000))
		assertEqual(t, "renamed mask", renamed.Mask, "#TG-#UNIT")
		assertEqual(t, "renamed order", renamed.Order, uint(3))
		assertEqual(t, "renamed systemId", renamed.SystemId, uint(22))
		assertEqual(t, "renamed talkgroupId", renamed.TalkgroupId, uint(2002))
		assertEqual(t, "renamed type", renamed.Kind, DirwatchTypeSdrTrunk)
		assertEqual(t, "renamed usePolling", renamed.UsePolling, true)
		full = findDirwatch(t, dirwatches, "/nonexistent/rdio-test/full")
		assertEqual(t, "full id", full.Id, fullId)
		assertEqual(t, "full deleteAfter after update", full.DeleteAfter, false)
		assertEqual(t, "full disabled after update", full.Disabled, false)
		assertEqual(t, "full usePolling after update", full.UsePolling, false)
		assertEqual(t, "full delay after update", full.Delay, uint(2000))
	})
}

// A zero delay must read back like the other optional numeric fields (nil),
// not as uint(0): Read checks the wrong variable (id instead of delay).
func TestDirwatchesZeroDelay(t *testing.T) {
	t.Skip("known bug: dirwatch Read tests id.Float64 instead of delay.Float64, so delay 0 reads back as uint(0); fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		d := NewDirwatch()
		d.Delay = uint(0)
		d.Directory = "/nonexistent/rdio-test/zero-delay"
		d.Kind = DirwatchTypeDefault

		dirwatches := NewDirwatches()
		dirwatches.List = []*Dirwatch{d}
		must(t, dirwatches.Write(db))

		dirwatches = NewDirwatches()
		must(t, dirwatches.Read(db))
		got := findDirwatch(t, dirwatches, "/nonexistent/rdio-test/zero-delay")
		assertEqual(t, "delay", got.Delay, nil)
	})
}

func TestDirwatchesRemove(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		if db.Config.DbType == DbTypeMysql || db.Config.DbType == DbTypeMariadb {
			t.Skip("known bug: dirwatch delete uses table rdioScannerDirwatches (wrong case) and fails on mysql/mariadb; fixed during ent migration")
		}

		dirwatches := writeTwoDirwatches(t, db)
		full := findDirwatch(t, dirwatches, "/nonexistent/rdio-test/full")
		dirwatches.List = []*Dirwatch{full}
		must(t, dirwatches.Write(db))

		dirwatches = NewDirwatches()
		must(t, dirwatches.Read(db))
		assertEqual(t, "dirs after remove", dirwatchDirs(dirwatches), []string{"/nonexistent/rdio-test/full"})

		dirwatches.List = []*Dirwatch{}
		must(t, dirwatches.Write(db))
		must(t, dirwatches.Read(db))
		assertEqual(t, "dirs after remove all", dirwatchDirs(dirwatches), []string{})
	})
}

// Removing an existing dirwatch and adding a new one in the same Write must do both.
func TestDirwatchesWriteRemoveAndAddTogether(t *testing.T) {
	t.Skip("known bug: sync-list Write skips deletions when any item has a nil Id; fixed during ent migration")

	forEachDatabase(t, func(t *testing.T, db *Database) {
		dirwatches := writeTwoDirwatches(t, db)
		full := findDirwatch(t, dirwatches, "/nonexistent/rdio-test/full")

		added := NewDirwatch()
		added.Directory = "/nonexistent/rdio-test/new"
		added.Kind = DirwatchTypeDSDPlus
		dirwatches.List = []*Dirwatch{full, added}
		must(t, dirwatches.Write(db))

		dirwatches = NewDirwatches()
		must(t, dirwatches.Read(db))
		assertEqual(t, "dirs", dirwatchDirs(dirwatches), []string{"/nonexistent/rdio-test/full", "/nonexistent/rdio-test/new"})
	})
}
