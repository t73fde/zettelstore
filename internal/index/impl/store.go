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

	"t73f.de/r/zero/dense"
	"t73f.de/r/zero/vocab"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/zettel"
)

type store struct {
	stats *stats

	mx    sync.RWMutex
	dense *denser
	voc   *vocab.Vocabulary

	// inverse maps metadata keys to their inverse.
	// Assumes that there will be no future inverse keys.
	inverse map[vocab.WordID]vocab.WordID
}
type storeZid uint32
type denser = dense.Dense[id.Zid, storeZid]

func makeStore(stats *stats, wordsHint int) *store {
	voc := vocab.New(wordsHint)
	inverse := make(map[vocab.WordID]vocab.WordID, 8)
	for kd := range meta.KeyDescriptionSeq() {
		keyID := voc.AddString(kd.Name)
		if invKey := kd.Inverse; invKey != "" {
			invID := voc.AddString(invKey)
			inverse[keyID] = invID
			inverse[invID] = keyID
		}
	}
	result := &store{
		stats:   stats,
		dense:   &denser{},
		voc:     voc,
		inverse: inverse,
	}
	return result
}

func (st *store) updateZettel(ztl *zettel.ParsedZettel, generation uint64) {
	st.mx.Lock()
	defer st.mx.Unlock()

	szid := st.dense.Encode(ztl.Zid)
	zc := zettelCollector{
		dense: st.dense,
		voc:   st.voc,
		szid:  szid,
	}
	zc.collect(ztl.Meta, ztl.Blocks)
	st.stats.finishUpdateZettel(generation)
}

// ----- remove zettel -------------------------------------------------------

func (st *store) removeZettel(zid id.Zid, generation uint64) {
	st.mx.Lock()
	defer st.mx.Unlock()

	st.dense.Remove(zid)
	st.stats.finishRemoveZettel(generation)
}
