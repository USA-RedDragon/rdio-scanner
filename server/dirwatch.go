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
	"errors"
	"fmt"
	"io/fs"
	"math"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	entdirwatch "github.com/USA-RedDragon/rdio-scanner/server/ent/dirwatch"
	"github.com/fsnotify/fsnotify"
)

const (
	DirwatchTypeDefault       = "default"
	DirwatchTypeDSDPlus       = "dsdplus"
	DirwatchTypeSdrTrunk      = "sdr-trunk"
	DirwatchTypeTrunkRecorder = "trunk-recorder"
)

type Dirwatch struct {
	Id          any    `json:"_id"`
	Delay       any    `json:"delay"`
	DeleteAfter bool   `json:"deleteAfter"`
	Directory   string `json:"directory"`
	Disabled    bool   `json:"disabled"`
	Extension   any    `json:"extension"`
	Frequency   any    `json:"frequency"`
	Mask        any    `json:"mask"`
	Order       any    `json:"order"`
	SystemId    any    `json:"systemId"`
	TalkgroupId any    `json:"talkgroupId"`
	Kind        any    `json:"type"`
	UsePolling  bool   `json:"usePolling"`
	controller  *Controller
	dirs        map[string]bool
	mutex       sync.Mutex
	timers      map[string]*time.Timer
	watcher     *fsnotify.Watcher
}

func NewDirwatch() *Dirwatch {
	return &Dirwatch{
		dirs:   map[string]bool{},
		mutex:  sync.Mutex{},
		timers: map[string]*time.Timer{},
	}
}

func (dirwatch *Dirwatch) FromMap(m map[string]any) *Dirwatch {
	switch v := m["_id"].(type) {
	case float64:
		dirwatch.Id = uint(v)
	}

	switch v := m["delay"].(type) {
	case float64:
		dirwatch.Delay = uint(v)
	}

	switch v := m["deleteAfter"].(type) {
	case bool:
		dirwatch.DeleteAfter = v
	}

	switch v := m["directory"].(type) {
	case string:
		dirwatch.Directory = v
	}

	switch v := m["disabled"].(type) {
	case bool:
		dirwatch.Disabled = v
	}

	switch v := m["extension"].(type) {
	case string:
		dirwatch.Extension = v
	}

	switch v := m["frequency"].(type) {
	case float64:
		dirwatch.Frequency = uint(v)
	}

	switch v := m["mask"].(type) {
	case string:
		dirwatch.Mask = v
	}

	switch v := m["order"].(type) {
	case float64:
		dirwatch.Order = uint(v)
	}

	switch v := m["systemId"].(type) {
	case float64:
		dirwatch.SystemId = uint(v)
	}

	switch v := m["talkgroupId"].(type) {
	case float64:
		dirwatch.TalkgroupId = uint(v)
	}

	switch v := m["type"].(type) {
	case string:
		dirwatch.Kind = v
	}

	switch v := m["usePolling"].(type) {
	case bool:
		dirwatch.UsePolling = v
	}

	return dirwatch
}

func (dirwatch *Dirwatch) Ingest(p string) {
	var err error

	switch dirwatch.Kind {
	case DirwatchTypeDSDPlus:
		err = dirwatch.ingestDSDPlus(p)
	case DirwatchTypeTrunkRecorder:
		err = dirwatch.ingestTrunkRecorder(p)
	case DirwatchTypeSdrTrunk:
		err = dirwatch.ingestSdrTrunk(p)
	default:
		err = dirwatch.ingestDefault(p)
	}

	if err != nil {
		dirwatch.controller.Logs.LogEvent(LogLevelWarn, fmt.Sprintf("dirwatch.ingest: %s, %s", err.Error(), p))
	}
}

func (dirwatch *Dirwatch) ingestDefault(p string) error {
	var (
		err error
		ext string
	)

	switch v := dirwatch.Extension.(type) {
	case string:
		if len(v) > 0 {
			ext = fmt.Sprintf(".%s", v)
		} else {
			ext = ".wav"
		}
	default:
		ext = ".wav"
	}

	if strings.EqualFold(path.Ext(p), ext) {
		call := NewCall()

		call.AudioName = filepath.Base(p)
		call.AudioType = mime.TypeByExtension(path.Ext(p))
		call.Frequency = dirwatch.Frequency
		call.DateTime = time.Now().UTC()

		if call.Audio, err = os.ReadFile(p); err != nil {
			return err
		}

		dirwatch.parseMask(call)

		switch v := dirwatch.SystemId.(type) {
		case uint:
			call.System = v
		}

		switch v := dirwatch.TalkgroupId.(type) {
		case uint:
			call.Talkgroup = v
		}

		if ok, err := call.IsValid(); ok {
			dirwatch.controller.Ingest <- call

			if dirwatch.DeleteAfter {
				if err = os.Remove(p); err != nil {
					return err
				}
			}

		} else {
			return err
		}
	}

	return err
}

func (dirwatch *Dirwatch) ingestDSDPlus(p string) error {
	var (
		err error
		ext string
	)

	switch v := dirwatch.Extension.(type) {
	case string:
		if len(v) > 0 {
			ext = fmt.Sprintf(".%s", v)
		} else {
			ext = ".mp3"
		}
	default:
		ext = ".mp3"
	}

	if !strings.EqualFold(path.Ext(p), ext) {
		return nil
	}

	call := NewCall()

	call.AudioName = filepath.Base(p)
	call.AudioType = mime.TypeByExtension(path.Ext(p))
	call.Frequency = dirwatch.Frequency

	switch v := dirwatch.SystemId.(type) {
	case uint:
		call.System = v
	}

	switch v := dirwatch.TalkgroupId.(type) {
	case uint:
		call.Talkgroup = v
	}

	if call.Audio, err = os.ReadFile(p); err != nil {
		return err
	}

	if err = ParseDSDPlusMeta(call, p); err != nil {
		return err
	}

	if ok, err := call.IsValid(); ok {
		dirwatch.controller.Ingest <- call

		if dirwatch.DeleteAfter {
			if err = os.Remove(p); err != nil {
				return err
			}
		}

	} else {
		return err
	}

	return nil
}

func (dirwatch *Dirwatch) ingestSdrTrunk(p string) error {
	var err error

	if !strings.EqualFold(path.Ext(p), ".mp3") {
		return nil
	}

	call := NewCall()

	call.AudioName = filepath.Base(p)
	call.AudioType = mime.TypeByExtension(path.Ext(p))
	call.Frequency = dirwatch.Frequency

	if call.Audio, err = os.ReadFile(p); err != nil {
		return err
	}

	if err = ParseSdrTrunkMeta(call, dirwatch.controller); err != nil {
		return err
	}

	if ok, err := call.IsValid(); ok {
		dirwatch.controller.Ingest <- call

		if dirwatch.DeleteAfter {
			if err = os.Remove(p); err != nil {
				return err
			}
		}

	} else {
		return err
	}

	return nil
}

func (dirwatch *Dirwatch) ingestTrunkRecorder(p string) error {
	var (
		b   []byte
		err error
		ext string
	)

	if !strings.EqualFold(path.Ext(p), ".json") {
		return nil
	}

	switch v := dirwatch.Extension.(type) {
	case string:
		if len(v) > 0 {
			ext = fmt.Sprintf(".%s", v)
		} else {
			ext = ".wav"
		}
	default:
		ext = ".wav"
	}

	base := strings.TrimSuffix(p, ".json")

	audioName := base + ext

	call := NewCall()

	call.AudioName = filepath.Base(audioName)
	call.AudioType = mime.TypeByExtension(path.Ext(audioName))
	call.Frequency = dirwatch.Frequency

	switch v := dirwatch.SystemId.(type) {
	case uint:
		call.System = v
	}

	if call.Audio, err = os.ReadFile(audioName); err != nil {
		return nil
	}

	if b, err = os.ReadFile(p); err != nil {
		return err
	}

	if err = ParseTrunkRecorderMeta(call, b); err != nil {
		return err
	}

	if ok, err := call.IsValid(); ok {
		dirwatch.controller.Ingest <- call

	} else {
		return err
	}

	if dirwatch.DeleteAfter {
		if err = os.Remove(p); err != nil {
			return err
		}
		if err = os.Remove(audioName); err != nil {
			return err
		}
	}

	return nil
}

func (dirwatch *Dirwatch) parseMask(call *Call) {
	var meta = [][]string{
		{"date", "#DATE", `\d{4}[-_]{0,1}\d{2}[-_]{0,1}\d{2}`},
		{"group", "#GROUP", `[a-zA-Z0-9\.\ -]+`},
		{"hz", "#HZ", `\d+`},
		{"khz", "#KHZ", `[\d\.]+`},
		{"mhz", "#MHZ", `[\d\.]+`},
		{"syslbl", "#SYSLBL", `[a-zA-Z0-9,\.\ -]+`},
		{"sys", "#SYS", `\d+`},
		{"tag", "#TAG", `[a-zA-Z0-9\.\ -]+`},
		{"tgafs", "#TGAFS", `\d{2}-\d{3}`},
		{"tghz", "#TGHZ", `\d+`},
		{"tgkhz", "#TGKHZ", `[\d\.]+`},
		{"tglbl", "#TGLBL", `[a-zA-Z0-9,\.\ -]+`},
		{"tgmhz", "#TGMHZ", `[\d\.]+`},
		{"tg", "#TG", `\d+`},
		{"time", "#TIME", `\d{2}[-:]{0,1}\d{2}[-:]{0,1}\d{2}`},
		{"unit", "#UNIT", `\d+`},
		{"ztime", "#ZTIME", `\d{2}[-:]{0,1}\d{2}[-:]{0,1}\d{2}`},
	}

	var (
		filename string
		mask     string
		metapos  = [][]any{}
		metaval  = map[string]any{}
	)

	switch v := dirwatch.Mask.(type) {
	case string:
		mask = v
	default:
		return
	}

	switch v := call.AudioName.(type) {
	case string:
		filename = v
	default:
		return
	}

	for _, v := range meta {
		if i := strings.Index(mask, v[1]); i != -1 {
			metapos = append(metapos, []any{v[0], i})
			mask = strings.Replace(mask, v[1], fmt.Sprintf("(%v)", v[2]), 1)
		}
	}

	sort.Slice(metapos, func(i int, j int) bool {
		return metapos[i][1].(int) < metapos[j][1].(int)
	})

	base := strings.TrimSuffix(filename, path.Ext(filename))
	for i, s := range regexp.MustCompile(mask).FindStringSubmatch(base) {
		if i > 0 {
			v := fmt.Sprintf("%v", metapos[i-1][0])
			metaval[v] = s
		}
	}

	switch vDate := metaval["date"].(type) {
	case string:
		vDate = regexp.MustCompile(`(\d{4})(\d{2})(\d{2})`).ReplaceAllString(vDate, "$1-$2-$3")
		switch vTime := metaval["time"].(type) {
		case string:
			vTime = regexp.MustCompile(`(\d{2})[^\d]*(\d{2})[^\d]*(\d{2})`).ReplaceAllString(vTime, "$1:$2:$3")
			if dateTime, err := time.ParseInLocation("2006-01-02T15:04:05", fmt.Sprintf("%vT%v", vDate, vTime), time.Now().Location()); err == nil {
				call.DateTime = dateTime.UTC()
			}
		default:
			switch vZtime := metaval["ztime"].(type) {
			case string:
				vZtime = regexp.MustCompile(`(\d{2})[^\d]*(\d{2})[^\d]*(\d{2})`).ReplaceAllString(vZtime, "$1:$2:$3")
				if dateTime, err := time.Parse("2006-01-02T15:04:05", fmt.Sprintf("%vT%v", vDate, vZtime)); err == nil {
					call.DateTime = dateTime.UTC()
				}
			default:
				vDate = regexp.MustCompile(`[^\d]`).ReplaceAllString(vDate, "")
				if sec, err := strconv.Atoi(vDate); err == nil {
					call.DateTime = time.Unix(int64(sec), 0).UTC()
				}
			}
		}
	}

	switch v := metaval["group"].(type) {
	case string:
		if len(v) > 0 && v != "-" {
			call.talkgroupGroup = v
		}
	}

	switch v := metaval["hz"].(type) {
	case string:
		if hz, err := strconv.ParseFloat(v, 64); err == nil {
			call.Frequency = uint(hz)
		}
	default:
		switch v := metaval["khz"].(type) {
		case string:
			if khz, err := strconv.ParseFloat(v, 64); err == nil {
				call.Frequency = uint(khz * 1e3)
			}
		default:
			switch v := metaval["mhz"].(type) {
			case string:
				if mhz, err := strconv.ParseFloat(v, 64); err == nil {
					call.Frequency = uint(mhz * 1e6)
				}
			}
		}
	}

	switch v := metaval["sys"].(type) {
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			call.System = uint(i)
		}
	default:
		switch v := metaval["syslbl"].(type) {
		case string:
			if system, ok := dirwatch.controller.Systems.GetSystem(v); ok {
				call.System = system.Id
			} else {
				call.System = dirwatch.controller.Systems.GetNewSystemId()
				call.systemLabel = v
			}
		}
	}

	switch v := metaval["tag"].(type) {
	case string:
		if len(v) > 0 && v != "-" {
			call.talkgroupTag = v
		}
	}

	switch v := metaval["tg"].(type) {
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			call.Talkgroup = uint(i)
		}
	default:
		switch v := metaval["tgafs"].(type) {
		case string:
			if len(v) == 6 && v[2] == '-' {
				if a, err := strconv.Atoi(v[:2]); err == nil {
					if b, err := strconv.Atoi(v[3:5]); err == nil {
						if c, err := strconv.Atoi(v[5:]); err == nil {
							call.Talkgroup = uint(a<<7 | b<<3 | c)
						}
					}
				}
			}
		default:
			switch v := metaval["tghz"].(type) {
			case string:
				if hz, err := strconv.ParseFloat(v, 64); err == nil {
					call.Frequency = uint(hz)
					call.Talkgroup = uint(hz / 1e3)
				}
			default:
				switch v := metaval["tgkhz"].(type) {
				case string:
					if khz, err := strconv.ParseFloat(v, 64); err == nil {
						call.Frequency = uint(khz * 1e3)
						call.Talkgroup = uint(khz)
					}
				default:
					switch v := metaval["tgmhz"].(type) {
					case string:
						if mhz, err := strconv.ParseFloat(v, 64); err == nil {
							call.Frequency = uint(mhz * 1e6)
							call.Talkgroup = uint(mhz * 1e3)
						}
					}
				}
			}
		}
	}

	switch v := metaval["tglbl"].(type) {
	case string:
		if len(v) > 0 {
			call.talkgroupLabel = v
		}
	}

	switch v := metaval["unit"].(type) {
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			switch sources := call.Sources.(type) {
			case []map[string]any:
				call.Sources = append(sources, map[string]any{"pos": 0, "src": uint(i)})
			}
		}
	}
}

func (dirwatch *Dirwatch) Start(controller *Controller) error {
	var (
		delay time.Duration
		err   error
	)

	if dirwatch.Disabled {
		return nil
	}

	if dirwatch.watcher != nil {
		return errors.New("dirwatch.start: already started")
	}

	dirwatch.controller = controller
	dirwatch.dirs = map[string]bool{}

	if dirwatch.watcher, err = fsnotify.NewWatcher(); err != nil {
		return err
	}

	switch v := dirwatch.Delay.(type) {
	case uint:
		delay = time.Duration(math.Max(float64(v), 2000)) * time.Millisecond
	default:
		delay = time.Duration(2000) * time.Millisecond
	}

	go func() {
		logError := func(err error) {
			controller.Logs.LogEvent(LogLevelError, fmt.Sprintf("dirwatch.watcher: %v", err.Error()))
		}

		newTimer := func(eventName string) *time.Timer {
			return time.AfterFunc(delay, func() {
				dirwatch.mutex.Lock()
				defer dirwatch.mutex.Unlock()

				delete(dirwatch.timers, eventName)

				if _, err := os.Stat(eventName); err == nil {
					dirwatch.Ingest(eventName)
				}
			})
		}

		defer func() {
			switch v := recover().(type) {
			case error:
				controller.Logs.LogEvent(LogLevelError, v.Error())
			}

			dirwatch.mutex.Lock()
			defer dirwatch.mutex.Unlock()

			for e, t := range dirwatch.timers {
				t.Stop()
				delete(dirwatch.timers, e)
			}

			if dirwatch.watcher != nil {
				dirwatch.Start(controller)
			}
		}()

		for {
			if dirwatch.watcher == nil {
				return
			}

			select {
			case event, ok := <-dirwatch.watcher.Events:
				if !ok {
					return
				}

				switch event.Op {
				case fsnotify.Create:
					if dirwatch.isDir(event.Name) {
						if err := dirwatch.walkDir(event.Name); err != nil {
							logError(err)
						}

					} else {
						dirwatch.mutex.Lock()
						if dirwatch.timers[event.Name] != nil {
							dirwatch.timers[event.Name].Stop()
						}
						dirwatch.timers[event.Name] = newTimer(event.Name)
						dirwatch.mutex.Unlock()
					}

				case fsnotify.Remove:
					if dirwatch.dirs[event.Name] {
						if err := dirwatch.watcher.Remove(event.Name); err == nil {
							delete(dirwatch.dirs, event.Name)
						} else {
							logError(err)
						}
					}

				case fsnotify.Write:
					dirwatch.mutex.Lock()
					if dirwatch.timers[event.Name] != nil {
						dirwatch.timers[event.Name].Stop()
					}
					dirwatch.timers[event.Name] = newTimer(event.Name)
					dirwatch.mutex.Unlock()
				}

			case err, ok := <-dirwatch.watcher.Errors:
				if ok {
					logError(err)
				}

				return
			}
		}
	}()

	go func() {
		defer func() {
			switch v := recover().(type) {
			case error:
				controller.Logs.LogEvent(LogLevelError, v.Error())
			}
		}()

		time.Sleep(delay)

		if err := fs.WalkDir(os.DirFS(dirwatch.Directory), ".", func(p string, _ fs.DirEntry, err error) error {
			fp := filepath.Join(dirwatch.Directory, p)

			if dirwatch.isDir(fp) {
				dirwatch.dirs[fp] = true
				dirwatch.watcher.Add(fp)

			} else if dirwatch.DeleteAfter {
				dirwatch.Ingest(fp)
			}

			return err
		}); err != nil {
			controller.Logs.LogEvent(LogLevelError, fmt.Sprintf("dirwatch.walkdir: %s", err.Error()))
		}
	}()

	return nil
}

func (dirwatch *Dirwatch) Stop() {
	if dirwatch.watcher != nil {
		w := dirwatch.watcher
		dirwatch.watcher = nil
		w.Close()
	}
}

type Dirwatches struct {
	List  []*Dirwatch
	mutex sync.Mutex
}

func NewDirwatches() *Dirwatches {
	return &Dirwatches{
		List:  []*Dirwatch{},
		mutex: sync.Mutex{},
	}
}

func (dirwatches *Dirwatches) FromMap(f []any) *Dirwatches {
	dirwatches.mutex.Lock()
	defer dirwatches.mutex.Unlock()

	dirwatches.Stop()

	dirwatches.List = []*Dirwatch{}

	for _, f := range f {
		switch v := f.(type) {
		case map[string]any:
			dirwatch := NewDirwatch().FromMap(v)
			dirwatches.List = append(dirwatches.List, dirwatch)
		}
	}

	return dirwatches
}

func (dirwatches *Dirwatches) Read(db *Database) error {
	dirwatches.mutex.Lock()
	defer dirwatches.mutex.Unlock()

	dirwatches.Stop()

	dirwatches.List = []*Dirwatch{}

	records, err := db.Ent.Dirwatch.Query().All(context.Background())
	if err != nil {
		return fmt.Errorf("dirwatches.read: %w", err)
	}

	for _, r := range records {
		dirwatch := NewDirwatch()

		dirwatch.Id = uint(r.ID)
		dirwatch.Directory = r.Directory
		dirwatch.DeleteAfter = r.DeleteAfter != nil && *r.DeleteAfter
		dirwatch.Disabled = r.Disabled != nil && *r.Disabled
		dirwatch.UsePolling = r.UsePolling != nil && *r.UsePolling

		if r.Delay != nil && *r.Delay > 0 {
			dirwatch.Delay = uint(*r.Delay)
		}
		if r.Extension != nil && len(*r.Extension) > 0 {
			dirwatch.Extension = *r.Extension
		}
		if r.Frequency != nil && *r.Frequency > 0 {
			dirwatch.Frequency = uint(*r.Frequency)
		}
		if r.Mask != nil && len(*r.Mask) > 0 {
			dirwatch.Mask = *r.Mask
		}
		if r.Order != nil && *r.Order > 0 {
			dirwatch.Order = uint(*r.Order)
		}
		if r.SystemID != nil && *r.SystemID > 0 {
			dirwatch.SystemId = uint(*r.SystemID)
		}
		if r.TalkgroupID != nil && *r.TalkgroupID > 0 {
			dirwatch.TalkgroupId = uint(*r.TalkgroupID)
		}
		if r.Type != nil && len(*r.Type) > 0 {
			dirwatch.Kind = *r.Type
		}

		dirwatches.List = append(dirwatches.List, dirwatch)
	}

	return nil
}
func (dirwatches *Dirwatches) Start(controller *Controller) {
	for i := range dirwatches.List {
		if err := dirwatches.List[i].Start(controller); err != nil {
			controller.Logs.LogEvent(LogLevelError, fmt.Sprintf("dirwatches.start: %s", err.Error()))
		}
	}
}

func (dirwatches *Dirwatches) Stop() {
	for i := range dirwatches.List {
		dirwatches.List[i].Stop()
	}
	dirwatches.List = []*Dirwatch{}
}

func (dirwatches *Dirwatches) Write(db *Database) error {
	dirwatches.mutex.Lock()
	defer dirwatches.mutex.Unlock()

	ctx := context.Background()

	formatError := func(err error) error {
		return fmt.Errorf("dirwatches.write: %w", err)
	}

	keep := make([]int, 0, len(dirwatches.List))
	for _, dirwatch := range dirwatches.List {
		if id, ok := rowID(dirwatch.Id); ok {
			keep = append(keep, id)
		}
	}

	if _, err := db.Ent.Dirwatch.Delete().Where(entdirwatch.IDNotIn(keep...)).Exec(ctx); err != nil {
		return formatError(err)
	}

	for _, dirwatch := range dirwatches.List {
		if id, ok := rowID(dirwatch.Id); ok {
			upd := db.Ent.Dirwatch.UpdateOneID(id).
				SetDirectory(dirwatch.Directory).
				SetDeleteAfter(dirwatch.DeleteAfter).
				SetDisabled(dirwatch.Disabled).
				SetUsePolling(dirwatch.UsePolling)
			setOrClearInt(upd.SetDelay, upd.ClearDelay, nillablePosInt(dirwatch.Delay))
			setOrClearInt(upd.SetFrequency, upd.ClearFrequency, nillablePosInt(dirwatch.Frequency))
			setOrClearInt(upd.SetOrder, upd.ClearOrder, nillablePosInt(dirwatch.Order))
			setOrClearInt(upd.SetSystemID, upd.ClearSystemID, nillablePosInt(dirwatch.SystemId))
			setOrClearInt(upd.SetTalkgroupID, upd.ClearTalkgroupID, nillablePosInt(dirwatch.TalkgroupId))
			setOrClearStr(upd.SetExtension, upd.ClearExtension, nillableStr(dirwatch.Extension))
			setOrClearStr(upd.SetMask, upd.ClearMask, nillableStr(dirwatch.Mask))
			setOrClearStr(upd.SetType, upd.ClearType, nillableStr(dirwatch.Kind))
			if err := upd.Exec(ctx); err != nil {
				return formatError(err)
			}
		} else {
			if err := db.Ent.Dirwatch.Create().
				SetDirectory(dirwatch.Directory).
				SetDeleteAfter(dirwatch.DeleteAfter).
				SetDisabled(dirwatch.Disabled).
				SetUsePolling(dirwatch.UsePolling).
				SetNillableDelay(nillablePosInt(dirwatch.Delay)).
				SetNillableFrequency(nillablePosInt(dirwatch.Frequency)).
				SetNillableOrder(nillablePosInt(dirwatch.Order)).
				SetNillableSystemID(nillablePosInt(dirwatch.SystemId)).
				SetNillableTalkgroupID(nillablePosInt(dirwatch.TalkgroupId)).
				SetNillableExtension(nillableStr(dirwatch.Extension)).
				SetNillableMask(nillableStr(dirwatch.Mask)).
				SetNillableType(nillableStr(dirwatch.Kind)).
				Exec(ctx); err != nil {
				return formatError(err)
			}
		}
	}

	return nil
}
func (dirwatch *Dirwatch) isDir(d string) bool {
	if fi, err := os.Stat(d); err == nil {
		if fi.IsDir() {
			return true
		}
	}

	return false
}

func (dirwatch *Dirwatch) walkDir(d string) error {
	dfs := os.DirFS(d)

	return fs.WalkDir(dfs, ".", func(p string, _ fs.DirEntry, err error) error {
		fp := filepath.Join(d, p)
		if dirwatch.isDir(fp) {
			if !dirwatch.dirs[fp] {
				dirwatch.dirs[fp] = true
				dirwatch.watcher.Add(fp)
			}
		}
		return err
	})
}
