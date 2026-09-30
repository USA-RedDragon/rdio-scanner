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

package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// The radio tables relate through natural ids (a system's "id", a
// talkgroup's "id"), not the "_id" row keys, and have no foreign keys, so
// they are plain fields rather than ent edges.

// System is a radio system (rdioScannerSystems). SystemID is the natural
// system id stored in the legacy "id" column; the row key is "_id".
type System struct{ ent.Schema }

func (System) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannersystems"}}
}

func (System) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.Bool("auto_populate").StorageKey("autopopulate").Optional().Nillable().Default(false),
		// "[1,2,3]": talkgroup ids to ignore.
		field.Text("blacklists").StorageKey("blacklists").SchemaType(textType),
		field.Int("system_id").StorageKey("id").SchemaType(intType).Unique(),
		field.String("label").StorageKey("label").SchemaType(varcharType),
		field.String("led").StorageKey("led").SchemaType(varcharType).Optional().Nillable(),
		field.Int("order").StorageKey("order").SchemaType(intType).Optional().Nillable(),
	}
}

// Talkgroup belongs to a system by natural ids (systemId, id)
// (rdioScannerTalkgroups).
type Talkgroup struct{ ent.Schema }

func (Talkgroup) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannertalkgroups"}}
}

func (Talkgroup) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.Int("frequency").StorageKey("frequency").SchemaType(intType).Optional().Nillable(),
		// rdioScannerGroups._id
		field.Int("group_id").StorageKey("groupid").SchemaType(intType),
		field.Int("talkgroup_id").StorageKey("id").SchemaType(intType),
		field.String("label").StorageKey("label").SchemaType(varcharType),
		field.String("led").StorageKey("led").SchemaType(varcharType).Optional().Nillable(),
		field.String("name").StorageKey("name").SchemaType(varcharType),
		field.Int("order").StorageKey("order").SchemaType(intType).Optional().Nillable(),
		field.Int("system_id").StorageKey("systemid").SchemaType(intType),
		// rdioScannerTags._id
		field.Int("tag_id").StorageKey("tagid").SchemaType(intType),
	}
}

func (Talkgroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("system_id", "talkgroup_id").Unique().StorageKey("rdio_scanner_talkgroups_system_id_id"),
	}
}

// Unit is a radio unit of a system (rdioScannerUnits).
type Unit struct{ ent.Schema }

func (Unit) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannerunits"}}
}

func (Unit) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.Int("unit_id").StorageKey("id").SchemaType(intType),
		field.String("label").StorageKey("label").SchemaType(varcharType),
		field.Int("order").StorageKey("order").SchemaType(intType).Optional().Nillable(),
		field.Int("system_id").StorageKey("systemid").SchemaType(intType),
	}
}

func (Unit) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("system_id", "unit_id").Unique().StorageKey("rdio_scanner_units_system_id_id"),
	}
}

// Call is a recorded transmission (rdioScannerCalls). Its row key is "id"
// (not "_id"). Frequencies, patches and sources are JSON text.
type Call struct{ ent.Schema }

func (Call) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannercalls"}}
}

func (Call) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("id").SchemaType(intType),
		field.Bytes("audio").StorageKey("audio").SchemaType(blobType),
		field.String("audio_name").StorageKey("audioname").SchemaType(varcharType).Optional().Nillable(),
		field.String("audio_type").StorageKey("audiotype").SchemaType(varcharType).Optional().Nillable(),
		field.Time("date_time").StorageKey("datetime").SchemaType(dateTimeType),
		field.Text("frequencies").StorageKey("frequencies").SchemaType(textType),
		field.Int("frequency").StorageKey("frequency").SchemaType(intType).Optional().Nillable(),
		field.Text("patches").StorageKey("patches").SchemaType(textType),
		field.Int("source").StorageKey("source").SchemaType(intType).Optional().Nillable(),
		field.Text("sources").StorageKey("sources").SchemaType(textType),
		// Natural ids (rdioScannerSystems.id, rdioScannerTalkgroups.id).
		field.Int("system").StorageKey("system").SchemaType(intType),
		field.Int("talkgroup").StorageKey("talkgroup").SchemaType(intType),
	}
}

func (Call) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("date_time", "system", "talkgroup").StorageKey("rdio_scanner_calls_date_time_system_talkgroup"),
	}
}
