//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

package usecase

import (
	"iter"

	"t73f.de/r/sx"
	zeroiter "t73f.de/r/zero/iter"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/collect"
)

// GetReferences is the usecase to retrieve references that occur in a zettel.
type GetReferences struct{}

// NewGetReferences creates a new usecase object.
func NewGetReferences() GetReferences { return GetReferences{} }

// RunByState returns all references of a zettel, sparated by their state:
// local, external, query. No zettel references are returned.
func (uc GetReferences) RunByState(block *sx.Pair) (local, ext, query *sx.Pair) {
	var lbLoc, lbQueries, lbExt sx.ListBuilder
	for ref := range collect.ReferenceSeq(block) {
		sym, _ := zsx.GetReference(ref)
		switch sym {
		case zsx.SymRefStateHosted, sz.SymRefStateBased:
			lbLoc.Add(ref)
		case zsx.SymRefStateExternal:
			lbExt.Add(ref)
		case sz.SymRefStateQuery:
			lbQueries.Add(ref)
		}
	}
	return lbLoc.List(), lbExt.List(), lbQueries.List()
}

// RunByExternal returns an iterator of all external references of a zettel.
func (uc GetReferences) RunByExternal(blocks *sx.Pair) iter.Seq[*sx.Pair] {
	return zeroiter.FilterSeq(
		collect.ReferenceSeq(blocks),
		func(ref *sx.Pair) bool {
			sym, _ := zsx.GetReference(ref)
			return zsx.SymRefStateExternal.IsEqualSymbol(sym)
		})
}

// RunByMeta returns all URLs that are stored in the metadata.
func (uc GetReferences) RunByMeta(m *meta.Meta) iter.Seq[string] {
	return func(yield func(string) bool) {
		for key, val := range m.All() {
			if meta.Type(key) == meta.TypeURL && !yield(string(val)) {
				return
			}
		}
	}
}
