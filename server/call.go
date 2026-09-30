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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/USA-RedDragon/rdio-scanner/server/ent"
	entcall "github.com/USA-RedDragon/rdio-scanner/server/ent/call"
	"github.com/USA-RedDragon/rdio-scanner/server/ent/predicate"
)

type Call struct {
	Id             any       `json:"id"`
	Audio          []byte    `json:"audio"`
	AudioName      any       `json:"audioName"`
	AudioType      any       `json:"audioType"`
	DateTime       time.Time `json:"dateTime"`
	Frequencies    any       `json:"frequencies"`
	Frequency      any       `json:"frequency"`
	Patches        any       `json:"patches"`
	Source         any       `json:"source"`
	Sources        any       `json:"sources"`
	System         uint      `json:"system"`
	Talkgroup      uint      `json:"talkgroup"`
	systemLabel    any
	talkgroupGroup any
	talkgroupLabel any
	talkgroupName  any
	talkgroupTag   any
	units          any
}

func NewCall() *Call {
	return &Call{
		Frequencies: []map[string]any{},
		Patches:     []uint{},
		Sources:     []map[string]any{},
	}
}

func (call *Call) IsValid() (ok bool, err error) {
	ok = true

	if len(call.Audio) <= 44 {
		ok = false
		err = errors.New("no audio")
	}

	if call.DateTime.Unix() == 0 {
		ok = false
		err = errors.New("no datetime")
	}

	if call.System < 1 {
		ok = false
		err = errors.New("no system")
	}

	if call.Talkgroup < 1 {
		ok = false
		err = errors.New("no talkgroup")
	}

	return ok, err
}

func (call *Call) MarshalJSON() ([]byte, error) {
	audio := fmt.Sprintf("%v", call.Audio)
	audio = strings.ReplaceAll(audio, " ", ",")

	return json.Marshal(map[string]any{
		"id": call.Id,
		"audio": map[string]any{
			"data": json.RawMessage(audio),
			"type": "Buffer",
		},
		"audioName":   call.AudioName,
		"audioType":   call.AudioType,
		"dateTime":    call.DateTime.Format(time.RFC3339),
		"frequencies": call.Frequencies,
		"frequency":   call.Frequency,
		"patches":     call.Patches,
		"source":      call.Source,
		"sources":     call.Sources,
		"system":      call.System,
		"talkgroup":   call.Talkgroup,
	})
}

func (call *Call) ToJson() (string, error) {
	if b, err := json.Marshal(call); err == nil {
		return string(b), nil
	} else {
		return "", fmt.Errorf("call.tojson: %v", err)
	}
}

type Calls struct {
	mutex sync.Mutex
}

func NewCalls() *Calls {
	return &Calls{
		mutex: sync.Mutex{},
	}
}

func (calls *Calls) CheckDuplicate(call *Call, msTimeFrame uint, db *Database) bool {
	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	d := time.Duration(msTimeFrame) * time.Millisecond
	from := call.DateTime.Add(-d)
	to := call.DateTime.Add(d)

	count, err := db.Ent.Call.Query().Where(
		entcall.DateTimeGTE(from),
		entcall.DateTimeLTE(to),
		entcall.System(int(call.System)),
		entcall.Talkgroup(int(call.Talkgroup)),
	).Count(context.Background())
	if err != nil {
		return false
	}

	return count > 0
}
func (calls *Calls) GetCall(id uint, db *Database) (*Call, error) {
	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	call := Call{Id: id}

	r, err := db.Ent.Call.Query().Where(entcall.ID(int(id))).Only(context.Background())
	if ent.IsNotFound(err) {
		return &call, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getcall: %w", err)
	}

	call.Audio = r.Audio
	call.System = uint(r.System)
	call.Talkgroup = uint(r.Talkgroup)
	call.DateTime = r.DateTime.UTC()

	if r.AudioName != nil {
		call.AudioName = *r.AudioName
	}
	if r.AudioType != nil {
		call.AudioType = *r.AudioType
	}
	if r.Frequency != nil && *r.Frequency > 0 {
		call.Frequency = uint(*r.Frequency)
	}
	if r.Source != nil && *r.Source > 0 {
		call.Source = uint(*r.Source)
	}
	if len(r.Frequencies) > 0 {
		if json.Unmarshal([]byte(r.Frequencies), &call.Frequencies) != nil {
			call.Frequencies = []any{}
		}
	}
	if len(r.Patches) > 0 {
		if json.Unmarshal([]byte(r.Patches), &call.Patches) != nil {
			call.Patches = []any{}
		}
	}
	if len(r.Sources) > 0 {
		if json.Unmarshal([]byte(r.Sources), &call.Sources) != nil {
			call.Sources = []any{}
		}
	}

	return &call, nil
}
func (calls *Calls) Prune(db *Database, pruneDays uint) error {
	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	cutoff := time.Now().UTC().Add(-24 * time.Hour * time.Duration(pruneDays))
	_, err := db.Ent.Call.Delete().Where(entcall.DateTimeLT(cutoff)).Exec(context.Background())

	return err
}
func (calls *Calls) Search(searchOptions *CallsSearchOptions, client *Client) (*CallsSearchResults, error) {
	const (
		ascOrder  = 1
		descOrder = -1
	)

	ctx := context.Background()

	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	db := client.Controller.Database

	formatError := func(err error) error {
		return fmt.Errorf("calls.search: %w", err)
	}

	searchResults := &CallsSearchResults{
		Options: searchOptions,
		Results: []CallsSearchResult{},
	}

	// scopeAndTalkgroups builds "(system = id and talkgroup in tgs)" for a
	// system/talkgroups pair, or "system = id" when talkgroups is all ("*").
	idFromAny := func(v any) (int, bool) { return rowID(v) }
	tgList := func(v any) []int {
		out := []int{}
		if list, ok := v.([]any); ok {
			for _, e := range list {
				if n, ok := rowID(e); ok {
					out = append(out, n)
				}
			}
		}
		return out
	}

	preds := []predicate.Call{}

	// Access scoping.
	if client.Access != nil {
		if list, ok := client.Access.Systems.([]any); ok {
			scopes := []predicate.Call{}
			for _, scope := range list {
				m, ok := scope.(map[string]any)
				if !ok {
					continue
				}
				id, ok := idFromAny(m["id"])
				if !ok {
					continue
				}
				switch tg := m["talkgroups"].(type) {
				case []any:
					scopes = append(scopes, entcall.And(entcall.System(id), entcall.TalkgroupIn(tgList(tg)...)))
				case string:
					if tg == "*" {
						scopes = append(scopes, entcall.System(id))
					}
				}
			}
			preds = append(preds, entcall.Or(scopes...))
		}
	}

	// System, then system+talkgroup (with optional patched-talkgroup match).
	if system, ok := searchOptions.System.(uint); ok {
		sysPreds := []predicate.Call{entcall.System(int(system))}
		if talkgroup, ok := searchOptions.Talkgroup.(uint); ok {
			if searchOptions.searchPatchedTalkgroups {
				tg := int(talkgroup)
				sysPreds = append(sysPreds, entcall.Or(
					entcall.Talkgroup(tg),
					entcall.Patches(fmt.Sprintf("[%d]", tg)),
					entcall.PatchesHasPrefix(fmt.Sprintf("[%d,", tg)),
					entcall.PatchesContains(fmt.Sprintf(",%d,", tg)),
					entcall.PatchesHasSuffix(fmt.Sprintf(",%d]", tg)),
				))
			} else {
				sysPreds = append(sysPreds, entcall.Talkgroup(int(talkgroup)))
			}
		}
		preds = append(preds, entcall.And(sysPreds...))
	}

	// Group and tag filters use the client's label -> system -> talkgroups maps.
	mapPred := func(m map[uint][]uint) {
		if len(m) == 0 {
			return
		}
		parts := []predicate.Call{}
		for id, tgs := range m {
			ints := make([]int, 0, len(tgs))
			for _, tg := range tgs {
				ints = append(ints, int(tg))
			}
			parts = append(parts, entcall.And(entcall.System(int(id)), entcall.TalkgroupIn(ints...)))
		}
		preds = append(preds, entcall.Or(parts...))
	}
	if v, ok := searchOptions.Group.(string); ok {
		mapPred(client.GroupsMap[v])
	}
	if v, ok := searchOptions.Tag.(string); ok {
		mapPred(client.TagsMap[v])
	}

	// DateStart/DateStop reflect the filter above, not the date window below.
	if first, err := db.Ent.Call.Query().Where(preds...).Order(ent.Asc(entcall.FieldDateTime)).First(ctx); err == nil {
		searchResults.DateStart = first.DateTime.UTC()
	} else if !ent.IsNotFound(err) {
		return nil, formatError(err)
	}
	if last, err := db.Ent.Call.Query().Where(preds...).Order(ent.Desc(entcall.FieldDateTime)).First(ctx); err == nil {
		searchResults.DateStop = last.DateTime.UTC()
	} else if ent.IsNotFound(err) {
		searchResults.DateStop = time.Now()
	} else {
		return nil, formatError(err)
	}

	sort := ascOrder
	if v, ok := searchOptions.Sort.(int); ok && v < 0 {
		sort = descOrder
	}

	if v, ok := searchOptions.Date.(time.Time); ok {
		minute := v.UTC().Truncate(time.Minute)
		var start, stop time.Time
		if sort == ascOrder {
			start = minute
			stop = minute.Add(24 * time.Hour)
		} else {
			stop = minute.Add(time.Minute)
			start = stop.Add(-24 * time.Hour)
		}
		preds = append(preds, entcall.DateTimeGTE(start), entcall.DateTimeLT(stop))
	}

	limit := 200
	if v, ok := searchOptions.Limit.(uint); ok {
		if v > 500 {
			limit = 500
		} else {
			limit = int(v)
		}
	}
	offset := 0
	if v, ok := searchOptions.Offset.(uint); ok {
		offset = int(v)
	}

	count, err := db.Ent.Call.Query().Where(preds...).Count(ctx)
	if err != nil {
		return nil, formatError(err)
	}
	searchResults.Count = uint(count)

	order := ent.Asc(entcall.FieldDateTime)
	if sort == descOrder {
		order = ent.Desc(entcall.FieldDateTime)
	}

	records, err := db.Ent.Call.Query().Where(preds...).Order(order).Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, formatError(err)
	}

	for _, r := range records {
		searchResults.Results = append(searchResults.Results, CallsSearchResult{
			Id:        uint(r.ID),
			DateTime:  r.DateTime.UTC(),
			System:    uint(r.System),
			Talkgroup: uint(r.Talkgroup),
		})
	}

	return searchResults, nil
}
func (calls *Calls) WriteCall(call *Call, db *Database) (uint, error) {
	var (
		b           []byte
		err         error
		frequencies string
		patches     string
		sources     string
	)

	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	formatError := func(err error) error {
		return fmt.Errorf("call.write: %w", err)
	}

	if v, ok := call.Frequencies.([]map[string]any); ok {
		if b, err = json.Marshal(v); err != nil {
			return 0, formatError(err)
		}
		frequencies = string(b)
	}
	if v, ok := call.Patches.([]uint); ok {
		if b, err = json.Marshal(v); err != nil {
			return 0, formatError(err)
		}
		patches = string(b)
	}
	if v, ok := call.Sources.([]map[string]any); ok {
		if b, err = json.Marshal(v); err != nil {
			return 0, formatError(err)
		}
		sources = string(b)
	}

	create := db.Ent.Call.Create().
		SetAudio(call.Audio).
		SetDateTime(call.DateTime).
		SetFrequencies(frequencies).
		SetPatches(patches).
		SetSources(sources).
		SetSystem(int(call.System)).
		SetTalkgroup(int(call.Talkgroup)).
		SetNillableAudioName(nillableStr(call.AudioName)).
		SetNillableAudioType(nillableStr(call.AudioType)).
		SetNillableFrequency(nillablePosInt(call.Frequency)).
		SetNillableSource(nillablePosInt(call.Source))

	if id, ok := rowID(call.Id); ok {
		create.SetID(id)
	}

	created, err := create.Save(context.Background())
	if err != nil {
		return 0, formatError(err)
	}

	return uint(created.ID), nil
}

type CallsSearchOptions struct {
	Date                    any `json:"date,omitempty"`
	Group                   any `json:"group,omitempty"`
	Limit                   any `json:"limit,omitempty"`
	Offset                  any `json:"offset,omitempty"`
	Sort                    any `json:"sort,omitempty"`
	System                  any `json:"system,omitempty"`
	Tag                     any `json:"tag,omitempty"`
	Talkgroup               any `json:"talkgroup,omitempty"`
	searchPatchedTalkgroups bool
}

func (searchOptions *CallsSearchOptions) fromMap(m map[string]any) error {
	switch v := m["date"].(type) {
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			searchOptions.Date = t
		}
	}

	switch v := m["group"].(type) {
	case string:
		searchOptions.Group = v
	}

	switch v := m["limit"].(type) {
	case float64:
		searchOptions.Limit = uint(v)
	}

	switch v := m["offset"].(type) {
	case float64:
		searchOptions.Offset = uint(v)
	}

	switch v := m["sort"].(type) {
	case float64:
		searchOptions.Sort = int(v)
	}

	switch v := m["system"].(type) {
	case float64:
		searchOptions.System = uint(v)
	}

	switch v := m["tag"].(type) {
	case string:
		searchOptions.Tag = v
	}

	switch v := m["talkgroup"].(type) {
	case float64:
		searchOptions.Talkgroup = uint(v)
	}

	return nil
}

type CallsSearchResult struct {
	Id        uint      `json:"id"`
	DateTime  time.Time `json:"dateTime"`
	System    uint      `json:"system"`
	Talkgroup uint      `json:"talkgroup"`
}

type CallsSearchResults struct {
	Count     uint                `json:"count"`
	DateStart time.Time           `json:"dateStart"`
	DateStop  time.Time           `json:"dateStop"`
	Options   *CallsSearchOptions `json:"options"`
	Results   []CallsSearchResult `json:"results"`
}
