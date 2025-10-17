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

package evaluator

import (
	"iter"

	"t73f.de/r/sx"
	zeroiter "t73f.de/r/zero/iter"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"
)

func evaluateMetadata(m *meta.Meta) *sx.Pair {
	var lb sx.ListBuilder
	lb.AddN(zsx.SymDescription, nil)
	for key, val := range m.All() {
		lb.Add(sx.MakeList(zsx.MakeText(key)))
		inlines := convertMetavalueToInlineSlice(val, meta.Type(key))
		lb.Add(zsx.MakeBlock(zsx.MakeBlock(zsx.MakeParaList(inlines))))
	}
	return zsx.MakeBlock(lb.List())
}

func convertMetavalueToInlineSlice(val meta.Value, dt *meta.DescriptionType) *sx.Pair {
	var vals iter.Seq[string]
	if dt.IsSet {
		vals = val.Fields()
	} else {
		vals = zeroiter.OneSeq(string(val))
	}
	makeLink := dt == meta.TypeID || dt == meta.TypeIDSet

	var lb sx.ListBuilder
	first := true
	for s := range vals {
		if first {
			first = false
		} else {
			lb.Add(zsx.MakeText(" "))
		}
		tn := zsx.MakeText(s)
		if makeLink {
			lb.Add(zsx.MakeLink(nil, sz.ScanReference(s), sx.MakeList(tn)))
		} else {
			lb.Add(tn)
		}
	}

	return lb.List()
}
