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
	"context"
	"testing"
	"time"

	"github.com/USA-RedDragon/rdio-scanner/server/ent/call"
	"github.com/USA-RedDragon/rdio-scanner/server/ent/talkgroup"
)

// TestEntSchemaMatchesLegacyTables writes and reads every entity through ent
// on a database built by the legacy migrations, so a column name, type or
// key that doesn't match the real tables fails here.
func TestEntSchemaMatchesLegacyTables(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, db *Database) {
		ctx := context.Background()
		c := db.Ent
		when := time.Date(2026, 9, 30, 12, 34, 56, 0, time.UTC)
		intp := func(v int) *int { return &v }
		strp := func(v string) *string { return &v }

		// Seeded by NewDatabase through ent.
		if n := c.Group.Query().CountX(ctx); n == 0 {
			t.Fatal("no seeded groups")
		}
		if n := c.Tag.Query().CountX(ctx); n == 0 {
			t.Fatal("no seeded tags")
		}

		acc := c.Access.Create().SetCode("abc").SetSystems(`"*"`).SetNillableLimit(intp(3)).SetNillableOrder(intp(1)).SetExpiration(when).SaveX(ctx)
		if got := c.Access.GetX(ctx, acc.ID); got.Limit == nil || *got.Limit != 3 || got.Expiration == nil || !got.Expiration.Equal(when) || got.Ident != nil {
			t.Fatalf("access round trip: %+v", got)
		}

		key := c.Apikey.Create().SetKey("k1").SetSystems(`"*"`).SetDisabled(true).SaveX(ctx)
		if got := c.Apikey.GetX(ctx, key.ID); got.Disabled == nil || !*got.Disabled {
			t.Fatalf("apikey round trip: %+v", got)
		}
		c.Apikey.UpdateOneID(key.ID).SetDisabled(false).SetIdent("x").ExecX(ctx)

		c.Downstream.Create().SetAPIKey("dk").SetURL("http://example").SetSystems("[]").ExecX(ctx)
		c.Dirwatch.Create().SetDirectory("/tmp/x").SetDelay(250).SetUsePolling(true).SetNillableSystemID(intp(1)).ExecX(ctx)
		c.Setting.Create().SetKey("k").SetVal(`"v"`).ExecX(ctx)
		c.LogEntry.Create().SetDateTime(when).SetLevel("info").SetMessage("hello").ExecX(ctx)

		sys := c.System.Create().SetSystemID(11).SetLabel("Sys").SetBlacklists("[]").SetNillableLed(strp("red")).SetOrder(1).SaveX(ctx)
		if got := c.System.GetX(ctx, sys.ID); got.SystemID != 11 || got.Led == nil || *got.Led != "red" {
			t.Fatalf("system round trip: %+v", got)
		}
		c.Talkgroup.Create().SetSystemID(11).SetTalkgroupID(100).SetLabel("TG").SetName("Talkgroup").SetGroupID(1).SetTagID(1).SetOrder(2).ExecX(ctx)
		c.Unit.Create().SetSystemID(11).SetUnitID(7).SetLabel("Unit").SetOrder(3).ExecX(ctx)
		// Reserved-word column ("order") in an update.
		c.Unit.Update().SetOrder(4).ExecX(ctx)
		tg := c.Talkgroup.Query().Where(talkgroup.SystemID(11), talkgroup.TalkgroupID(100)).OnlyX(ctx)
		if tg.Order == nil || *tg.Order != 2 {
			t.Fatalf("talkgroup round trip: %+v", tg)
		}

		audio := []byte{0, 1, 2, 250, 255}
		cl := c.Call.Create().SetAudio(audio).SetAudioName("a.m4a").SetDateTime(when).SetFrequencies("[]").SetPatches("[]").SetSources("[]").SetSystem(11).SetTalkgroup(100).SaveX(ctx)
		got := c.Call.GetX(ctx, cl.ID)
		if !bytes.Equal(got.Audio, audio) || !got.DateTime.UTC().Equal(when) || got.Frequency != nil {
			t.Fatalf("call round trip: %+v", got)
		}
		if n := c.Call.Query().Where(call.DateTimeGTE(when.Add(-time.Second)), call.DateTimeLTE(when.Add(time.Second)), call.System(11)).CountX(ctx); n != 1 {
			t.Fatalf("call time-range query: got %d, want 1", n)
		}

		// The legacy code must still read what ent wrote.
		groups := NewGroups()
		if err := groups.Read(db); err != nil {
			t.Fatal(err)
		}
		systems := NewSystems()
		if err := systems.Read(db); err != nil {
			t.Fatal(err)
		}

		for _, n := range []int{
			c.Access.Delete().ExecX(ctx), c.Apikey.Delete().ExecX(ctx), c.Downstream.Delete().ExecX(ctx),
			c.Dirwatch.Delete().ExecX(ctx), c.LogEntry.Delete().ExecX(ctx), c.Call.Delete().ExecX(ctx),
			c.Unit.Delete().ExecX(ctx), c.Talkgroup.Delete().ExecX(ctx), c.System.Delete().ExecX(ctx),
		} {
			if n == 0 {
				t.Fatal("delete removed nothing")
			}
		}
	})
}
