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
	"cmp"
	"sync"

	"t73f.de/r/zero/roster"
	"t73f.de/r/zero/vocab"
)

type store struct {
	stats *stats

	mx     sync.RWMutex
	hint   int                                      // expected number of zettel
	zettel map[denseZid]*storeZettel                // all existing zettel
	words  map[vocab.WordID]roster.Roster[denseZid] // word -> zettel containing it
	urls   map[string]roster.Roster[denseZid]       // URL -> zettel containing it
}

// storeZettel stores data of a specific zettel. After adding it to a store,
// it is treated as immutable.
type storeZettel struct {
	meta  storeMeta
	words roster.Roster[vocab.WordID]
	urls  roster.Roster[string]
}

func makeStore(stats *stats, h sizeHints) *store {
	return &store{
		stats:  stats,
		hint:   h.zettel,
		zettel: make(map[denseZid]*storeZettel, h.zettel),
		words:  make(map[vocab.WordID]roster.Roster[denseZid], h.words),
		urls:   make(map[string]roster.Roster[denseZid], h.urls),
	}
}

// grow dimensions an empty store. It has no effect on a store with content.
func (st *store) grow(n int) {
	st.mx.Lock()
	defer st.mx.Unlock()
	if n > st.hint && len(st.zettel) == 0 {
		st.hint = n
		st.zettel = make(map[denseZid]*storeZettel, n)
	}
}

func (st *store) count() int {
	st.mx.RLock()
	defer st.mx.RUnlock()
	return len(st.zettel)
}

func (st *store) urlCount() int {
	st.mx.RLock()
	defer st.mx.RUnlock()
	return len(st.urls)
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
		addPosting(st.words, word, dzid)
	}
	for u := range newSz.urls.Values() {
		addPosting(st.urls, u, dzid)
	}
}

func (st *store) updateZettel(dzid denseZid, prevSz, newSz *storeZettel) {
	syncPostings(st.words, dzid, prevSz.words, newSz.words)
	syncPostings(st.urls, dzid, prevSz.urls, newSz.urls)
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
		st.deleteZettel(dzid, sz)
		return true
	}
	return false
}

func (st *store) deleteZettel(dzid denseZid, sz *storeZettel) {
	for w := range sz.words.Values() {
		removePosting(st.words, w, dzid)
	}
	for u := range sz.urls.Values() {
		removePosting(st.urls, u, dzid)
	}
}

// ----- posting helpers -----------------------------------------------------

func addPosting[K cmp.Ordered](m map[K]roster.Roster[denseZid], key K, dzid denseZid) {
	r := m[key]
	r.Insert(dzid)
	m[key] = r
}

// removePosting removes dzid from the set under key. An empty set is deleted.
func removePosting[K cmp.Ordered](m map[K]roster.Roster[denseZid], key K, dzid denseZid) {
	if r, found := m[key]; found {
		r.Delete(dzid)
		if r.IsEmpty() {
			delete(m, key)
		} else {
			m[key] = r
		}
	}
}

// syncPostings files dzid under the keys that are new and removes it from the
// keys that are gone.
func syncPostings[K cmp.Ordered](
	m map[K]roster.Roster[denseZid], dzid denseZid, prev, next roster.Roster[K],
) {
	removed, added := prev.Delta(next)
	for k := range removed.Values() {
		removePosting(m, k, dzid)
	}
	for k := range added.Values() {
		addPosting(m, k, dzid)
	}
}
