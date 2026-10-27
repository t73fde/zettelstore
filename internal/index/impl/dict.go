//-----------------------------------------------------------------------------
// Copyright (c) 2026-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2026-present Detlef Stern
//-----------------------------------------------------------------------------

package impl

import (
	"slices"
	"sync"

	"t73f.de/r/zero/dense"
	"t73f.de/r/zero/roster"
	"t73f.de/r/zero/vocab"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

type denseZid uint32

// dictionary maps zettel identifier to dense identifier and words to word
// identifier. Identifier are never given back; a dense identifier may
// therefore refer to a zettel that does not (yet) exist. The store decides
// about existence. All methods are safe for concurrent use.
type dictionary struct {
	mx    sync.RWMutex
	dense *dense.Dense[id.Zid, denseZid]
	voc   *vocab.Vocabulary

	// inverse maps metadata keys to their inverse. It is read-only after
	// construction. Assumes that there will be no future inverse keys.
	inverse map[vocab.WordID]vocab.WordID
}

func makeDictionary(hints sizeHints) *dictionary {
	voc := vocab.New(hints.words)

	inverse := make(map[vocab.WordID]vocab.WordID, 8)
	for kd := range meta.KeyDescriptionSeq() {
		keyID := voc.AddString(kd.Name)
		if invKey := kd.Inverse; invKey != "" {
			invID := voc.AddString(invKey)
			inverse[keyID] = invID
			inverse[invID] = keyID
		}
	}
	d := &dense.Dense[id.Zid, denseZid]{}
	d.Grow(hints.zettel)
	return &dictionary{
		dense:   d,
		voc:     voc,
		inverse: inverse,
	}
}

func (d *dictionary) Encode(zid id.Zid) denseZid {
	d.mx.Lock()
	defer d.mx.Unlock()
	return d.dense.Encode(zid)
}

func (d *dictionary) Decode(dzid denseZid) (id.Zid, bool) {
	d.mx.RLock()
	defer d.mx.RUnlock()
	return d.dense.Decode(dzid)
}

// Find returns the dense identifier of zid, if one was already given. It does
// not give a new one.
func (d *dictionary) Find(zid id.Zid) (denseZid, bool) {
	d.mx.RLock()
	defer d.mx.RUnlock()
	return d.dense.Find(zid)
}

func (d *dictionary) AddBytes(b []byte) vocab.WordID {
	d.mx.Lock()
	defer d.mx.Unlock()
	return d.voc.AddBytes(b)
}

func (d *dictionary) AddString(s string) vocab.WordID {
	d.mx.Lock()
	defer d.mx.Unlock()
	return d.voc.AddString(s)
}

// LookupString returns the identifier of s, if it is known. Otherwise
// vocab.InvalidWordID is returned.
func (d *dictionary) LookupString(s string) vocab.WordID {
	d.mx.RLock()
	defer d.mx.RUnlock()
	return d.voc.LookupString(s)
}

func (d *dictionary) GetString(id vocab.WordID) string {
	d.mx.RLock()
	defer d.mx.RUnlock()
	return d.voc.GetString(id)
}

func (d *dictionary) numWords() int {
	d.mx.RLock()
	defer d.mx.RUnlock()
	return d.voc.Count()
}

func (d *dictionary) numZids() int {
	d.mx.RLock()
	defer d.mx.RUnlock()
	return d.dense.Count()
}

func (d *dictionary) readStats(st *dictStats) {
	d.mx.RLock()
	defer d.mx.RUnlock()
	st.NumWords = d.voc.Count()
	st.Vocabulary = d.voc.StoreLen()
	st.WordCount = d.voc.WordCount()
}

type dictStats struct {
	NumWords   int
	Vocabulary int
	WordCount  map[uint16]int
}

// buildMeta converts the compact form into a metadata object.
func (d *dictionary) buildMeta(zid id.Zid, sm storeMeta) *meta.Meta {
	pairs := make(map[meta.Key]meta.Value, len(sm.ks))
	d.mx.RLock()
	for i, k := range sm.ks {
		pairs[d.voc.GetString(k)] = sm.vs[i]
	}
	d.mx.RUnlock()
	return meta.NewWithMap(zid, pairs)
}

// lookupZidText returns the dense identifier of the zid that is represented by
// the text s. It reports false, if s is not a zid or if the zid is unknown.
func (d *dictionary) lookupZidText(s string) (denseZid, bool) {
	if zid, err := id.Parse(s); err == nil {
		return d.Find(zid)
	}
	return 0, false
}

// zidRoster converts a set of dense identifier into a set of zettel identifier.
func (d *dictionary) zidRoster(dzids roster.Roster[denseZid]) roster.Roster[id.Zid] {
	var result roster.Roster[id.Zid]
	if dzids.IsEmpty() {
		return result
	}

	zids := make([]id.Zid, 0, dzids.Count())
	d.mx.RLock()
	for dz := range dzids.Values() {
		if zid, found := d.dense.Decode(dz); found {
			zids = append(zids, zid)
		}
	}
	d.mx.RUnlock()

	// The order of dense identifier is not the order of zettel identifier.
	// Inserting in ascending order appends and does not shift elements.
	slices.Sort(zids)
	for _, zid := range zids {
		result.Insert(zid)
	}
	return result
}
