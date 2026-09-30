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
	"fmt"
	"sort"
	"sync"

	enttalkgroup "github.com/USA-RedDragon/rdio-scanner/server/ent/talkgroup"
)

type Talkgroup struct {
	Frequency any `json:"frequency"`
	group     string
	GroupId   uint   `json:"groupId"`
	Id        uint   `json:"id"`
	Label     string `json:"label"`
	Led       any    `json:"led"`
	Name      string `json:"name"`
	Order     uint   `json:"order"`
	TagId     uint   `json:"tagId"`
	tag       string
}

func (talkgroup *Talkgroup) FromMap(m map[string]any) *Talkgroup {
	switch v := m["id"].(type) {
	case float64:
		talkgroup.Id = uint(v)
	}

	switch v := m["frequency"].(type) {
	case float64:
		talkgroup.Frequency = uint(v)
	}

	switch v := m["group"].(type) {
	case string:
		talkgroup.group = v
	}

	switch v := m["groupId"].(type) {
	case float64:
		talkgroup.GroupId = uint(v)
	}

	switch v := m["label"].(type) {
	case string:
		talkgroup.Label = v
	}

	switch v := m["led"].(type) {
	case string:
		talkgroup.Led = v
	}

	switch v := m["name"].(type) {
	case string:
		talkgroup.Name = v
	}

	switch v := m["order"].(type) {
	case float64:
		talkgroup.Order = uint(v)
	}

	switch v := m["tag"].(type) {
	case string:
		talkgroup.tag = v
	}

	switch v := m["tagId"].(type) {
	case float64:
		talkgroup.TagId = uint(v)
	}

	return talkgroup
}

type TalkgroupMap map[string]any

type Talkgroups struct {
	List  []*Talkgroup
	mutex sync.Mutex
}

func NewTalkgroups() *Talkgroups {
	return &Talkgroups{
		List:  []*Talkgroup{},
		mutex: sync.Mutex{},
	}
}

func (talkgroups *Talkgroups) FromMap(f []any) *Talkgroups {
	talkgroups.mutex.Lock()
	defer talkgroups.mutex.Unlock()

	talkgroups.List = []*Talkgroup{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			talkgroup := &Talkgroup{}
			talkgroup.FromMap(m)
			talkgroups.List = append(talkgroups.List, talkgroup)
		}
	}

	return talkgroups
}

func (talkgroups *Talkgroups) GetTalkgroup(f any) (system *Talkgroup, ok bool) {
	talkgroups.mutex.Lock()
	defer talkgroups.mutex.Unlock()

	switch v := f.(type) {
	case uint:
		for _, talkgroup := range talkgroups.List {
			if talkgroup.Id == v {
				return talkgroup, true
			}
		}
	case string:
		for _, talkgroup := range talkgroups.List {
			if talkgroup.Label == v {
				return talkgroup, true
			}
		}
	}

	return nil, false
}

func (talkgroups *Talkgroups) Read(db *Database, systemId uint) error {
	talkgroups.mutex.Lock()
	defer talkgroups.mutex.Unlock()

	talkgroups.List = []*Talkgroup{}

	records, err := db.Ent.Talkgroup.Query().Where(enttalkgroup.SystemID(int(systemId))).All(context.Background())
	if err != nil {
		return fmt.Errorf("talkgroups.read: %w", err)
	}

	for _, r := range records {
		talkgroup := &Talkgroup{
			GroupId: uint(r.GroupID),
			Id:      uint(r.TalkgroupID),
			Label:   r.Label,
			Name:    r.Name,
			Order:   0,
			TagId:   uint(r.TagID),
		}
		if r.Order != nil && *r.Order > 0 {
			talkgroup.Order = uint(*r.Order)
		}
		if r.Frequency != nil && *r.Frequency > 0 {
			talkgroup.Frequency = uint(*r.Frequency)
		}
		if r.Led != nil && len(*r.Led) > 0 {
			talkgroup.Led = *r.Led
		}
		talkgroups.List = append(talkgroups.List, talkgroup)
	}

	sort.Slice(talkgroups.List, func(i int, j int) bool {
		return talkgroups.List[i].Order < talkgroups.List[j].Order
	})

	return nil
}
func (talkgroups *Talkgroups) Write(db *Database, systemId uint) error {
	talkgroups.mutex.Lock()
	defer talkgroups.mutex.Unlock()

	ctx := context.Background()
	sysId := int(systemId)

	formatError := func(err error) error {
		return fmt.Errorf("talkgroups.write: %w", err)
	}

	keep := make([]int, 0, len(talkgroups.List))
	for _, talkgroup := range talkgroups.List {
		keep = append(keep, int(talkgroup.Id))
	}

	if _, err := db.Ent.Talkgroup.Delete().Where(enttalkgroup.SystemID(sysId), enttalkgroup.TalkgroupIDNotIn(keep...)).Exec(ctx); err != nil {
		return formatError(err)
	}

	for _, talkgroup := range talkgroups.List {
		exists, err := db.Ent.Talkgroup.Query().Where(enttalkgroup.SystemID(sysId), enttalkgroup.TalkgroupID(int(talkgroup.Id))).Exist(ctx)
		if err != nil {
			return formatError(err)
		}
		if exists {
			upd := db.Ent.Talkgroup.Update().Where(enttalkgroup.SystemID(sysId), enttalkgroup.TalkgroupID(int(talkgroup.Id))).
				SetGroupID(int(talkgroup.GroupId)).
				SetTagID(int(talkgroup.TagId)).
				SetLabel(talkgroup.Label).
				SetName(talkgroup.Name)
			setOrClearInt(upd.SetFrequency, upd.ClearFrequency, nillablePosInt(talkgroup.Frequency))
			setOrClearInt(upd.SetOrder, upd.ClearOrder, nillablePosInt(talkgroup.Order))
			setOrClearStr(upd.SetLed, upd.ClearLed, nillableStr(talkgroup.Led))
			if _, err := upd.Save(ctx); err != nil {
				return formatError(err)
			}
		} else {
			if err := db.Ent.Talkgroup.Create().
				SetTalkgroupID(int(talkgroup.Id)).
				SetSystemID(sysId).
				SetGroupID(int(talkgroup.GroupId)).
				SetTagID(int(talkgroup.TagId)).
				SetLabel(talkgroup.Label).
				SetName(talkgroup.Name).
				SetNillableFrequency(nillablePosInt(talkgroup.Frequency)).
				SetNillableOrder(nillablePosInt(talkgroup.Order)).
				SetNillableLed(nillableStr(talkgroup.Led)).
				Exec(ctx); err != nil {
				return formatError(err)
			}
		}
	}

	return nil
}

type TalkgroupsMap []TalkgroupMap
