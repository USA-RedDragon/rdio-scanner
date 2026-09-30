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
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"

	"gopkg.in/ini.v1"
)

// https://goreleaser.com/cookbooks/using-main.version/
//
//nolint:golint,gochecknoglobals
var (
	version = "dev"
	commit  = "none"
)

const (
	DbTypeMariadb    string = "mariadb"
	DbTypeMysql      string = "mysql"
	DbTypeSqlite     string = "sqlite"
	DbTypePostgresql string = "postgresql"
)

type sslMode string

const (
	SSLModeDisable    sslMode = "disable"
	SSLModeRequire    sslMode = "require"
	SSLModeVerifyCA   sslMode = "verify-ca"
	SSLModeVerifyFull sslMode = "verify-full"
)

type Config struct {
	BaseDir          string
	ConfigFile       string
	DbType           string
	DbFile           string
	DbHost           string
	DbPort           uint
	DbName           string
	DBSSLMode        sslMode
	DbUsername       string
	DbPassword       string
	MetricsPort      uint
	Listen           string
	daemon           *Daemon
	newAdminPassword string
}

func NewConfig() *Config {
	const defaultConfigFile = "rdio-scanner.ini"

	var (
		command       = flag.String(COMMAND_ARG, "", fmt.Sprintf("advanced administrative tasks (use -%s %s for usage)", COMMAND_ARG, COMMAND_HELP))
		config        = &Config{}
		configSave    = flag.Bool("config_save", false, fmt.Sprintf("save configuration to %s", defaultConfigFile))
		serviceAction = flag.String("service", "", "service command, one of start, stop, restart, install, uninstall")
		showVersion   = flag.Bool("version", false, "show application version")
	)

	defaultDbType := os.Getenv("DB_TYPE")
	if defaultDbType == "" {
		defaultDbType = DbTypeSqlite
	}

	defaultDbHost := os.Getenv("DB_HOST")
	if defaultDbHost == "" {
		defaultDbHost = "localhost"
	}

	dbPortStr := os.Getenv("DB_PORT")
	if dbPortStr == "" {
		dbPortStr = "3306"
	}
	defaultDbPort := uint(0)
	tmp, err := strconv.Atoi(dbPortStr)
	if err != nil {
		defaultDbPort = uint(tmp)
	} else {
		defaultDbPort = uint(tmp)
	}

	const (
		defaultAdminUrl = "/admin"
		defaultDbFile   = "rdio-scanner.db"
		defaultListen   = ":3000"
	)

	if exe, err := os.Executable(); err == nil {
		if !regexp.MustCompile(`go-build[0-9]+`).Match([]byte(exe)) {
			config.BaseDir = filepath.Dir(exe)
			if !config.isBaseDirWritable() {
				if h, err := os.UserHomeDir(); err == nil {
					config.BaseDir = filepath.Join(h, "Rdio Scanner")
					if _, err := os.Stat(config.BaseDir); os.IsNotExist(err) {
						os.MkdirAll(config.BaseDir, 0770)
					}
				}
			}
		}
	}

	flag.StringVar(&config.BaseDir, "base_dir", config.BaseDir, "base directory where all data will be written")
	flag.StringVar(&config.DbFile, "db_file", defaultDbFile, "sqlite database file")
	flag.StringVar(&config.DbHost, "db_host", defaultDbHost, "database host ip or hostname")
	flag.StringVar(&config.DbName, "db_name", "", "database name")
	flag.StringVar((*string)(&config.DBSSLMode), "db_sslmode", string(SSLModeDisable), fmt.Sprintf("database ssl mode, one of %s, %s, %s, or %s", SSLModeDisable, SSLModeRequire, SSLModeVerifyCA, SSLModeVerifyFull))
	flag.StringVar(&config.DbPassword, "db_pass", "", "database password")
	flag.UintVar(&config.DbPort, "db_port", defaultDbPort, "database host port")
	flag.StringVar(&config.DbType, "db_type", defaultDbType, fmt.Sprintf("database type, one of %s, %s, %s, or %s", DbTypeSqlite, DbTypeMariadb, DbTypeMysql, DbTypePostgresql))
	flag.StringVar(&config.DbUsername, "db_user", "", "database user name")
	flag.UintVar(&config.MetricsPort, "metrics_port", 0, "port for prometheus metrics")
	flag.StringVar(&config.ConfigFile, "config", defaultConfigFile, "server config file")
	flag.StringVar(&config.Listen, "listen", defaultListen, "listening address")
	flag.StringVar(&config.newAdminPassword, "admin_password", "", "change admin password")
	flag.Parse()

	dbUsernameEnv := os.Getenv("DB_USER")
	if dbUsernameEnv != "" {
		config.DbUsername = dbUsernameEnv
	}

	dbPasswordEnv := os.Getenv("DB_PASS")
	if dbPasswordEnv != "" {
		config.DbPassword = dbPasswordEnv
	}

	dbNameEnv := os.Getenv("DB_NAME")
	if dbNameEnv != "" {
		config.DbName = dbNameEnv
	}

	dbSSLModeEnv := os.Getenv("DB_SSLMODE")
	if dbSSLModeEnv != "" {
		config.DBSSLMode = sslMode(dbSSLModeEnv)
	}

	metricsPortEnvStr := os.Getenv("METRICS_PORT")
	if metricsPortEnvStr != "" {
		metricsPort, err := strconv.Atoi(metricsPortEnvStr)
		if err == nil {
			config.MetricsPort = uint(metricsPort)
		} else {
			log.Fatalf("error: %s\n", err.Error())
		}
	}

	if !config.isBaseDirWritable() {
		log.Fatalf("no write permissions in %s", config.BaseDir)
	}

	switch {
	case *configSave:
		if err := config.saveConfig(); err == nil {
			fmt.Printf("%s file created\n", config.ConfigFile)
			os.Exit(0)
		} else {
			fmt.Printf("error: %s\n", err.Error())
			os.Exit(-1)
		}

	case *showVersion:
		fmt.Printf("Version %s Commit %s", version, commit)
		os.Exit(0)

	default:
		if cfg, err := ini.Load(config.GetConfigFilePath()); err == nil {
			if v := cfg.Section("").Key("db_file").String(); len(v) > 0 {
				config.DbFile = v
			}

			if v := cfg.Section("").Key("db_host").String(); len(v) > 0 {
				config.DbHost = v
			}

			if v := cfg.Section("").Key("db_name").String(); len(v) > 0 {
				config.DbName = v
			}

			if v := cfg.Section("").Key("db_sslmode").String(); len(v) > 0 {
				config.DBSSLMode = sslMode(v)
			}

			if v := cfg.Section("").Key("db_pass").String(); len(v) > 0 {
				config.DbPassword = v
			}

			if config.DbPort, err = cfg.Section("").Key("db_port").Uint(); err != nil {
				config.DbPort = defaultDbPort
			}

			if v := cfg.Section("").Key("db_type").String(); len(v) > 0 {
				config.DbType = v
			}

			if v := cfg.Section("").Key("db_user").String(); len(v) > 0 {
				config.DbUsername = v
			}

			if v := cfg.Section("").Key("listen").String(); len(v) > 0 {
				config.Listen = v
			}
		}

		if !(config.DbType == DbTypeMariadb || config.DbType == DbTypeMysql || config.DbType == DbTypePostgresql || config.DbType == DbTypeSqlite) {
			fmt.Printf("unknown database type %s\n", config.DbType)
			return nil
		}
	}

	if *command != "" {
		NewCommand(config.BaseDir).Do(*command)
	}

	if *serviceAction != "" {
		config.daemon = NewDaemon().Control(*serviceAction)
	}

	return config
}

func (config *Config) GetConfigFilePath() string {
	return config.GetPath(config.ConfigFile)
}

func (config *Config) GetDbFilePath() string {
	return config.GetPath(config.DbFile)
}

func (config *Config) GetPath(p string) string {
	if path.IsAbs(p) {
		return p
	}
	return filepath.Join(config.BaseDir, p)
}

func (config *Config) isBaseDirWritable() bool {
	if f, err := os.CreateTemp(config.BaseDir, ".tmp*"); err == nil {
		f.Close()
		os.Remove(f.Name())
		return true
	}
	return false
}

func (config *Config) saveConfig() error {
	ini := []string{}

	if config.DbType == DbTypeSqlite {
		if config.DbFile != "" {
			ini = append(ini, fmt.Sprintf("db_file = %s", config.DbFile))
		}

	} else {
		if config.DbHost != "" {
			ini = append(ini, fmt.Sprintf("db_host = %s", config.DbHost))
		}

		if config.DbName != "" {
			ini = append(ini, fmt.Sprintf("db_name = %s", config.DbName))
		}

		if config.DBSSLMode != "" {
			ini = append(ini, fmt.Sprintf("db_sslmode = %s", config.DBSSLMode))
		}

		if config.DbPassword != "" {
			ini = append(ini, fmt.Sprintf("db_pass = %s", config.DbPassword))
		}

		if config.DbPort > 0 {
			ini = append(ini, fmt.Sprintf("db_port = %s", strconv.Itoa(int(config.DbPort))))
		}
	}

	if config.DbType != "" {
		ini = append(ini, fmt.Sprintf("db_type = %s", config.DbType))
	}

	if config.DbUsername != "" && config.DbType != DbTypeSqlite {
		ini = append(ini, fmt.Sprintf("db_user = %s", config.DbUsername))
	}

	if config.Listen != "" {
		ini = append(ini, fmt.Sprintf("listen = %s", config.Listen))
	}

	file, err := os.Create(config.GetConfigFilePath())
	if err != nil {
		return err
	}

	for _, line := range ini {
		_, err := file.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	return file.Close()
}
