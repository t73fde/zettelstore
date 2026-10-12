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
	"math"
	"sync"

	"t73f.de/r/zero/vocab"
	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/zettel"
)

type store struct {
	stats *stats

	mx     sync.RWMutex
	zidNum map[id.Zid]storeZid
	numZid []id.Zid
	voc    *vocab.Vocabulary
}
type storeZid uint32

func makeStore(stats *stats) *store {
	result := &store{
		stats:  stats,
		zidNum: make(map[id.Zid]storeZid),
		voc:    vocab.New(0),
	}
	return result
}

func (st *store) updateZettel(ztl *zettel.ParsedZettel, generation uint64) {
	defer func() {
		st.stats.finishUpdateZettel(generation)
	}()
	st.mx.Lock()
	defer st.mx.Unlock()
	szid := st.nextStoreZid(ztl.Zid)
	st.collect(szid, ztl.Blocks)
}

func (st *store) nextStoreZid(zid id.Zid) storeZid {
	if result, found := st.zidNum[zid]; found {
		_ = st.numZid[result] // For testing: assert that other direction is filled
		return result
	}
	l := len(st.numZid)
	if l >= math.MaxUint32 {
		panic("no more storeZid")
	}
	result := storeZid(len(st.numZid))
	st.numZid = append(st.numZid, zid)
	st.zidNum[zid] = result
	return result
}

// ----- remove zettel -------------------------------------------------------

func (st *store) removeZettel(_ id.Zid, generation uint64) {
	defer func() {
		st.stats.finishRemoveZettel(generation)
	}()
	st.mx.Lock()
	defer st.mx.Unlock()
}
