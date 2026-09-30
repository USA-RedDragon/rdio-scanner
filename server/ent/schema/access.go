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
)

// Access is a listener access code (rdioScannerAccesses). Systems is JSON text:
// "*" or a list of {id, talkgroups}.
type Access struct{ ent.Schema }

func (Access) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscanneraccesses"}}
}

func (Access) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.String("code").StorageKey("code").SchemaType(varcharType).Unique(),
		field.Time("expiration").StorageKey("expiration").SchemaType(dateTimeType).Optional().Nillable(),
		field.String("ident").StorageKey("ident").SchemaType(varcharType).Optional().Nillable(),
		field.Int("limit").StorageKey("limit").SchemaType(intType).Optional().Nillable(),
		field.Int("order").StorageKey("order").SchemaType(intType).Optional().Nillable(),
		field.Text("systems").StorageKey("systems").SchemaType(textType),
	}
}

// Apikey is an upload API key (rdioScannerApiKeys). Systems is JSON text.
type Apikey struct{ ent.Schema }

func (Apikey) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannerapikeys"}}
}

func (Apikey) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.Bool("disabled").StorageKey("disabled").Optional().Nillable().Default(false),
		field.String("ident").StorageKey("ident").SchemaType(varcharType).Optional().Nillable(),
		field.String("key").StorageKey("key").SchemaType(varcharType).Unique(),
		field.Int("order").StorageKey("order").SchemaType(intType).Optional().Nillable(),
		field.Text("systems").StorageKey("systems").SchemaType(textType),
	}
}

// Downstream is a server that received calls are forwarded to
// (rdioScannerDownstreams). Systems is JSON text.
type Downstream struct{ ent.Schema }

func (Downstream) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannerdownstreams"}}
}

func (Downstream) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.String("api_key").StorageKey("apikey").SchemaType(varcharType),
		field.Bool("disabled").StorageKey("disabled").Optional().Nillable().Default(false),
		field.Int("order").StorageKey("order").SchemaType(intType).Optional().Nillable(),
		field.Text("systems").StorageKey("systems").SchemaType(textType),
		field.String("url").StorageKey("url").SchemaType(varcharType),
	}
}

// Dirwatch is a watched directory that calls are ingested from
// (rdioScannerDirWatches).
type Dirwatch struct{ ent.Schema }

func (Dirwatch) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "rdioscannerdirwatches"}}
}

func (Dirwatch) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").StorageKey("_id").SchemaType(intType),
		field.Int("delay").StorageKey("delay").SchemaType(intType).Optional().Nillable().Default(0),
		field.Bool("delete_after").StorageKey("deleteafter").Optional().Nillable().Default(false),
		field.String("directory").StorageKey("directory").SchemaType(varcharType).Unique(),
		field.Bool("disabled").StorageKey("disabled").Optional().Nillable().Default(false),
		field.String("extension").StorageKey("extension").SchemaType(varcharType).Optional().Nillable(),
		field.Int("frequency").StorageKey("frequency").SchemaType(intType).Optional().Nillable(),
		field.String("mask").StorageKey("mask").SchemaType(varcharType).Optional().Nillable(),
		field.Int("order").StorageKey("order").SchemaType(intType).Optional().Nillable(),
		// Natural ids (rdioScannerSystems.id, rdioScannerTalkgroups.id).
		field.Int("system_id").StorageKey("systemid").SchemaType(intType).Optional().Nillable(),
		field.Int("talkgroup_id").StorageKey("talkgroupid").SchemaType(intType).Optional().Nillable(),
		field.String("type").StorageKey("type").SchemaType(varcharType).Optional().Nillable(),
		field.Bool("use_polling").StorageKey("usepolling").Optional().Nillable().Default(false),
	}
}
