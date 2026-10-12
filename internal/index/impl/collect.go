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
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zsx"
)

// Collect data for one specific zettel.

func (st *store) collect(szid storeZid, blocks *sx.Pair) {
	zc := zettelCollector{st, szid}
	zsx.WalkIt(&zc, blocks, nil)
}

type zettelCollector struct {
	store *store
	szid  storeZid
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
		// data.addRef(ref)
		_ = ref
	case zsx.SymEmbed:
		_, ref, _, _ := zsx.GetEmbed(node)
		// data.addRef(ref)
		_ = ref
	case zsx.SymTransclude:
		_, ref, _ := zsx.GetTransclusion(node)
		// data.addRef(ref)
		_ = ref
	case zsx.SymCite:
		_, key, _ := zsx.GetCite(node)
		zc.addText(key)
	}
	return false
}
func (zc *zettelCollector) VisitItAfter(*sx.Pair, *sx.Pair) {}
func (zc *zettelCollector) addText(s string) {
	for word := range zerostrings.NormalizeWordSeq(s) {
		_ = zc.store.voc.AddBytes(word)
	}
}
