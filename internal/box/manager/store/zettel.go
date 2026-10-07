//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package store

import (
	"maps"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/idset"
)

// ZettelIndex contains all index data of a zettel.
type ZettelIndex struct {
	Zid         id.Zid                  // zid of the indexed zettel
	meta        *meta.Meta              // full metadata
	backrefs    idset.ZidSet            // set of back references
	inverseRefs map[string]idset.ZidSet // references of inverse keys
	deadrefs    idset.ZidSet            // set of dead references
	words       WordSet
	urls        WordSet
}

// NewZettelIndex creates a new zettel index.
func NewZettelIndex(m *meta.Meta) *ZettelIndex {
	return &ZettelIndex{
		Zid:         m.Zid,
		meta:        m,
		backrefs:    idset.ZidSet{},
		inverseRefs: make(map[string]idset.ZidSet),
		deadrefs:    idset.ZidSet{},
	}
}

// AddBackRef adds a reference to a zettel where the current zettel links to
// without any more information.
func (zi *ZettelIndex) AddBackRef(zid id.Zid) {
	zi.backrefs.Insert(zid)
}

// AddInverseRef adds a named reference to a zettel. On that zettel, the given
// metadata key should point back to the current zettel.
func (zi *ZettelIndex) AddInverseRef(key string, zid id.Zid) {
	if zids, ok := zi.inverseRefs[key]; ok {
		zids.Insert(zid)
		return
	}
	var s idset.ZidSet
	s.Insert(zid)
	zi.inverseRefs[key] = s
}

// AddDeadRef adds a dead reference to a zettel.
func (zi *ZettelIndex) AddDeadRef(zid id.Zid) {
	zi.deadrefs.Insert(zid)
}

// SetWords sets the words to the given value.
func (zi *ZettelIndex) SetWords(words WordSet) { zi.words = words }

// SetUrls sets the words to the given value.
func (zi *ZettelIndex) SetUrls(urls WordSet) { zi.urls = urls }

// GetDeadRefs returns all dead references as a sorted list.
func (zi *ZettelIndex) GetDeadRefs() idset.ZidSet { return zi.deadrefs }

// GetMeta return just the raw metadata.
func (zi *ZettelIndex) GetMeta() *meta.Meta { return zi.meta }

// GetBackRefs returns all back references as a sorted list.
func (zi *ZettelIndex) GetBackRefs() idset.ZidSet { return zi.backrefs }

// GetInverseRefs returns all inverse meta references as a map of strings to a sorted list of references
func (zi *ZettelIndex) GetInverseRefs() map[string]idset.ZidSet {
	return maps.Clone(zi.inverseRefs)
}

// GetWords returns a reference to the set of words. It must not be modified.
func (zi *ZettelIndex) GetWords() WordSet { return zi.words }

// GetUrls returns a reference to the set of URLs. It must not be modified.
func (zi *ZettelIndex) GetUrls() WordSet { return zi.urls }
