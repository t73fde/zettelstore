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
	"sync"

	"t73f.de/r/zero/roster"
	"t73f.de/r/zero/vocab"
)

type store struct {
	stats *stats

	mx     sync.RWMutex
	zettel map[denseZid]*storeZettel                // all existing zettel
	words  map[vocab.WordID]roster.Roster[denseZid] // word -> zettel containing it
	urls   map[string]roster.Roster[denseZid]       // URL -> zettel containing it
}

// storeZettel stores data of a specific zettel.
//
// TODO: just an idea: make it immutable.
type storeZettel struct {
	meta  storeMeta
	words roster.Roster[vocab.WordID]
	urls  roster.Roster[string]
}

func makeStore(stats *stats) *store {
	return &store{
		stats:  stats,
		zettel: make(map[denseZid]*storeZettel),
		words:  make(map[vocab.WordID]roster.Roster[denseZid]),
		urls:   make(map[string]roster.Roster[denseZid]),
	}
}

// grow dimensions an empty store. It has no effect on a store with content.
func (st *store) grow(n int) {
	st.mx.Lock()
	defer st.mx.Unlock()
	if len(st.zettel) == 0 && n > 0 {
		st.zettel = make(map[denseZid]*storeZettel, n)
	}
}

func (st *store) count() int {
	st.mx.RLock()
	defer st.mx.RUnlock()
	return len(st.zettel)
}

// upsertZettel takes ownership of az.
func (st *store) upsertZettel(dzid denseZid, az *analyzedZettel) {
	st.replace(dzid, az)
	st.stats.finishUpdateZettel()
}

func (st *store) replace(dzid denseZid, az *analyzedZettel) {
	st.mx.Lock()
	defer st.mx.Unlock()

	newSz := st.buildStoreZettel(az)
	if prevSz, found := st.zettel[dzid]; found {
		st.updateZettel(dzid, prevSz, newSz)
	} else {
		st.insertZettel(dzid, newSz)
	}
	st.zettel[dzid] = newSz
}

func (st *store) insertZettel(dzid denseZid, newSz *storeZettel) {
	for word := range newSz.words.Values() {
		r := st.words[word]
		r.Insert(dzid)
		st.words[word] = r
	}
	for u := range newSz.urls.Values() {
		r := st.urls[u]
		r.Insert(dzid)
		st.urls[u] = r
	}
}

func (st *store) updateZettel(dzid denseZid, prevSz, newSz *storeZettel) {
}

func (st *store) buildStoreZettel(az *analyzedZettel) *storeZettel {
	result := &storeZettel{
		meta:  az.meta,
		words: az.words,
		urls:  az.urls,
	}
	return result
}

// ----- removeZettel --------------------------------------------------------

func (st *store) removeZettel(dzid denseZid) {
	if st.drop(dzid) {
		st.stats.finishRemoveZettel()
	}
}

func (st *store) drop(dzid denseZid) bool {
	st.mx.Lock()
	defer st.mx.Unlock()

	if sz, found := st.zettel[dzid]; found {
		delete(st.zettel, dzid)
		st.deleteZettel(sz)
		return true
	}
	return false
}

func (st *store) deleteZettel(sz *storeZettel) {}
