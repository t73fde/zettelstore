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
)

type store struct {
	stats *stats

	mx     sync.RWMutex
	zettel map[denseZid]*analyzedZettel // all existing zettel
}

func makeStore(stats *stats, zettelHint int) *store {
	return &store{
		stats:  stats,
		zettel: make(map[denseZid]*analyzedZettel, zettelHint),
	}
}

// grow dimensions an empty store. It has no effect on a store with content.
func (st *store) grow(n int) {
	st.mx.Lock()
	defer st.mx.Unlock()
	if len(st.zettel) == 0 && n > 0 {
		st.zettel = make(map[denseZid]*analyzedZettel, n)
	}
}

func (st *store) count() int {
	st.mx.RLock()
	defer st.mx.RUnlock()
	return len(st.zettel)
}

func (st *store) exists(dzid denseZid) bool {
	st.mx.RLock()
	defer st.mx.RUnlock()
	_, found := st.zettel[dzid]
	return found
}

// upsertZettel takes ownership of az.
func (st *store) upsertZettel(dzid denseZid, az *analyzedZettel) {
	st.replace(dzid, az)
	st.stats.finishUpdateZettel()
}

func (st *store) replace(dzid denseZid, az *analyzedZettel) {
	st.mx.Lock()
	defer st.mx.Unlock()

	// TODO: remove postings of the old version, add postings of az.
	st.zettel[dzid] = az
}

func (st *store) removeZettel(dzid denseZid) {
	st.drop(dzid)
	st.stats.finishRemoveZettel()
}

func (st *store) drop(dzid denseZid) {
	st.mx.Lock()
	defer st.mx.Unlock()
	// TODO: remove postings of st.zettel[dzid].
	delete(st.zettel, dzid)
}
