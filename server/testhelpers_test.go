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

// Shared helpers for the data layer tests.

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// must fails the test immediately on a non-nil error.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// normalizeJSON turns any JSON-able value into its generic decoded form
// (map[string]any, []any, float64, string, bool, nil) so values of different
// Go types that serialise identically compare equal. A string holding JSON
// (the form FromMap produces for "systems") is decoded; any other string is
// kept as is (e.g. the "*" systems wildcard).
func normalizeJSON(t *testing.T, v any) any {
	t.Helper()
	if s, ok := v.(string); ok {
		var out any
		if err := json.Unmarshal([]byte(s), &out); err == nil {
			return out
		}
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("normalizeJSON: %v", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("normalizeJSON: %v", err)
	}
	return out
}

// assertJSONEqual compares two values by their normalised JSON form.
func assertJSONEqual(t *testing.T, what string, got, want any) {
	t.Helper()
	g, w := normalizeJSON(t, got), normalizeJSON(t, want)
	if !reflect.DeepEqual(g, w) {
		t.Errorf("%s: got %#v, want %#v", what, g, w)
	}
}

// sameSecond reports whether two instants are equal at second precision.
func sameSecond(a, b time.Time) bool {
	return a.UTC().Truncate(time.Second).Equal(b.UTC().Truncate(time.Second))
}

// assertSameSecond fails when two instants differ at second precision.
func assertSameSecond(t *testing.T, what string, got, want time.Time) {
	t.Helper()
	if !sameSecond(got, want) {
		t.Errorf("%s: got %v, want %v", what, got.UTC(), want.UTC())
	}
}

// assertEqual is a small reflect.DeepEqual assertion.
func assertEqual(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %#v, want %#v", what, got, want)
	}
}

// testClient returns a Client that is good enough for Calls.Search: no
// websocket, just a controller holding the database.
func testClient(db *Database) *Client {
	return &Client{Controller: &Controller{Database: db}}
}

// withLocalZone sets time.Local to zone for the rest of the test.
// Used to make time-zone dependent behaviour deterministic.
func withLocalZone(t *testing.T, zone *time.Location) {
	t.Helper()
	prev := time.Local
	time.Local = zone
	t.Cleanup(func() { time.Local = prev })
}
