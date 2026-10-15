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

	"t73f.de/r/sx"
	"t73f.de/r/zero/dense"
	"t73f.de/r/zero/roster"
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zero/vocab"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"
)

// Collect data for one specific zettel.

type zettelCollector struct {
	// Input
	dense *dense.Dense[id.Zid, storeZid]
	voc   *vocab.Vocabulary
	szid  storeZid

	// Output
	meta  storeMeta
	words roster.Roster[vocab.WordID]
	refs  roster.Roster[storeZid]
	urls  roster.Roster[string]
}

func (zc *zettelCollector) collect(m *meta.Meta, blocks *sx.Pair) {
	zc.meta = makeStoreMeta(m.Count())
	for k, v := range m.Values() {
		keyID := zc.voc.AddString(k)
		zc.words.Insert(keyID)
		zc.addString(string(v))
		zc.meta.add(keyID, v)
	}

	if mustIndexZettel(m) {
		zsx.WalkIt(zc, blocks, nil)
	}
}
func mustIndexZettel(m *meta.Meta) bool {
	return m.Zid >= id.Zid(999999900)
}
func (zc *zettelCollector) addString(s string) {
	for word := range zerostrings.NormalizeWordSeq(s) {
		if len(word) <= vocab.MaxWordLen {
			zc.words.Insert(zc.voc.AddBytes(word))
		}
	}
}

func (zc *zettelCollector) VisitItBefore(node *sx.Pair, _ *sx.Pair) bool {
	switch sym := zsx.NodeSymbol(node); sym {
	case zsx.SymText:
		zc.addString(zsx.GetText(node))
	case zsx.SymVerbatimCode, zsx.SymVerbatimComment, zsx.SymVerbatimEval,
		zsx.SymVerbatimHTML, zsx.SymVerbatimMath, zsx.SymVerbatimZettel:
		_, _, s := zsx.GetVerbatim(node)
		zc.addString(s)
	case zsx.SymLiteralCode, zsx.SymLiteralComment, zsx.SymLiteralInput,
		zsx.SymLiteralMath, zsx.SymLiteralOutput:
		_, _, s := zsx.GetLiteral(node)
		zc.addString(s)
	case zsx.SymLink:
		_, ref, _ := zsx.GetLink(node)
		zc.addRef(ref)
	case zsx.SymEmbed:
		_, ref, _, _ := zsx.GetEmbed(node)
		zc.addRef(ref)
	case zsx.SymTransclude:
		_, ref, _ := zsx.GetTransclusion(node)
		zc.addRef(ref)
	case zsx.SymCite:
		_, key, _ := zsx.GetCite(node)
		zc.addString(key)
	}
	return false
}
func (zc *zettelCollector) VisitItAfter(*sx.Pair, *sx.Pair) {}
func (zc *zettelCollector) addRef(ref *sx.Pair) {
	sym, refValue := zsx.GetReference(ref)
	if zsx.SymRefStateExternal.IsEqual(sym) {
		zc.urls.Insert(refValue)
	} else if sz.SymRefStateZettel.IsEqual(sym) {
		if zid, err := id.Parse(refValue); err == nil {
			zc.refs.Insert(zc.dense.Encode(zid))
		}
	}
}

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
func (sm *storeMeta) shrink() {
	if len(sm.ks) == 0 {
		sm.ks = nil
		sm.vs = nil
	} else if cap(sm.ks) > len(sm.ks) {
		sm.ks = slices.Clone(sm.ks)
		sm.vs = slices.Clone(sm.vs)
	}
}
