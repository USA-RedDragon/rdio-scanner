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

// Test harness: every test runs against each supported database. SQLite
// always runs; PostgreSQL, MySQL and MariaDB run in containers (one per
// engine for the whole package, a fresh database per test). Set
// RDIO_TEST_DATABASES to a comma-separated subset (e.g. "sqlite") to skip
// the containers.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mariadb"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testEngine struct {
	dbType   string
	host     string
	port     uint
	user     string
	password string
	adminDSN func(host string, port uint) (driver, dsn string)
	err      error
}

var (
	testEnginesOnce sync.Once
	testEngines     = map[string]*testEngine{}
	testDbCounter   atomic.Uint64
)

func wantedTestDatabases() []string {
	if v := os.Getenv("RDIO_TEST_DATABASES"); v != "" {
		return strings.Split(v, ",")
	}
	return []string{DbTypeSqlite, DbTypePostgresql, DbTypeMysql, DbTypeMariadb}
}

func startTestEngines() {
	ctx := context.Background()
	for _, name := range wantedTestDatabases() {
		switch name {
		case DbTypePostgresql:
			e := &testEngine{dbType: name, user: "rdio", password: "rdio"}
			c, err := postgres.Run(ctx, "postgres:16-alpine",
				postgres.WithDatabase("rdio"), postgres.WithUsername(e.user), postgres.WithPassword(e.password),
				testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(2*time.Minute)))
			e.err = err
			if err == nil {
				e.host, e.port, e.err = containerAddr(ctx, c, "5432/tcp")
			}
			e.adminDSN = func(host string, port uint) (string, string) {
				return "postgres", fmt.Sprintf("host=%s port=%d user=rdio password=rdio dbname=rdio sslmode=disable", host, port)
			}
			testEngines[name] = e
		case DbTypeMysql, DbTypeMariadb:
			e := &testEngine{dbType: name, user: "root", password: "rdio"}
			var (
				c   testcontainers.Container
				err error
			)
			if name == DbTypeMysql {
				c, err = mysql.Run(ctx, "mysql:8.4", mysql.WithDatabase("rdio"), mysql.WithPassword(e.password), mysql.WithUsername("root"))
			} else {
				c, err = mariadb.Run(ctx, "mariadb:11", mariadb.WithDatabase("rdio"), mariadb.WithPassword(e.password), mariadb.WithUsername("root"))
			}
			e.err = err
			if err == nil {
				e.host, e.port, e.err = containerAddr(ctx, c, "3306/tcp")
			}
			e.adminDSN = func(host string, port uint) (string, string) {
				return "mysql", fmt.Sprintf("root:rdio@tcp(%s:%d)/", host, port)
			}
			testEngines[name] = e
		}
	}
}

func containerAddr(ctx context.Context, c testcontainers.Container, port string) (string, uint, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return "", 0, err
	}
	p, err := c.MappedPort(ctx, port)
	if err != nil {
		return "", 0, err
	}
	return host, uint(p.Num()), nil
}

// newTestDatabase opens a migrated and seeded Database of the given type on a
// database nobody else uses.
func newTestDatabase(t *testing.T, dbType string) *Database {
	t.Helper()

	config := &Config{DbType: dbType, DBSSLMode: SSLModeDisable}

	if dbType == DbTypeSqlite {
		config.BaseDir = t.TempDir()
		config.DbFile = "rdio-scanner.db"
	} else {
		testEnginesOnce.Do(startTestEngines)
		e := testEngines[dbType]
		if e == nil {
			t.Skipf("%s not requested in RDIO_TEST_DATABASES", dbType)
		}
		if e.err != nil {
			t.Skipf("%s container unavailable: %v", dbType, e.err)
		}

		name := fmt.Sprintf("rdio_test_%d_%d", os.Getpid(), testDbCounter.Add(1))
		driver, dsn := e.adminDSN(e.host, e.port)
		admin, err := sql.Open(driver, dsn)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = admin.Exec("create database " + name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if dbType == DbTypePostgresql {
				_, _ = admin.Exec("drop database if exists " + name + " with (force)")
			} else {
				_, _ = admin.Exec("drop database if exists " + name)
			}
			admin.Close()
		})

		config.DbHost = e.host
		config.DbPort = e.port
		config.DbName = name
		config.DbUsername = e.user
		config.DbPassword = e.password
	}

	db := NewDatabase(config)
	t.Cleanup(func() { db.Sql.Close() })
	return db
}

// forEachDatabase runs fn as a subtest against every requested database.
func forEachDatabase(t *testing.T, fn func(t *testing.T, db *Database)) {
	for _, dbType := range wantedTestDatabases() {
		dbType := dbType
		t.Run(dbType, func(t *testing.T) {
			fn(t, newTestDatabase(t, dbType))
		})
	}
}
