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
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	entdialect "entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/USA-RedDragon/rdio-scanner/server/ent"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type Database struct {
	Config         *Config
	DateTimeFormat string
	Sql            *sql.DB
	// Ent is the typed client over the same connection pool as Sql. Data
	// access moves to it table by table; Sql remains for the legacy
	// migrations.
	Ent *ent.Client
}

func NewDatabase(config *Config) *Database {
	var err error

	database := &Database{Config: config}

	switch config.DbType {
	case DbTypeSqlite:
		database.DateTimeFormat = "2006-01-02 15:04:05.000 -07:00"

		dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout%%3d10000", config.GetDbFilePath())

		if database.Sql, err = sql.Open("sqlite", dsn); err != nil {
			log.Fatal(err)
		}

	case DbTypeMariadb, DbTypeMysql:
		database.DateTimeFormat = "2006-01-02 15:04:05"

		// parseTime: datetimes scan into time.Time (ent needs it; ParseDateTime
		// accepts time.Time too).
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", config.DbUsername, config.DbPassword, config.DbHost, config.DbPort, config.DbName)

		if database.Sql, err = sql.Open("mysql", dsn); err != nil {
			log.Fatal(err)
		}

	case DbTypePostgresql:
		database.DateTimeFormat = "2006-01-02 15:04:05.000000+00"

		dsn := fmt.Sprintf("host=%s port=%d user=%s "+
			"password=%s dbname=%s sslmode=%s", config.DbHost, config.DbPort, config.DbUsername, config.DbPassword, config.DbName, config.DBSSLMode)

		if database.Sql, err = sql.Open("postgres", dsn); err != nil {
			log.Fatal(err)
		}

	default:
		log.Fatalf("unknown database type %s\n", config.DbType)
	}

	database.Sql.SetConnMaxLifetime(time.Minute)
	database.Sql.SetMaxIdleConns(25)
	database.Sql.SetMaxOpenConns(25)

	database.Ent = ent.NewClient(ent.Driver(entsql.OpenDB(database.entDialect(), database.Sql)))

	if err = database.migrate(); err != nil {
		log.Fatal(err)
	}

	if err = database.seed(); err != nil {
		log.Fatal(err)
	}

	return database
}

func (db *Database) entDialect() string {
	switch db.Config.DbType {
	case DbTypeMariadb, DbTypeMysql:
		return entdialect.MySQL
	case DbTypePostgresql:
		return entdialect.Postgres
	default:
		return entdialect.SQLite
	}
}

func (db *Database) ParseDateTime(f any) (time.Time, error) {
	switch v := f.(type) {
	case []uint8:
		return time.Parse(db.DateTimeFormat, string(v))
	case string:
		return time.Parse(db.DateTimeFormat, v)
	case time.Time:
		return v, nil
	default:
		return time.Time{}, fmt.Errorf("unknown datetime format %T", v)
	}
}

func (db *Database) migrate() error {
	var (
		err     error
		verbose bool
	)

	verbose, err = db.prepareMigration()

	if err == nil {
		err = db.migration20191028144433(verbose)
	}
	if err == nil {
		err = db.migration20191029092201(verbose)
	}
	if err == nil {
		err = db.migration20191126135515(verbose)
	}
	if err == nil {
		err = db.migration20191220093214(verbose)
	}
	if err == nil {
		err = db.migration20200123094105(verbose)
	}
	if err == nil {
		err = db.migration20200428132918(verbose)
	}
	if err == nil {
		err = db.migration20210115105958(verbose)
	}
	if err == nil {
		err = db.migration20210830092027(verbose)
	}
	if err == nil {
		err = db.migration20211202094819(verbose)
	}
	if err == nil {
		err = db.migration20220101070000(verbose)
	}
	if err == nil {
		err = db.migration20260930000000(verbose)
	}

	return err
}

func (db *Database) migrateWithSchema(name string, schemas []string, verbose bool) error {
	var (
		count int = 0
		err   error
		query string
		tx    *sql.Tx
	)

	formatError := func(err error, query string) error {
		return fmt.Errorf("%s while doing %s", err.Error(), query)
	}

	if db.Config.DbType != DbTypePostgresql {
		query = fmt.Sprintf("select count(*) from `rdioScannerMeta` where `name` = '%s'", name)
	} else {
		query = fmt.Sprintf("select count(*) from rdioScannerMeta where name = '%s'", name)
	}
	if err = db.Sql.QueryRow(query).Scan(&count); err != nil {
		return formatError(err, query)
	}

	if count == 0 {
		if verbose {
			log.Printf("running database migration %s", name)
		}

		if tx, err = db.Sql.Begin(); err == nil {
			for _, query = range schemas {
				if _, err = tx.Exec(query); err != nil {
					tx.Rollback()
					return formatError(err, query)
				}
			}

			if db.Config.DbType != DbTypePostgresql {
				query = fmt.Sprintf("insert into `rdioScannerMeta` (`name`) values ('%s')", name)
			} else {
				query = fmt.Sprintf("insert into rdioScannerMeta (name) values ('%s')", name)
			}
			if _, err = tx.Exec(query); err != nil {
				tx.Rollback()
				return formatError(err, query)
			}

			if err = tx.Commit(); err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return nil
}

func (db *Database) migration20191028144433(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"create table `rdioScannerSystems` (`id` integer primary key autoincrement, `createdAt` datetime not null, `updatedAt` datetime not null, `name` varchar(255) not null, `system` integer not null, `talkgroups` json not null)",
			"create unique index `rdio_scanner_systems_system` on `rdioScannerSystems` (`system`)",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"create table rdioScannerSystems (id serial primary key, createdAt timestamp not null, updatedAt timestamp not null, name varchar(255) not null, system integer not null, talkgroups json not null)",
			"create unique index rdio_scanner_systems_system on rdioScannerSystems (system)",
		}
	} else {
		queries = []string{
			"create table `rdioScannerSystems` (`id` integer primary key auto_increment, `createdAt` datetime not null, `updatedAt` datetime not null, `name` varchar(255) not null, `system` integer not null, `talkgroups` json not null)",
			"create unique index `rdio_scanner_systems_system` on `rdioScannerSystems` (`system`)",
		}
	}
	return db.migrateWithSchema("20191028144433-create-rdio-scanner-system", queries, verbose)
}

func (db *Database) migration20191029092201(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"create table `rdioScannerCalls` (`id` integer primary key autoincrement, `createdAt` datetime not null, `updatedAt` datetime not null, `audio` longblob not null, `emergency` tinyint(1) not null, `freq` integer not null, `freqList` json not null, `startTime` datetime not null, `stopTime` datetime not null, `srcList` json not null, `system` integer not null, `talkgroup` integer not null)",
			"create index `rdio_scanner_calls_start_time` on `rdioScannerCalls` (`startTime`)",
			"create index `rdio_scanner_calls_system` on `rdioScannerCalls` (`system`)",
			"create index `rdio_scanner_calls_talkgroup` on `rdioScannerCalls` (`talkgroup`)",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"create table rdioScannerCalls (id serial primary key, createdAt timestamp not null, updatedAt timestamp not null, audio bytea not null, emergency boolean not null, freq integer not null, freqList json not null, startTime timestamp not null, stopTime timestamp not null, srcList json not null, system integer not null, talkgroup integer not null)",
			"create index rdio_scanner_calls_start_time on rdioScannerCalls (startTime)",
			"create index rdio_scanner_calls_system on rdioScannerCalls (system)",
			"create index rdio_scanner_calls_talkgroup on rdioScannerCalls (talkgroup)",
		}
	} else {
		queries = []string{
			"create table `rdioScannerCalls` (`id` integer primary key auto_increment, `createdAt` datetime not null, `updatedAt` datetime not null, `audio` longblob not null, `emergency` tinyint(1) not null, `freq` integer not null, `freqList` json not null, `startTime` datetime not null, `stopTime` datetime not null, `srcList` json not null, `system` integer not null, `talkgroup` integer not null)",
			"create index `rdio_scanner_calls_start_time` on `rdioScannerCalls` (`startTime`)",
			"create index `rdio_scanner_calls_system` on `rdioScannerCalls` (`system`)",
			"create index `rdio_scanner_calls_talkgroup` on `rdioScannerCalls` (`talkgroup`)",
		}
	}
	return db.migrateWithSchema("20191029092201-create-rdio-scanner-call", queries, verbose)
}

func (db *Database) migration20191126135515(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"drop index `rdio_scanner_calls_system`",
			"drop index `rdio_scanner_calls_talkgroup`",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"drop index rdio_scanner_calls_system",
			"drop index rdio_scanner_calls_talkgroup",
		}
	} else {
		queries = []string{
			"drop index `rdio_scanner_calls_system` on `rdioScannerCalls`",
			"drop index `rdio_scanner_calls_talkgroup` on `rdioScannerCalls`",
		}
	}
	return db.migrateWithSchema("20191126135515-optimize-rdio-scanner-calls", queries, verbose)
}

func (db *Database) migration20191220093214(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"alter table `rdioScannerCalls` add column `audioName` varchar(255)",
			"alter table `rdioScannerCalls` add column `audioType` varchar(255)",
			"alter table `rdioScannerSystems` add column `aliases` json not null",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"alter table rdioScannerCalls add column audioName varchar(255)",
			"alter table rdioScannerCalls add column audioType varchar(255)",
			"alter table rdioScannerSystems add column aliases json not null",
		}
	} else {
		queries = []string{
			"alter table `rdioScannerCalls` add column `audioName` varchar(255)",
			"alter table `rdioScannerCalls` add column `audioType` varchar(255)",
			"alter table `rdioScannerSystems` add column `aliases` json not null",
		}
	}
	return db.migrateWithSchema("20191220093214-new-v3-tables", queries, verbose)
}

func (db *Database) migration20200123094105(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"create index `rdio_scanner_calls_system` on `rdioScannerCalls` (`system`)",
			"create index `rdio_scanner_calls_system_talkgroup` on `rdioScannerCalls` (`system`, `talkgroup`)",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"create index rdio_scanner_calls_system on rdioScannerCalls (system)",
			"create index rdio_scanner_calls_system_talkgroup on rdioScannerCalls (system, talkgroup)",
		}
	} else {
		queries = []string{
			"create index `rdio_scanner_calls_system` on `rdioScannerCalls` (`system`)",
			"create index `rdio_scanner_calls_system_talkgroup` on `rdioScannerCalls` (`system`, `talkgroup`)",
		}
	}
	return db.migrateWithSchema("20200123094105-optimize-rdio-scanner-calls", queries, verbose)
}

func (db *Database) migration20200428132918(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"drop table `rdioScannerSystems`",
			"create table `rdioScannerCalls2` (`id` integer primary key autoincrement, `audio` longblob not null, `audioName` varchar(255), `audioType` varchar(255), `dateTime` datetime not null, `frequencies` json not null, `frequency` integer, `source` integer, `sources` json not null, `system` integer not null, `talkgroup` integer not null)",
			"insert into `rdioScannerCalls2` select `id`, `audio`, `audioName`, `audioType`, `startTime`, `freqList`, `freq`, null, `srcList`, `system`, `talkgroup` from `rdioScannerCalls`",
			"drop table `rdioScannerCalls`",
			"alter table `rdioScannerCalls2` rename to `rdioScannerCalls`",
			"create index `rdio_scanner_calls_date_time_system_talkgroup` on `rdioScannerCalls` (`dateTime`, `system`, `talkgroup`)",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"drop table rdioScannerSystems",
			"create table rdioScannerCalls2 (id serial primary key, audio bytea not null, audioName varchar(255), audioType varchar(255), dateTime timestamp not null, frequencies json not null, frequency integer, source integer, sources json not null, system integer not null, talkgroup integer not null)",
			"insert into rdioScannerCalls2 select id, audio, audioName, audioType, startTime, freqList, freq, null, srcList, system, talkgroup from rdioScannerCalls",
			"drop table rdioScannerCalls",
			"alter table rdioScannerCalls2 rename to rdioScannerCalls",
			"create index rdio_scanner_calls_date_time_system_talkgroup on rdioScannerCalls (dateTime, system, talkgroup)",
		}
	} else {
		queries = []string{
			"drop table `rdioScannerSystems`",
			"create table `rdioScannerCalls2` (`id` integer primary key auto_increment, `audio` longblob not null, `audioName` varchar(255), `audioType` varchar(255), `dateTime` datetime not null, `frequencies` json not null, `frequency` integer, `source` integer, `sources` json not null, `system` integer not null, `talkgroup` integer not null)",
			"insert into `rdioScannerCalls2` select `id`, `audio`, `audioName`, `audioType`, `startTime`, `freqList`, `freq`, null, `srcList`, `system`, `talkgroup` from `rdioScannerCalls`",
			"drop table `rdioScannerCalls`",
			"alter table `rdioScannerCalls2` rename to `rdioScannerCalls`",
			"create index `rdio_scanner_calls_date_time_system_talkgroup` on `rdioScannerCalls` (`dateTime`, `system`, `talkgroup`)",
		}
	}
	return db.migrateWithSchema("20200428132918-new-v4-tables", queries, verbose)
}

func (db *Database) migration20210115105958(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"create table `rdioScannerAccesses` (`_id` integer primary key autoincrement, `code` varchar(255) not null unique, `expiration` datetime, `ident` varchar(255), `limit` integer, `order` integer, `systems` text not null)",
			"create table `rdioScannerApiKeys` (`_id` integer primary key autoincrement, `disabled` tinyint(1) default 0, `ident` varchar(255), `key` varchar(255) not null unique, `order` integer, `systems` text not null)",
			"create table `rdioScannerCalls2` (`id` integer primary key autoincrement, `audio` longblob not null, `audioName` varchar(255), `audioType` varchar(255), `dateTime` datetime not null, `frequencies` text not null, `frequency` integer, `source` integer, `sources` text not null, `system` integer not null, `talkgroup` integer not null)",
			"create index `rdio_scanner_calls2_date_time_system_talkgroup` on `rdioScannerCalls2` (`dateTime`, `system`, `talkgroup`)",
			"insert into `rdioScannerCalls2` select `id`, `audio`, `audioName`, `audioType`, `dateTime`, `frequencies`, `frequency`, `source`, `sources`, `system`, `talkgroup` from `rdioScannerCalls`",
			"drop table `rdioScannerCalls`",
			"alter table `rdioScannerCalls2` rename to `rdioScannerCalls`",
			"create table `rdioScannerConfigs` (`_id` integer primary key autoincrement, `key` varchar(255) not null unique, `val` text not null)",
			"create index `rdio_scanner_configs_key` on `rdioScannerConfigs` (`key`)",
			"create table `rdioScannerDirWatches` (`_id` integer primary key autoincrement, `delay` integer default 0, `deleteAfter` tinyint(1) default 0, `directory` varchar(255) not null unique, `disabled` tinyint(1) default 0, `extension` varchar(255), `frequency` integer, `mask` varchar(255), `order` integer, `systemId` integer, `talkgroupId` integer, `type` varchar(255), `usePolling` tinyint(1) default 0)",
			"create table `rdioScannerDownstreams` (`_id` integer primary key autoincrement, `apiKey` varchar(255) not null unique, `disabled` tinyint(1) default 0, `order` integer, `systems` text not null, `url` varchar(255) not null)",
			"create table `rdioScannerGroups` (`_id` integer primary key autoincrement, `label` varchar(255) not null)",
			"create table `rdioScannerLogs` (`_id` integer primary key autoincrement, `dateTime` datetime not null, `level` varchar(255) not null, `message` varchar(255) not null)",
			"create index `rdio_scanner_logs_date_time_level` on `rdioScannerLogs` (`dateTime`, `level`)",
			"create table `rdioScannerSystems` (`_id` integer primary key autoincrement, `autoPopulate` tinyint(1) default 0, `blacklists` text not null, `id` integer not null unique, `label` varchar(255) not null, `led` varchar(255), `order` integer, `talkgroups` text not null, `units` text not null)",
			"create table `rdioScannerTags` (`_id` integer primary key autoincrement, `label` varchar(255) not null)",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"create table rdioScannerAccesses (_id serial primary key, code varchar(255) not null unique, expiration timestamp, ident varchar(255), \"limit\" integer, \"order\" integer, systems text not null)",
			"create table rdioScannerApiKeys (_id serial primary key, disabled boolean default false, ident varchar(255), key varchar(255) not null unique, \"order\" integer, systems text not null)",
			"create table rdioScannerCalls2 (id serial primary key, audio bytea not null, audioName varchar(255), audioType varchar(255), dateTime timestamp not null, frequencies text not null, frequency integer, source integer, sources text not null, system integer not null, talkgroup integer not null)",
			"create index rdio_scanner_calls2_date_time_system_talkgroup on rdioScannerCalls2 (dateTime, system, talkgroup)",
			"insert into rdioScannerCalls2 select id, audio, audioName, audioType, dateTime, frequencies, frequency, source, sources, system, talkgroup from rdioScannerCalls",
			"drop table rdioScannerCalls",
			"alter table rdioScannerCalls2 rename to rdioScannerCalls",
			"create table rdioScannerConfigs (_id serial primary key, key varchar(255) not null unique, val text not null)",
			"create index rdio_scanner_configs_key on rdioScannerConfigs (key)",
			"create table rdioScannerDirWatches (_id serial primary key, delay integer default 0, deleteAfter boolean default false, directory varchar(255) not null unique, disabled boolean default false, extension varchar(255), frequency integer, mask varchar(255), \"order\" integer, systemId integer, talkgroupId integer, type varchar(255), usePolling boolean default false)",
			"create table rdioScannerDownstreams (_id serial primary key, apiKey varchar(255) not null unique, disabled boolean default false, \"order\" integer, systems text not null, url varchar(255) not null)",
			"create table rdioScannerGroups (_id serial primary key, label varchar(255) not null)",
			"create table rdioScannerLogs (_id serial primary key, dateTime timestamp not null, level varchar(255) not null, message varchar(255) not null)",
			"create index rdio_scanner_logs_date_time_level on rdioScannerLogs (dateTime, level)",
			"create table rdioScannerSystems (_id serial primary key, autoPopulate boolean default false, blacklists text not null, id integer not null unique, label varchar(255) not null, led varchar(255), \"order\" integer, talkgroups text not null, units text not null)",
			"create table rdioScannerTags (_id serial primary key, label varchar(255) not null)",
		}
	} else {
		queries = []string{
			"create table `rdioScannerAccesses` (`_id` integer primary key auto_increment, `code` varchar(255) not null unique, `expiration` datetime, `ident` varchar(255), `limit` integer, `order` integer, `systems` text not null)",
			"create table `rdioScannerApiKeys` (`_id` integer primary key auto_increment, `disabled` tinyint(1) default 0, `ident` varchar(255), `key` varchar(255) not null unique, `order` integer, `systems` text not null)",
			"create table `rdioScannerCalls2` (`id` integer primary key auto_increment, `audio` longblob not null, `audioName` varchar(255), `audioType` varchar(255), `dateTime` datetime not null, `frequencies` text not null, `frequency` integer, `source` integer, `sources` text not null, `system` integer not null, `talkgroup` integer not null)",
			"create index `rdio_scanner_calls2_date_time_system_talkgroup` on `rdioScannerCalls2` (`dateTime`, `system`, `talkgroup`)",
			"insert into `rdioScannerCalls2` select `id`, `audio`, `audioName`, `audioType`, `dateTime`, `frequencies`, `frequency`, `source`, `sources`, `system`, `talkgroup` from `rdioScannerCalls`",
			"drop table `rdioScannerCalls`",
			"alter table `rdioScannerCalls2` rename to `rdioScannerCalls`",
			"create table `rdioScannerConfigs` (`_id` integer primary key auto_increment, `key` varchar(255) not null unique, `val` text not null)",
			"create index `rdio_scanner_configs_key` on `rdioScannerConfigs` (`key`)",
			"create table `rdioScannerDirWatches` (`_id` integer primary key auto_increment, `delay` integer default 0, `deleteAfter` tinyint(1) default 0, `directory` varchar(255) not null unique, `disabled` tinyint(1) default 0, `extension` varchar(255), `frequency` integer, `mask` varchar(255), `order` integer, `systemId` integer, `talkgroupId` integer, `type` varchar(255), `usePolling` tinyint(1) default 0)",
			"create table `rdioScannerDownstreams` (`_id` integer primary key auto_increment, `apiKey` varchar(255) not null unique, `disabled` tinyint(1) default 0, `order` integer, `systems` text not null, `url` varchar(255) not null)",
			"create table `rdioScannerGroups` (`_id` integer primary key auto_increment, `label` varchar(255) not null)",
			"create table `rdioScannerLogs` (`_id` integer primary key auto_increment, `dateTime` datetime not null, `level` varchar(255) not null, `message` varchar(255) not null)",
			"create index `rdio_scanner_logs_date_time_level` on `rdioScannerLogs` (`dateTime`, `level`)",
			"create table `rdioScannerSystems` (`_id` integer primary key auto_increment, `autoPopulate` tinyint(1) default 0, `blacklists` text not null, `id` integer not null unique, `label` varchar(255) not null, `led` varchar(255), `order` integer, `talkgroups` text not null, `units` text not null)",
			"create table `rdioScannerTags` (`_id` integer primary key auto_increment, `label` varchar(255) not null)",
		}
	}
	return db.migrateWithSchema("20210115105958-new-v5.1-tables", queries, verbose)
}

func (db *Database) migration20210830092027(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"create table `rdioScannerSystems2` (`_id` integer primary key autoincrement, `autoPopulate` tinyint(1) default 0, `blacklists` text not null, `id` integer not null unique, `label` varchar(255) not null, `led` varchar(255), `order` integer, `talkgroups` longtext not null, `units` longtext not null)",
			"insert into `rdioScannerSystems2` select `_id`, `autoPopulate`, `blacklists`, `id`, `label`, `led`, `order`, `talkgroups`, `units` from `rdioScannerSystems`",
			"drop table `rdioScannerSystems`",
			"alter table `rdioScannerSystems2` rename to `rdioScannerSystems`",
			"drop index `rdio_scanner_calls2_date_time_system_talkgroup`",
			"create index `rdio_scanner_calls_date_time_system_talkgroup` on `rdioScannerCalls` (`dateTime`, `system`, `talkgroup`)",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"create table rdioScannerSystems2 (_id serial primary key, autoPopulate boolean default false, blacklists text not null, id integer not null unique, label varchar(255) not null, led varchar(255), \"order\" integer, talkgroups text not null, units text not null)",
			"insert into rdioScannerSystems2 select _id, autoPopulate, blacklists, id, label, led, \"order\", talkgroups, units from rdioScannerSystems",
			"drop table rdioScannerSystems",
			"alter table rdioScannerSystems2 rename to rdioScannerSystems",
			"drop index rdio_scanner_calls2_date_time_system_talkgroup",
			"create index rdio_scanner_calls_date_time_system_talkgroup on rdioScannerCalls (dateTime, system, talkgroup)",
		}
	} else {
		queries = []string{
			"alter table `rdioScannerSystems` modify `talkgroups` longtext null not null",
			"alter table `rdioScannerSystems` modify `units` longtext null not null",
			"drop index `rdio_scanner_calls2_date_time_system_talkgroup` on `rdioScannerCalls`",
			"create index `rdio_scanner_calls_date_time_system_talkgroup` on `rdioScannerCalls` (`dateTime`, `system`, `talkgroup`)",
		}
	}
	return db.migrateWithSchema("20210830092027-v6.0-rename-index", queries, verbose)
}

func (db *Database) migration20211202094819(verbose bool) error {
	var queries []string
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"alter table `rdioScannerDownstreams` rename to `rdioScannerDownstreams2`",
			"create table `rdioScannerDownstreams` (`_id` integer primary key autoincrement, `apiKey` varchar(255) not null, `disabled` tinyint(1) default 0, `order` integer, `systems` text not null, `url` varchar(255) not null)",
			"insert into `rdioScannerDownstreams` select * from `rdioScannerDownstreams2`",
			"drop table `rdioScannerDownstreams2`",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"alter table rdioScannerDownstreams rename to rdioScannerDownstreams2",
			"create table rdioScannerDownstreams (_id serial primary key, apiKey varchar(255) not null, disabled boolean default false, \"order\" integer, systems text not null, url varchar(255) not null)",
			"insert into rdioScannerDownstreams select * from rdioScannerDownstreams2",
			"drop table rdioScannerDownstreams2",
		}
	} else {
		queries = []string{
			"alter table `rdioScannerDownstreams` rename to `rdioScannerDownstreams2`",
			"create table `rdioScannerDownstreams` (`_id` integer primary key auto_increment, `apiKey` varchar(255) not null, `disabled` tinyint(1) default 0, `order` integer, `systems` text not null, `url` varchar(255) not null)",
			"insert into `rdioScannerDownstreams` select * from `rdioScannerDownstreams2`",
			"drop table `rdioScannerDownstreams2`",
		}
	}
	return db.migrateWithSchema("20211202094819-v6.0.2-alter-table", queries, verbose)
}

func (db *Database) migration20220101070000(verbose bool) error {
	var (
		err        error
		frequency  any
		id         uint
		label      string
		led        any
		name       string
		queries    []string
		rows       *sql.Rows
		stra       string
		strb       string
		talkgroups []*Talkgroup
		units      []*Unit
	)
	if db.Config.DbType == DbTypeSqlite {
		queries = []string{
			"create table `rdioScannerCalls2` (`id` integer primary key autoincrement, `audio` longblob not null, `audioName` varchar(255), `audioType` varchar(255), `dateTime` datetime not null, `frequencies` text not null, `frequency` integer, `patches` text not null, `source` integer, `sources` text not null, `system` integer not null, `talkgroup` integer not null)",
			"insert into `rdioScannerCalls2` select `id`, `audio`, `audioName`, `audioType`, `dateTime`, `frequencies`, `frequency`, '[]', `source`, `sources`, `system`, `talkgroup` from `rdioScannerCalls`",
			"drop table `rdioScannerCalls`",
			"alter table `rdioScannerCalls2` rename to `rdioScannerCalls`",
			"create index `rdio_scanner_calls_date_time_system_talkgroup` on `rdioScannerCalls` (`dateTime`, `system`, `talkgroup`)",
			"create table `rdioScannerSystems2` (`_id` integer primary key autoincrement, `autoPopulate` tinyint(1) default 0, `blacklists` text not null, `id` integer not null unique, `label` varchar(255) not null, `led` varchar(255), `order` integer)",
			"insert into `rdioScannerSystems2` select `_id`, `autoPopulate`, `blacklists`, `id`, `label`, `led`, `order` from `rdioScannerSystems`",
			"drop table `rdioScannerSystems`",
			"alter table `rdioScannerSystems2` rename to `rdioScannerSystems`",
			"create table `rdioScannerTalkgroups` (`_id` integer primary key autoincrement, `frequency` integer, `groupId` integer not null, `id` integer not null, `label` varchar(255) not null, `led` varchar(255), `name` varchar(255) not null, `order` integer, `systemId` integer not null, `tagId` integer not null)",
			"create unique index `rdio_scanner_talkgroups_system_id_id` on `rdioScannerTalkgroups` (`systemId`, `id`)",
			"create table `rdioScannerUnits` (`_id` integer primary key autoincrement, `id` integer not null, `label` varchar(255) not null, `order` integer, `systemId` integer not null)",
			"create unique index `rdio_scanner_units_system_id_id` on `rdioScannerUnits` (`systemId`, `id`)",
		}
	} else if db.Config.DbType == DbTypePostgresql {
		queries = []string{
			"alter table rdioScannerCalls add column patches text not null",
			"alter table rdioScannerSystems drop column talkgroups",
			"alter table rdioScannerSystems drop column units",
			"create table rdioScannerTalkgroups (_id serial primary key, frequency integer, groupId integer not null, id integer not null, label varchar(255) not null, led varchar(255), name varchar(255) not null, \"order\" integer, systemId integer not null, tagId integer not null)",
			"create unique index rdio_scanner_talkgroups_system_id_id on rdioScannerTalkgroups (systemId, id)",
			"create table rdioScannerUnits (_id serial primary key, id integer not null, label varchar(255) not null, \"order\" integer, systemId integer not null)",
			"create unique index rdio_scanner_units_system_id_id on rdioScannerUnits (systemId, id)",
		}
	} else {
		queries = []string{
			"alter table `rdioScannerCalls` add column `patches` text not null",
			"alter table `rdioScannerSystems` drop column `talkgroups`",
			"alter table `rdioScannerSystems` drop column `units`",
			"create table `rdioScannerTalkgroups` (`_id` integer primary key auto_increment, `frequency` integer, `groupId` integer not null, `id` integer not null, `label` varchar(255) not null, `led` varchar(255), `name` varchar(255) not null, `order` integer, `systemId` integer not null, `tagId` integer not null)",
			"create unique index `rdio_scanner_talkgroups_system_id_id` on `rdioScannerTalkgroups` (`systemId`, `id`)",
			"create table `rdioScannerUnits` (`_id` integer primary key auto_increment, `id` integer not null, `label` varchar(255) not null, `order` integer, `systemId` integer not null)",
			"create unique index `rdio_scanner_units_system_id_id` on `rdioScannerUnits` (`systemId`, `id`)",
		}
	}
	q := "select `id`, `talkgroups`, `units` from `rdioScannerSystems`"
	if db.Config.DbType == DbTypePostgresql {
		q = "select id, talkgroups, units from rdioScannerSystems"
	}
	if rows, err = db.Sql.Query(q); err == nil {
		for rows.Next() {
			if err = rows.Scan(&id, &stra, &strb); err != nil {
				break
			}
			if err = json.Unmarshal([]byte(stra), &talkgroups); err != nil {
				break
			}
			if err = json.Unmarshal([]byte(strb), &units); err != nil {
				break
			}
			for i, tg := range talkgroups {
				switch v := tg.Frequency.(type) {
				case uint:
					frequency = v
				default:
					frequency = "null"
				}
				label = strings.ReplaceAll(tg.Label, "'", "''")
				switch v := tg.Led.(type) {
				case string:
					led = fmt.Sprintf("'%v'", strings.ReplaceAll(v, "'", "''"))
				default:
					led = "null"
				}
				name = strings.ReplaceAll(tg.Name, "'", "''")
				tg.Order = uint(i + 1)
				if db.Config.DbType != DbTypePostgresql {
					queries = append(queries, fmt.Sprintf("insert into `rdioScannerTalkgroups` (`frequency`, `groupId`, `id`, `label`, `led`, `name`, `order`, `systemId`, `tagId`) values (%v, %v, %v, '%v', %v, '%v', %v, %v, %v)", frequency, tg.GroupId, tg.Id, label, led, name, tg.Order, id, tg.TagId))
				} else {
					queries = append(queries, fmt.Sprintf("insert into rdioScannerTalkgroups (frequency, groupId, id, label, led, name, \"order\", systemId, tagId) values (%v, %v, %v, '%v', %v, '%v', %v, %v, %v)", frequency, tg.GroupId, tg.Id, label, led, name, tg.Order, id, tg.TagId))
				}
			}
			for i, unit := range units {
				label = strings.ReplaceAll(unit.Label, "'", "''")
				unit.Order = uint(i + 1)
				if db.Config.DbType != DbTypePostgresql {
					queries = append(queries, fmt.Sprintf("insert into `rdioScannerUnits` (`id`, `label`, `order`, `systemId`) values (%v, '%v', %v, %v)", unit.Id, label, unit.Order, id))
				} else {
					queries = append(queries, fmt.Sprintf("insert into rdioScannerUnits (id, label, \"order\", systemId) values (%v, '%v', %v, %v)", unit.Id, label, unit.Order, id))
				}
			}
		}
		rows.Close()
		if err != nil {
			return err
		}
	}
	return db.migrateWithSchema("20220101070000-v6.1.0", queries, verbose)
}

func (db *Database) prepareMigration() (bool, error) {
	var (
		err     error
		verbose bool = true
		query   string
	)

	if db.Config.DbType != DbTypePostgresql {
		query = "select count(*) as count from `rdioScannerMeta`"
		if _, err = db.Sql.Exec(query); err != nil {
			query = "select count(*) as count from `SequelizeMeta`"
			if _, err = db.Sql.Exec(query); err == nil {
				log.Println("Preparing for database migration")
				query = "alter table `SequelizeMeta` rename to `rdioScannerMeta`"
				_, err = db.Sql.Exec(query)
			} else {
				verbose = false
				query = "create table `rdioScannerMeta` (name varchar(255) not null unique primary key)"
				_, err = db.Sql.Exec(query)
			}
		}
	} else {
		query = "select count(*) as count from rdioScannerMeta"
		if _, err = db.Sql.Exec(query); err != nil {
			query = "select count(*) as count from SequelizeMeta"
			if _, err = db.Sql.Exec(query); err == nil {
				log.Println("Preparing for database migration")
				query = "alter table SequelizeMeta rename to rdioScannerMeta"
				_, err = db.Sql.Exec(query)
			} else {
				verbose = false
				query = "create table rdioScannerMeta (name varchar(255) not null unique primary key)"
				_, err = db.Sql.Exec(query)
			}
		}
	}
	return verbose, err
}

// migration20260930000000 makes every stored table and column name lowercase
// so ent and the queries can use one spelling everywhere. PostgreSQL already
// stores them lowercase (the legacy DDL was unquoted).
//   - MySQL/MariaDB match table names case-sensitively (with the default
//     lower_case_table_names=0), so the tables are renamed.
//   - SQLite matches names case-insensitively in queries but labels result
//     columns with their declared spelling ("audioName"), which ent can't map,
//     so the mixed-case columns are renamed.
//
// rdioScannerMeta keeps its name: the migration code above addresses it
// before this migration runs.
func (db *Database) migration20260930000000(verbose bool) error {
	queries := []string{}

	if db.Config.DbType == DbTypeMariadb || db.Config.DbType == DbTypeMysql {
		rows, err := db.Sql.Query("select table_name from information_schema.tables where table_schema = database() and table_name like 'rdioScanner%'")
		if err != nil {
			return fmt.Errorf("migration20260930000000: %w", err)
		}
		for rows.Next() {
			var name string
			if err = rows.Scan(&name); err != nil {
				break
			}
			if name != "rdioScannerMeta" && name != strings.ToLower(name) {
				queries = append(queries, fmt.Sprintf("rename table `%s` to `%s`", name, strings.ToLower(name)))
			}
		}
		rows.Close()
		if err != nil {
			return fmt.Errorf("migration20260930000000: %w", err)
		}
	}

	if db.Config.DbType == DbTypeSqlite {
		rows, err := db.Sql.Query("select m.name, p.name from sqlite_master m join pragma_table_info(m.name) p where m.type = 'table' and m.name like 'rdioScanner%' and m.name <> 'rdioScannerMeta' and p.name <> lower(p.name)")
		if err != nil {
			return fmt.Errorf("migration20260930000000: %w", err)
		}
		for rows.Next() {
			var table, column string
			if err = rows.Scan(&table, &column); err != nil {
				break
			}
			queries = append(queries, fmt.Sprintf("alter table \"%s\" rename column \"%s\" to \"%s\"", table, column, strings.ToLower(column)))
		}
		rows.Close()
		if err != nil {
			return fmt.Errorf("migration20260930000000: %w", err)
		}
	}

	return db.migrateWithSchema("20260930000000-v7.3-lowercase-names", queries, verbose)
}

func (db *Database) seed() error {
	if err := db.seedGroups(); err != nil {
		return err
	}

	if err := db.seedTags(); err != nil {
		return err
	}

	return nil
}

func (db *Database) seedGroups() error {
	ctx := context.Background()

	count, err := db.Ent.Group.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("database.seedgroups: %w", err)
	}
	if count > 0 {
		return nil
	}

	creates := make([]*ent.GroupCreate, 0, len(defaults.groups))
	for _, label := range defaults.groups {
		creates = append(creates, db.Ent.Group.Create().SetLabel(label))
	}
	if err = db.Ent.Group.CreateBulk(creates...).Exec(ctx); err != nil {
		return fmt.Errorf("database.seedgroups: %w", err)
	}

	return nil
}

func (db *Database) seedTags() error {
	ctx := context.Background()

	count, err := db.Ent.Tag.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("database.seedtags: %w", err)
	}
	if count > 0 {
		return nil
	}

	creates := make([]*ent.TagCreate, 0, len(defaults.tags))
	for _, label := range defaults.tags {
		creates = append(creates, db.Ent.Tag.Create().SetLabel(label))
	}
	if err = db.Ent.Tag.CreateBulk(creates...).Exec(ctx); err != nil {
		return fmt.Errorf("database.seedtags: %w", err)
	}

	return nil
}
