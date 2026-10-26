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

	mx       sync.RWMutex
	zettel   map[denseZid]*storeZettel                // all existing zettel
	backward map[denseZid]roster.Roster[denseZid]     // target -> zettel that reference it; the target may not exist
	words    map[vocab.WordID]roster.Roster[denseZid] // word -> zettel containing it
	urls     map[string]roster.Roster[denseZid]       // URL -> zettel containing it
}

// storeZettel stores data of a specific zettel. After adding it to a store,
// it is treated as immutable.
type storeZettel struct {
	meta        storeMeta
	forward     roster.Roster[denseZid]
	words       roster.Roster[vocab.WordID]
	urls        roster.Roster[string]
	metaForward []metaZid
}

func makeStore(stats *stats, h sizeHints) *store {
	return &store{
		stats:    stats,
		zettel:   make(map[denseZid]*storeZettel, h.zettel),
		backward: make(map[denseZid]roster.Roster[denseZid], h.zettel),
		words:    make(map[vocab.WordID]roster.Roster[denseZid], h.words),
		urls:     make(map[string]roster.Roster[denseZid], h.urls),
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

func (st *store) getZettel(dzid denseZid) *storeZettel {
	st.mx.RLock()
	defer st.mx.RUnlock()
	if sz, found := st.zettel[dzid]; found {
		return sz
	}
	return nil
}

// ----- store modification --------------------------------------------------

// upsertZettel takes ownership of az.
func (st *store) upsertZettel(dzid denseZid, az *analyzedZettel) {
	st.replace(dzid, az)
	st.stats.finishUpdateZettel()
}

// replace upserts analyzed zettel data into the store.
//
// Invariant: for every zettel r in st.zettel, t is in r.forward if and only if
// r is in st.backward[t]. Self-references are kept.
func (st *store) replace(dzid denseZid, az *analyzedZettel) {
	st.mx.Lock()
	defer st.mx.Unlock()

	newSz := (*storeZettel)(az) // az is owned by the store now
	if prevSz, found := st.zettel[dzid]; found {
		st.updateZettel(dzid, prevSz, newSz)
	} else {
		st.insertZettel(dzid, newSz)
	}
	st.zettel[dzid] = newSz
}

func (st *store) insertZettel(dzid denseZid, newSz *storeZettel) {
	for t := range newSz.forward.Values() {
		addPosting(st.backward, t, dzid)
	}
	for word := range newSz.words.Values() {
		addPosting(st.words, word, dzid)
	}
	for u := range newSz.urls.Values() {
		addPosting(st.urls, u, dzid)
	}
}

func (st *store) updateZettel(dzid denseZid, prevSz, newSz *storeZettel) {
	syncPostings(st.backward, dzid, prevSz.forward, newSz.forward)
	syncPostings(st.words, dzid, prevSz.words, newSz.words)
	syncPostings(st.urls, dzid, prevSz.urls, newSz.urls)
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
	for t := range sz.forward.Values() {
		removePosting(st.backward, t, dzid)
	}
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

// ----- queries -------------------------------------------------------------

// matchEqual returns the dense identifier of all zettel that match word
// exactly. All data is read under one lock and therefore come from the same
// state of the store. The result is a new set, owned by the caller.
func (st *store) matchEqual(word string, wid vocab.WordID, hasZid bool, dzid denseZid) roster.Roster[denseZid] {
	st.mx.RLock()
	defer st.mx.RUnlock()

	// Postings are modified in place, therefore they must be copied.
	var result roster.Roster[denseZid]
	if wid != vocab.InvalidWordID {
		result = st.words[wid].Clone()
	}
	if r, found := st.urls[word]; found {
		result.Or(r)
	}
	if hasZid {
		// The dense identifier may exist without a zettel (e.g. as a reference
		// target). Existence is decided by the store.
		if st.zettel[dzid] != nil {
			result.Insert(dzid)
		}

		// Add zettel with references of the queries zettel.
		// Include dead backwards also, because they should be found.
		result.Or(st.backward[dzid])
	}
	return result
}
