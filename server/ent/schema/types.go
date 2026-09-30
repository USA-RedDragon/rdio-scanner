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

// Package schema describes the rdio-scanner tables exactly as the legacy
// migrations (server/database.go) leave them, so that ent can read and write
// existing databases without any data migration.
//
// Naming: every table and column is spelled in lowercase. PostgreSQL stores
// the legacy names lowercase (they were created unquoted), SQLite and
// MySQL/MariaDB match column names case-insensitively, and the baseline
// migration renames the MySQL/MariaDB tables (the only case-sensitive ones)
// to lowercase. ent always quotes identifiers, so the names below must be the
// exact stored names on PostgreSQL.
//
// Types: each column's per-dialect type matches the legacy DDL, so Atlas
// diffs against this schema describe the real databases.
package schema

import (
	"entgo.io/ent/dialect"
)

// Legacy per-dialect column types.
var (
	blobType = map[string]string{
		dialect.SQLite:   "longblob",
		dialect.MySQL:    "longblob",
		dialect.Postgres: "bytea",
	}
	dateTimeType = map[string]string{
		dialect.SQLite:   "datetime",
		dialect.MySQL:    "datetime",
		dialect.Postgres: "timestamp",
	}
	intType = map[string]string{
		dialect.SQLite:   "integer",
		dialect.MySQL:    "int",
		dialect.Postgres: "integer",
	}
	textType = map[string]string{
		dialect.SQLite:   "text",
		dialect.MySQL:    "text",
		dialect.Postgres: "text",
	}
	varcharType = map[string]string{
		dialect.SQLite:   "varchar(255)",
		dialect.MySQL:    "varchar(255)",
		dialect.Postgres: "varchar(255)",
	}
)
