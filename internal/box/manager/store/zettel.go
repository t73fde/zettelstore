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
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"
)

// ZettelIndex contains all index data of a zettel.
type ZettelIndex struct {
	Zid         id.Zid                // zid of the indexed zettel
	meta        *meta.Meta            // full metadata
	backrefs    *idset.Set            // set of back references
	inverseRefs map[string]*idset.Set // references of inverse keys
	deadrefs    *idset.Set            // set of dead references
	words       WordSet
	urls        WordSet
}

// NewZettelIndex creates a new zettel index.
func NewZettelIndex(m *meta.Meta) *ZettelIndex {
	return &ZettelIndex{
		Zid:         m.Zid,
		meta:        m,
		backrefs:    idset.New(),
		inverseRefs: make(map[string]*idset.Set),
		deadrefs:    idset.New(),
	}
}

// AddBackRef adds a reference to a zettel where the current zettel links to
// without any more information.
func (zi *ZettelIndex) AddBackRef(zid id.Zid) { zi.backrefs.Add(zid) }

// AddInverseRef adds a named reference to a zettel. On that zettel, the given
// metadata key should point back to the current zettel.
func (zi *ZettelIndex) AddInverseRef(key string, zid id.Zid) {
	if zids, ok := zi.inverseRefs[key]; ok {
		zids.Add(zid)
		return
	}
	zi.inverseRefs[key] = idset.New(zid)
}

// AddDeadRef adds a dead reference to a zettel.
func (zi *ZettelIndex) AddDeadRef(zid id.Zid) {
	zi.deadrefs.Add(zid)
}

// SetWords sets the words to the given value.
func (zi *ZettelIndex) SetWords(words WordSet) { zi.words = words }

// SetUrls sets the words to the given value.
func (zi *ZettelIndex) SetUrls(urls WordSet) { zi.urls = urls }

// GetDeadRefs returns all dead references as a sorted list.
func (zi *ZettelIndex) GetDeadRefs() *idset.Set { return zi.deadrefs }

// GetMeta return just the raw metadata.
func (zi *ZettelIndex) GetMeta() *meta.Meta { return zi.meta }

// GetBackRefs returns all back references as a sorted list.
func (zi *ZettelIndex) GetBackRefs() *idset.Set { return zi.backrefs }

// GetInverseRefs returns all inverse meta references as a map of strings to a sorted list of references
func (zi *ZettelIndex) GetInverseRefs() map[string]*idset.Set {
	return maps.Clone(zi.inverseRefs)
}

// GetWords returns a reference to the set of words. It must not be modified.
func (zi *ZettelIndex) GetWords() WordSet { return zi.words }

// GetUrls returns a reference to the set of URLs. It must not be modified.
func (zi *ZettelIndex) GetUrls() WordSet { return zi.urls }
