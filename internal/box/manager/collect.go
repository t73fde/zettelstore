//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package manager

import (
	"strings"

	"t73f.de/r/sx"
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/box/manager/store"
)

type collectData struct {
	refs  *idset.Set
	words store.WordSet
	urls  store.WordSet
}

func (data *collectData) initialize() {
	data.refs = idset.New()
	data.words = store.NewWordSet()
	data.urls = store.NewWordSet()
}

func collectZettelIndexData(blocks *sx.Pair, data *collectData) {
	zsx.WalkIt(data, blocks, nil)
}

func (data *collectData) VisitBefore(node *sx.Pair, _ *sx.Pair) bool {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymText:
			data.addText(zsx.GetText(node))
		case zsx.SymVerbatimCode, zsx.SymVerbatimComment, zsx.SymVerbatimEval,
			zsx.SymVerbatimHTML, zsx.SymVerbatimMath, zsx.SymVerbatimZettel:
			_, _, s := zsx.GetVerbatim(node)
			data.addText(s)
		case zsx.SymLiteralCode, zsx.SymLiteralComment, zsx.SymLiteralInput,
			zsx.SymLiteralMath, zsx.SymLiteralOutput:
			_, _, s := zsx.GetLiteral(node)
			data.addText(s)
		case zsx.SymLink:
			_, ref, _ := zsx.GetLink(node)
			data.addRef(ref)
		case zsx.SymEmbed:
			_, ref, _, _ := zsx.GetEmbed(node)
			data.addRef(ref)
		case zsx.SymTransclude:
			_, ref, _ := zsx.GetTransclusion(node)
			data.addRef(ref)
		case zsx.SymCite:
			_, key, _ := zsx.GetCite(node)
			data.addText(key)
		}
	}
	return false
}
func (data *collectData) VisitAfter(*sx.Pair, *sx.Pair) {}

func (data *collectData) addRef(ref *sx.Pair) {
	sym, refValue := zsx.GetReference(ref)
	if zsx.SymRefStateExternal.IsEqual(sym) {
		data.urls.Add(strings.ToLower(refValue))
	} else if sz.SymRefStateZettel.IsEqual(sym) {
		if zid, err := id.Parse(refValue); err == nil {
			data.refs.Add(zid)
		}
	}
}

func (data *collectData) addText(s string) {
	for _, word := range zerostrings.NormalizeWords(s) {
		data.words.Add(word)
	}
}
