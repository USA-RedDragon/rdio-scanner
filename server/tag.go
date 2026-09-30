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

	enttag "github.com/USA-RedDragon/rdio-scanner/server/ent/tag"
)

type Tag struct {
	Id    any    `json:"_id"`
	Label string `json:"label"`
}

func (tag *Tag) FromMap(m map[string]any) *Tag {
	switch v := m["_id"].(type) {
	case float64:
		tag.Id = uint(v)
	}

	switch v := m["label"].(type) {
	case string:
		tag.Label = v
	}

	return tag
}

type Tags struct {
	List  []*Tag
	mutex sync.Mutex
}

func NewTags() *Tags {
	return &Tags{
		List:  []*Tag{},
		mutex: sync.Mutex{},
	}
}

func (tags *Tags) FromMap(f []any) *Tags {
	tags.mutex.Lock()
	defer tags.mutex.Unlock()

	tags.List = []*Tag{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			tag := &Tag{}
			tag.FromMap(m)
			tags.List = append(tags.List, tag)
		}
	}

	return tags
}

func (tags *Tags) GetTag(f any) (tag *Tag, ok bool) {
	tags.mutex.Lock()
	defer tags.mutex.Unlock()

	switch v := f.(type) {
	case uint:
		for _, tag := range tags.List {
			if tag.Id == v {
				return tag, true
			}
		}
	case string:
		for _, tag := range tags.List {
			if tag.Label == v {
				return tag, true
			}
		}
	}

	return nil, false
}

func (tags *Tags) GetTagsMap(systemsMap *SystemsMap) TagsMap {
	tagsMap := TagsMap{}

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
				fTalkgroupTag = talkgroup["tag"]
				fTalkgroupId  = talkgroup["id"]
				talkgroupTag  string
				talkgroupId   uint
			)

			switch v := fTalkgroupTag.(type) {
			case string:
				talkgroupTag = v
			}

			switch v := fTalkgroupId.(type) {
			case uint:
				talkgroupId = v
			}

			tag, ok := tags.GetTag(talkgroupTag)
			if !ok {
				continue
			}

			if tagsMap[tag.Label] == nil {
				tagsMap[tag.Label] = map[uint][]uint{}
			}

			if tagsMap[tag.Label][systemId] == nil {
				tagsMap[tag.Label][systemId] = []uint{}
			}

			found := false
			for _, id := range tagsMap[tag.Label][systemId] {
				if id == talkgroupId {
					found = true
					break
				}
			}
			if !found {
				tagsMap[tag.Label][systemId] = append(tagsMap[tag.Label][systemId], talkgroupId)
			}
		}
	}

	return tagsMap
}

func (tags *Tags) Read(db *Database) error {
	tags.mutex.Lock()
	defer tags.mutex.Unlock()

	tags.List = []*Tag{}

	records, err := db.Ent.Tag.Query().All(context.Background())
	if err != nil {
		return fmt.Errorf("tags.read: %w", err)
	}

	for _, r := range records {
		tags.List = append(tags.List, &Tag{Id: uint(r.ID), Label: r.Label})
	}

	return nil
}

func (tags *Tags) Write(db *Database) error {
	tags.mutex.Lock()
	defer tags.mutex.Unlock()

	ctx := context.Background()

	formatError := func(err error) error {
		return fmt.Errorf("tags.write: %w", err)
	}

	keep := make([]int, 0, len(tags.List))
	for _, tag := range tags.List {
		if id, ok := rowID(tag.Id); ok {
			keep = append(keep, id)
		}
	}

	if _, err := db.Ent.Tag.Delete().Where(enttag.IDNotIn(keep...)).Exec(ctx); err != nil {
		return formatError(err)
	}

	for _, tag := range tags.List {
		if id, ok := rowID(tag.Id); ok {
			if err := db.Ent.Tag.UpdateOneID(id).SetLabel(tag.Label).Exec(ctx); err != nil {
				return formatError(err)
			}
		} else {
			if err := db.Ent.Tag.Create().SetLabel(tag.Label).Exec(ctx); err != nil {
				return formatError(err)
			}
		}
	}

	return nil
}

type TagsMap map[string]map[uint][]uint
