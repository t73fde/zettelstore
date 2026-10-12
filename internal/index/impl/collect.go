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

type storeMeta map[vocab.WordID]meta.Value

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
	zc.meta = make(storeMeta)
	for k, v := range m.All() {
		keyId := zc.voc.AddString(k)
		zc.words.Insert(keyId)
		for word := range zerostrings.NormalizeWordSeq(string(v)) {
			zc.words.Insert(zc.voc.AddBytes(word))
		}
		zc.meta[keyId] = v
	}

	if mustIndexZettel(m) {
		zsx.WalkIt(zc, blocks, nil)
	}
}
func mustIndexZettel(m *meta.Meta) bool {
	return m.Zid >= id.Zid(999999900)
}

func (zc *zettelCollector) VisitItBefore(node *sx.Pair, _ *sx.Pair) bool {
	switch sym := zsx.NodeSymbol(node); sym {
	case zsx.SymText:
		zc.addText(zsx.GetText(node))
	case zsx.SymVerbatimCode, zsx.SymVerbatimComment, zsx.SymVerbatimEval,
		zsx.SymVerbatimHTML, zsx.SymVerbatimMath, zsx.SymVerbatimZettel:
		_, _, s := zsx.GetVerbatim(node)
		zc.addText(s)
	case zsx.SymLiteralCode, zsx.SymLiteralComment, zsx.SymLiteralInput,
		zsx.SymLiteralMath, zsx.SymLiteralOutput:
		_, _, s := zsx.GetLiteral(node)
		zc.addText(s)
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
		zc.addText(key)
	}
	return false
}
func (zc *zettelCollector) VisitItAfter(*sx.Pair, *sx.Pair) {}
func (zc *zettelCollector) addText(s string) {
	for word := range zerostrings.NormalizeWordSeq(s) {
		zc.words.Insert(zc.voc.AddBytes(word))
	}
}
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
