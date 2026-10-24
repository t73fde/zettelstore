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
	"sort"

	zeroslices "t73f.de/r/zero/slices"
	"t73f.de/r/zero/vocab"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

// storeMeta is a compact representation of a meta map. The two separate
// slices are to avoid padding. storeMeta does not need to be a map.
type storeMeta struct {
	ks []vocab.WordID
	vs []meta.Value
}

func makeStoreMeta(sizeHint int) storeMeta {
	return storeMeta{
		ks: make([]vocab.WordID, 0, sizeHint),
		vs: make([]meta.Value, 0, sizeHint),
	}
}
func (sm *storeMeta) add(key vocab.WordID, val meta.Value) {
	sm.ks = append(sm.ks, key)
	sm.vs = append(sm.vs, val)
}
func (sm *storeMeta) sort() {
	sort.Sort(sm)
}
func (sm *storeMeta) shrink() {
	sm.ks = zeroslices.Shrink(sm.ks)
	sm.vs = zeroslices.Shrink(sm.vs)
}

func (sm storeMeta) toMeta(zid id.Zid, dict *dictionary) *meta.Meta {
	m := make(map[string]string, len(sm.ks))
	for i, k := range sm.ks {
		m[dict.GetString(k)] = string(sm.vs[i])
	}
	return meta.NewWithData(zid, m)
}

// ----- helpers, to make metaStore sortable.

func (sm storeMeta) Len() int { return len(sm.ks) }

func (sm storeMeta) Less(i, j int) bool {
	return sm.ks[i] < sm.ks[j]
}

func (sm storeMeta) Swap(i, j int) {
	sm.ks[i], sm.ks[j] = sm.ks[j], sm.ks[i]
	sm.vs[i], sm.vs[j] = sm.vs[j], sm.vs[i]
}
