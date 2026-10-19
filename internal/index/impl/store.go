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

type store struct {
	stats *stats
	dict  *dictionary
}

func makeStore(stats *stats, dict *dictionary) *store {
	return &store{
		stats: stats,
		dict:  dict,
	}
}

func (st *store) updateZettel(_ *analyzedZettel, generation uint64) {
	st.stats.finishUpdateZettel(generation)
}

// ----- remove zettel -------------------------------------------------------

func (st *store) removeZettel(dzid denseZid, generation uint64) {
	// TODO: remove zettel from main store

	// Remove denseZid from dictionary
	dict := st.dict
	dict.Lock()
	defer dict.Unlock()
	dict.RemoveValue(dzid)

	st.stats.finishRemoveZettel(generation)
}
