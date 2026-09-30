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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/USA-RedDragon/rdio-scanner/server/ent"
	"github.com/USA-RedDragon/rdio-scanner/server/ent/setting"
	"golang.org/x/crypto/bcrypt"
)

type Options struct {
	AfsSystems                  string `json:"afsSystems"`
	AudioConversion             uint   `json:"audioConversion"`
	AudioBitrate                uint   `json:"audioBitrate"`
	AutoPopulate                bool   `json:"autoPopulate"`
	Branding                    string `json:"branding"`
	DimmerDelay                 uint   `json:"dimmerDelay"`
	DisableDuplicateDetection   bool   `json:"disableDuplicateDetection"`
	DuplicateDetectionTimeFrame uint   `json:"duplicateDetectionTimeFrame"`
	KeypadBeeps                 string `json:"keypadBeeps"`
	MaxClients                  uint   `json:"maxClients"`
	PlaybackGoesLive            bool   `json:"playbackGoesLive"`
	PruneCallDays               uint   `json:"pruneCallDays"`
	PruneLogDays                uint   `json:"pruneLogDays"`
	SearchPatchedTalkgroups     bool   `json:"searchPatchedTalkgroups"`
	ShowListenersCount          bool   `json:"showListenersCount"`
	SortTalkgroups              bool   `json:"sortTalkgroups"`
	TagsToggle                  bool   `json:"tagsToggle"`
	Time12hFormat               bool   `json:"time12hFormat"`
	adminPassword               string
	adminPasswordNeedChange     bool
	mutex                       sync.Mutex
	secret                      string
}

const (
	AUDIO_CONVERSION_DISABLED          = 0
	AUDIO_CONVERSION_ENABLED           = 1
	AUDIO_CONVERSION_ENABLED_NORM      = 2
	AUDIO_CONVERSION_ENABLED_LOUD_NORM = 3
)

func NewOptions() *Options {
	return &Options{
		mutex: sync.Mutex{},
	}
}

func (options *Options) FromMap(m map[string]any) *Options {
	options.mutex.Lock()
	defer options.mutex.Unlock()

	switch v := m["afsSystems"].(type) {
	case string:
		options.AfsSystems = v
	}

	switch v := m["audioConversion"].(type) {
	case float64:
		options.AudioConversion = uint(v)
	default:
		options.MaxClients = defaults.options.audioConversion
	}

	switch v := m["audioBitrate"].(type) {
	case uint:
		options.AudioBitrate = v
	case float64:
		options.AudioBitrate = uint(v)
	default:
		options.AudioBitrate = defaults.options.audioBitrate
	}
	if options.AudioBitrate > 128 {
		options.AudioBitrate = 128
	}
	if options.AudioBitrate < 6 {
		options.AudioBitrate = 6
	}

	switch v := m["autoPopulate"].(type) {
	case bool:
		options.AutoPopulate = v
	default:
		options.AutoPopulate = defaults.options.autoPopulate
	}

	switch v := m["branding"].(type) {
	case string:
		options.Branding = v
	}

	switch v := m["dimmerDelay"].(type) {
	case float64:
		options.DimmerDelay = uint(v)
	default:
		options.DimmerDelay = defaults.options.dimmerDelay
	}

	switch v := m["disableAudioConversion"].(type) {
	case bool:
		if v {
			options.AudioConversion = 2
		} else {
			options.AudioConversion = 0
		}
	}

	switch v := m["disableDuplicateDetection"].(type) {
	case bool:
		options.DisableDuplicateDetection = v
	default:
		options.DisableDuplicateDetection = defaults.options.disableDuplicateDetection
	}

	switch v := m["duplicateDetectionTimeFrame"].(type) {
	case float64:
		options.DuplicateDetectionTimeFrame = uint(v)
	default:
		options.DuplicateDetectionTimeFrame = defaults.options.duplicateDetectionTimeFrame
	}

	switch v := m["keypadBeeps"].(type) {
	case string:
		options.KeypadBeeps = v
	default:
		options.KeypadBeeps = defaults.options.keypadBeeps
	}

	switch v := m["maxClients"].(type) {
	case float64:
		options.MaxClients = uint(v)
	default:
		options.MaxClients = defaults.options.maxClients
	}

	switch v := m["playbackGoesLive"].(type) {
	case bool:
		options.PlaybackGoesLive = v
	}

	switch v := m["pruneCallDays"].(type) {
	case float64:
		options.PruneCallDays = uint(v)
	default:
		options.PruneCallDays = defaults.options.pruneCallDays
	}

	switch v := m["pruneLogDays"].(type) {
	case float64:
		options.PruneLogDays = uint(v)
	default:
		options.PruneLogDays = defaults.options.pruneLogDays
	}

	switch v := m["searchPatchedTalkgroups"].(type) {
	case bool:
		options.SearchPatchedTalkgroups = v
	default:
		options.SearchPatchedTalkgroups = defaults.options.searchPatchedTalkgroups
	}

	switch v := m["showListenersCount"].(type) {
	case bool:
		options.ShowListenersCount = v
	default:
		options.ShowListenersCount = defaults.options.showListenersCount
	}

	switch v := m["sortTalkgroups"].(type) {
	case bool:
		options.SortTalkgroups = v
	default:
		options.SortTalkgroups = defaults.options.sortTalkgroups
	}

	switch v := m["tagsToggle"].(type) {
	case bool:
		options.TagsToggle = v
	default:
		options.TagsToggle = defaults.options.tagsToggle
	}

	switch v := m["time12hFormat"].(type) {
	case bool:
		options.Time12hFormat = v
	default:
		options.Time12hFormat = defaults.options.time12hFormat
	}

	return options
}

func (options *Options) Read(db *Database) error {
	var (
		defaultPassword []byte
		err             error
		s               string
	)

	options.mutex.Lock()
	defer options.mutex.Unlock()

	defaultPassword, _ = bcrypt.GenerateFromPassword([]byte(defaults.adminPassword), bcrypt.DefaultCost)

	options.adminPassword = string(defaultPassword)
	options.adminPasswordNeedChange = defaults.adminPasswordNeedChange
	options.AudioConversion = defaults.options.audioConversion
	options.AudioBitrate = defaults.options.audioBitrate
	options.AutoPopulate = defaults.options.autoPopulate
	options.DimmerDelay = defaults.options.dimmerDelay
	options.DisableDuplicateDetection = defaults.options.disableDuplicateDetection
	options.DuplicateDetectionTimeFrame = defaults.options.duplicateDetectionTimeFrame
	options.KeypadBeeps = defaults.options.keypadBeeps
	options.MaxClients = defaults.options.maxClients
	options.PlaybackGoesLive = defaults.options.playbackGoesLive
	options.PruneCallDays = defaults.options.pruneCallDays
	options.PruneLogDays = defaults.options.pruneLogDays
	options.SearchPatchedTalkgroups = defaults.options.searchPatchedTalkgroups
	options.ShowListenersCount = defaults.options.showListenersCount
	options.SortTalkgroups = defaults.options.sortTalkgroups
	options.TagsToggle = defaults.options.tagsToggle

	ctx := context.Background()

	if s, err = db.readSetting(ctx, "adminPassword"); err != nil {
		return fmt.Errorf("options.read: %w", err)
	} else if s != "" {
		var v string
		if json.Unmarshal([]byte(s), &v) == nil {
			options.adminPassword = v
		}
	}

	if s, err = db.readSetting(ctx, "adminPasswordNeedChange"); err != nil {
		return fmt.Errorf("options.read: %w", err)
	} else if s != "" {
		var b bool
		if json.Unmarshal([]byte(s), &b) == nil {
			options.adminPasswordNeedChange = b
		}
	}

	if s, err = db.readSetting(ctx, "options"); err != nil {
		return fmt.Errorf("options.read: %w", err)
	} else if s != "" {
		var m map[string]any

		if json.Unmarshal([]byte(s), &m) == nil {
			switch v := m["afsSystems"].(type) {
			case string:
				options.AfsSystems = v
			}

			switch v := m["audioConversion"].(type) {
			case float64:
				options.AudioConversion = uint(v)
			}

			switch v := m["audioBitrate"].(type) {
			case uint:
				options.AudioBitrate = v
			case float64:
				options.AudioBitrate = uint(v)
			}

			switch v := m["autoPopulate"].(type) {
			case bool:
				options.AutoPopulate = v
			}

			switch v := m["branding"].(type) {
			case string:
				options.Branding = v
			}

			switch v := m["dimmerDelay"].(type) {
			case float64:
				options.DimmerDelay = uint(v)
			}

			switch v := m["disableDuplicateDetection"].(type) {
			case bool:
				options.DisableDuplicateDetection = v
			}

			switch v := m["duplicateDetectionTimeFrame"].(type) {
			case float64:
				options.DuplicateDetectionTimeFrame = uint(v)
			}

			switch v := m["keypadBeeps"].(type) {
			case string:
				options.KeypadBeeps = v
			}

			switch v := m["maxClients"].(type) {
			case float64:
				options.MaxClients = uint(v)
			}

			switch v := m["playbackGoesLive"].(type) {
			case bool:
				options.PlaybackGoesLive = v
			}

			switch v := m["pruneCallDays"].(type) {
			case float64:
				options.PruneCallDays = uint(v)
			}

			switch v := m["pruneLogDays"].(type) {
			case float64:
				options.PruneLogDays = uint(v)
			}

			switch v := m["searchPatchedTalkgroups"].(type) {
			case bool:
				options.SearchPatchedTalkgroups = v
			}

			switch v := m["showListenersCount"].(type) {
			case bool:
				options.ShowListenersCount = v
			}

			switch v := m["sortTalkgroups"].(type) {
			case bool:
				options.SortTalkgroups = v
			}

			switch v := m["tagsToggle"].(type) {
			case bool:
				options.TagsToggle = v
			}

			switch v := m["time12hFormat"].(type) {
			case bool:
				options.Time12hFormat = v
			}
		}
	}

	if s, err = db.readSetting(ctx, "secret"); err != nil {
		return fmt.Errorf("options.read: %w", err)
	} else if s != "" {
		var v string
		if json.Unmarshal([]byte(s), &v) == nil {
			options.secret = v
		}
	}

	// The JWT signing secret was never persisted by older versions, leaving
	// it empty so admin tokens could be forged. Generate and store one once.
	if options.secret == "" {
		buf := make([]byte, 32)
		if _, err = rand.Read(buf); err != nil {
			return fmt.Errorf("options.read: %w", err)
		}
		options.secret = hex.EncodeToString(buf)
		if b, err := json.Marshal(options.secret); err == nil {
			if err = db.writeSetting(ctx, "secret", string(b)); err != nil {
				return fmt.Errorf("options.read: %w", err)
			}
		}
	}

	return nil
}

// readSetting returns the stored JSON value for a config key, or "" if absent.
func (db *Database) readSetting(ctx context.Context, key string) (string, error) {
	s, err := db.Ent.Setting.Query().Where(setting.Key(key)).Only(ctx)
	if ent.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.Val, nil
}

// writeSetting upserts a config key to val.
func (db *Database) writeSetting(ctx context.Context, key, val string) error {
	return db.Ent.Setting.Create().SetKey(key).SetVal(val).
		OnConflictColumns("key").UpdateVal().Exec(ctx)
}

func (options *Options) Write(db *Database) error {
	options.mutex.Lock()
	defer options.mutex.Unlock()

	ctx := context.Background()

	formatError := func(err error) error {
		return fmt.Errorf("options.write: %v", err)
	}

	write := func(key string, value any) error {
		b, err := json.Marshal(value)
		if err != nil {
			return formatError(err)
		}
		if err = db.writeSetting(ctx, key, string(b)); err != nil {
			return formatError(err)
		}
		return nil
	}

	if err := write("adminPassword", options.adminPassword); err != nil {
		return err
	}
	if err := write("adminPasswordNeedChange", options.adminPasswordNeedChange); err != nil {
		return err
	}
	return write("options", map[string]any{
		"afsSystems":                  options.AfsSystems,
		"audioConversion":             options.AudioConversion,
		"audioBitrate":                options.AudioBitrate,
		"autoPopulate":                options.AutoPopulate,
		"branding":                    options.Branding,
		"dimmerDelay":                 options.DimmerDelay,
		"disableDuplicateDetection":   options.DisableDuplicateDetection,
		"duplicateDetectionTimeFrame": options.DuplicateDetectionTimeFrame,
		"keypadBeeps":                 options.KeypadBeeps,
		"maxClients":                  options.MaxClients,
		"playbackGoesLive":            options.PlaybackGoesLive,
		"pruneLogDays":                options.PruneLogDays,
		"pruneCallDays":               options.PruneCallDays,
		"searchPatchedTalkgroups":     options.SearchPatchedTalkgroups,
		"showListenersCount":          options.ShowListenersCount,
		"sortTalkgroups":              options.SortTalkgroups,
		"tagsToggle":                  options.TagsToggle,
		"time12hFormat":               options.Time12hFormat,
	})
}
