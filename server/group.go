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
	"sync"

	entgroup "github.com/USA-RedDragon/rdio-scanner/server/ent/group"
)

type Group struct {
	Id    any    `json:"_id"`
	Label string `json:"label"`
}

func (group *Group) FromMap(m map[string]any) *Group {
	switch v := m["_id"].(type) {
	case float64:
		group.Id = uint(v)
	}

	switch v := m["label"].(type) {
	case string:
		group.Label = v
	}

	return group
}

type Groups struct {
	List  []*Group
	mutex sync.Mutex
}

func NewGroups() *Groups {
	return &Groups{
		List:  []*Group{},
		mutex: sync.Mutex{},
	}
}

func (groups *Groups) FromMap(f []any) *Groups {
	groups.mutex.Lock()
	defer groups.mutex.Unlock()

	groups.List = []*Group{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			group := &Group{}
			group.FromMap(m)
			groups.List = append(groups.List, group)
		}
	}

	return groups
}

func (groups *Groups) GetGroup(f any) (group *Group, ok bool) {
	groups.mutex.Lock()
	defer groups.mutex.Unlock()

	switch v := f.(type) {
	case uint:
		for _, group := range groups.List {
			if group.Id == v {
				return group, true
			}
		}
	case string:
		for _, group := range groups.List {
			if group.Label == v {
				return group, true
			}
		}
	}

	return nil, false
}

func (groups *Groups) GetGroupsMap(systemsMap *SystemsMap) GroupsMap {
	var groupsMap = GroupsMap{}

	for _, system := range *systemsMap {
		var (
			fSystemId     = system["id"]
			fTalkgroups   = system["talkgroups"]
			systemId      uint
			talkgroupsMap TalkgroupsMap
		)

		switch v := fSystemId.(type) {
		case uint:
			systemId = v
		}

		switch v := fTalkgroups.(type) {
		case TalkgroupsMap:
			talkgroupsMap = v
		}

		for _, talkgroup := range talkgroupsMap {
			var (
				fTalkgroupGroup = talkgroup["group"]
				fTalkgroupId    = talkgroup["id"]
				talkgroupGroup  string
				talkgroupId     uint
			)

			switch v := fTalkgroupGroup.(type) {
			case string:
				talkgroupGroup = v
			}

			switch v := fTalkgroupId.(type) {
			case uint:
				talkgroupId = v
			}

			group, ok := groups.GetGroup(talkgroupGroup)
			if !ok {
				continue
			}

			if groupsMap[group.Label] == nil {
				groupsMap[group.Label] = map[uint][]uint{}
			}

			if groupsMap[group.Label][systemId] == nil {
				groupsMap[group.Label][systemId] = []uint{}
			}

			found := false
			for _, id := range groupsMap[group.Label][systemId] {
				if id == talkgroupId {
					found = true
					break
				}
			}
			if !found {
				groupsMap[group.Label][systemId] = append(groupsMap[group.Label][systemId], talkgroupId)
			}
		}
	}

	return groupsMap
}

func (groups *Groups) Read(db *Database) error {
	groups.mutex.Lock()
	defer groups.mutex.Unlock()

	groups.List = []*Group{}

	records, err := db.Ent.Group.Query().All(context.Background())
	if err != nil {
		return fmt.Errorf("groups.read: %w", err)
	}

	for _, r := range records {
		if len(r.Label) == 0 {
			continue
		}
		groups.List = append(groups.List, &Group{Id: uint(r.ID), Label: r.Label})
	}

	return nil
}

func (groups *Groups) Write(db *Database) error {
	groups.mutex.Lock()
	defer groups.mutex.Unlock()

	ctx := context.Background()

	formatError := func(err error) error {
		return fmt.Errorf("groups.write: %w", err)
	}

	// Ids to keep are those of existing items still in the list. Rows whose id
	// is not kept are removed; items without an id are new and get inserted.
	// (New items no longer suppress deletion, the old sync-list bug.)
	keep := make([]int, 0, len(groups.List))
	for _, group := range groups.List {
		if id, ok := rowID(group.Id); ok {
			keep = append(keep, id)
		}
	}

	if _, err := db.Ent.Group.Delete().Where(entgroup.IDNotIn(keep...)).Exec(ctx); err != nil {
		return formatError(err)
	}

	for _, group := range groups.List {
		if id, ok := rowID(group.Id); ok {
			if err := db.Ent.Group.UpdateOneID(id).SetLabel(group.Label).Exec(ctx); err != nil {
				return formatError(err)
			}
		} else {
			if err := db.Ent.Group.Create().SetLabel(group.Label).Exec(ctx); err != nil {
				return formatError(err)
			}
		}
	}

	return nil
}

type GroupsMap map[string]map[uint][]uint
