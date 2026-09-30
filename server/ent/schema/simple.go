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

// Group is a talkgroup group (rdioScannerGroups).
type Group struct{ ent.Schema }

func (Group) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannergroups"}}
}

func (Group) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.String("label").StorageKey("label").SchemaType(varcharType),
	}
}

// Tag is a talkgroup tag (rdioScannerTags).
type Tag struct{ ent.Schema }

func (Tag) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannertags"}}
}

func (Tag) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.String("label").StorageKey("label").SchemaType(varcharType),
	}
}

// Setting is a key/value setting (rdioScannerConfigs; named Setting because
// ent reserves "config"). Values are JSON text.
type Setting struct{ ent.Schema }

func (Setting) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannerconfigs"}}
}

func (Setting) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.String("key").StorageKey("key").SchemaType(varcharType).Unique(),
		field.Text("val").StorageKey("val").SchemaType(textType),
	}
}

func (Setting) Indexes() []ent.Index {
	return []ent.Index{
		// Redundant with the unique constraint, but it exists in every
		// legacy database.
		index.Fields("key").StorageKey("rdio_scanner_configs_key"),
	}
}

// LogEntry is an application log event (rdioScannerLogs; ent reserves "Log").
type LogEntry struct{ ent.Schema }

func (LogEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannerlogs"}}
}

func (LogEntry) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.Time("date_time").StorageKey("datetime").SchemaType(dateTimeType),
		field.String("level").StorageKey("level").SchemaType(varcharType),
		field.String("message").StorageKey("message").SchemaType(varcharType),
	}
}

func (LogEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("date_time", "level").StorageKey("rdio_scanner_logs_date_time_level"),
	}
}
