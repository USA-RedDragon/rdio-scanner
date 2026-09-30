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
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// optionsSnapshot captures every persisted option (Options holds a mutex, so
// it is compared through this map rather than copied).
func optionsSnapshot(o *Options) map[string]any {
	return map[string]any{
		"adminPassword":               o.adminPassword,
		"adminPasswordNeedChange":     o.adminPasswordNeedChange,
		"afsSystems":                  o.AfsSystems,
		"audioConversion":             o.AudioConversion,
		"audioBitrate":                o.AudioBitrate,
		"autoPopulate":                o.AutoPopulate,
		"branding":                    o.Branding,
		"dimmerDelay":                 o.DimmerDelay,
		"disableDuplicateDetection":   o.DisableDuplicateDetection,
		"duplicateDetectionTimeFrame": o.DuplicateDetectionTimeFrame,
		"keypadBeeps":                 o.KeypadBeeps,
		"maxClients":                  o.MaxClients,
		"playbackGoesLive":            o.PlaybackGoesLive,
		"pruneCallDays":               o.PruneCallDays,
		"pruneLogDays":                o.PruneLogDays,
		"searchPatchedTalkgroups":     o.SearchPatchedTalkgroups,
		"showListenersCount":          o.ShowListenersCount,
		"sortTalkgroups":              o.SortTalkgroups,
		"tagsToggle":                  o.TagsToggle,
		"time12hFormat":               o.Time12hFormat,
	}
}

func readOptions(t *testing.T, db *Database) *Options {
	t.Helper()
	o := NewOptions()
	must(t, o.Read(db))
	return o
}

func TestOptionsReadWrite(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		// A fresh database reads back the defaults, including the default
		// admin password (as a bcrypt hash) that must be changed.
		o := readOptions(t, db)
		if err := bcrypt.CompareHashAndPassword([]byte(o.adminPassword), []byte(defaults.adminPassword)); err != nil {
			t.Errorf("fresh db: admin password is not the default: %v", err)
		}
		assertEqual(t, "fresh adminPasswordNeedChange", o.adminPasswordNeedChange, defaults.adminPasswordNeedChange)
		assertEqual(t, "fresh maxClients", o.MaxClients, defaults.options.maxClients)
		assertEqual(t, "fresh keypadBeeps", o.KeypadBeeps, defaults.options.keypadBeeps)
		assertEqual(t, "fresh pruneCallDays", o.PruneCallDays, defaults.options.pruneCallDays)

		hash, err := bcrypt.GenerateFromPassword([]byte("s3cret-test"), bcrypt.MinCost)
		must(t, err)
		o.adminPassword = string(hash)
		o.adminPasswordNeedChange = false
		o.AfsSystems = "1,2"
		o.AudioConversion = AUDIO_CONVERSION_ENABLED_LOUD_NORM
		o.AudioBitrate = 64
		o.AutoPopulate = !defaults.options.autoPopulate
		o.Branding = "Test Scanner"
		o.DimmerDelay = 1234
		o.DisableDuplicateDetection = true
		o.DuplicateDetectionTimeFrame = 750
		o.KeypadBeeps = "none"
		o.MaxClients = 42
		o.PlaybackGoesLive = true
		o.PruneCallDays = 14
		o.PruneLogDays = 3
		o.SearchPatchedTalkgroups = true
		o.ShowListenersCount = true
		o.SortTalkgroups = true
		o.TagsToggle = true
		o.Time12hFormat = true
		want := optionsSnapshot(o)
		must(t, o.Write(db))

		assertEqual(t, "after first write", optionsSnapshot(readOptions(t, db)), want)

		// Writing identical values again must succeed and change nothing.
		must(t, o.Write(db))
		must(t, o.Write(db))
		assertEqual(t, "after identical writes", optionsSnapshot(readOptions(t, db)), want)

		// Change them again: the update path, not the insert path.
		o.adminPasswordNeedChange = true
		o.Branding = ""
		o.MaxClients = 7
		o.DisableDuplicateDetection = false
		o.SearchPatchedTalkgroups = false
		o.Time12hFormat = false
		o.AudioConversion = AUDIO_CONVERSION_DISABLED
		want = optionsSnapshot(o)
		must(t, o.Write(db))
		got := readOptions(t, db)
		assertEqual(t, "after update", optionsSnapshot(got), want)
		if err := bcrypt.CompareHashAndPassword([]byte(got.adminPassword), []byte("s3cret-test")); err != nil {
			t.Errorf("admin password does not verify after round trip: %v", err)
		}
	})
}

// The JWT signing secret must exist once options have been written and read
// back, and must be stable across reads. Nothing ever generates or writes it,
// so admin tokens are signed with an empty HMAC key.
func TestOptionsSecret(t *testing.T) {

	forEachDatabase(t, func(t *testing.T, db *Database) {
		o := readOptions(t, db)
		must(t, o.Write(db))

		first := readOptions(t, db)
		if first.secret == "" {
			t.Fatal("no secret after Write+Read on a fresh database")
		}
		must(t, first.Write(db))
		second := readOptions(t, db)
		assertEqual(t, "secret stable across writes", second.secret, first.secret)
	})
}
