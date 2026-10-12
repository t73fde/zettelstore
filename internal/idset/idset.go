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

// Package idset implements sets of zettel identifier.
package idset

import (
	"t73f.de/r/zero/roster"
	"t73f.de/r/zsc/domain/id"
)

// ZidSet is a set of zettel identifier, stored as an sorted array.
type ZidSet = roster.Roster[id.Zid]

// RetrievePredicate reports whether the given zettel identifier is part of a
// search result.
type RetrievePredicate func(id.Zid) bool

// SearchResult is a set of zettel identifiers that is the result of an index
// search. It hides the concrete set representation, so that the representation
// can be changed depending on the size of the set.
//
// And and Or modify the receiver. They panic, if the receiver is frozen by a
// call to MakePredicate. A SearchResult is not safe for concurrent use.
type SearchResult struct {
	base   roster.Roster[id.Zid]
	frozen bool
}

// NewSearchResult creates a SearchResult that takes ownership of base.
func NewSearchResult(base roster.Roster[id.Zid]) *SearchResult {
	return &SearchResult{base: base}
}

// IsEmpty reports whether the result contains no identifier.
func (sr *SearchResult) IsEmpty() bool { return sr.base.IsEmpty() }

// Count returns the number of identifiers in the result.
func (sr *SearchResult) Count() int { return sr.base.Count() }

// Clone returns an independent, not frozen copy of the result.
func (sr *SearchResult) Clone() *SearchResult {
	return &SearchResult{base: sr.base.Clone()}
}

// And restricts the result to identifiers that are also contained in other.
// It panics, if the receiver is frozen. The argument other is only read and
// may be frozen.
func (sr *SearchResult) And(other *SearchResult) {
	if sr.frozen {
		panic("search: And on frozen SearchResult")
	}
	sr.base.And(other.base)
}

// Or extends the result by all identifiers contained in other. It panics, if
// the receiver is frozen. The argument other is only read and may be frozen.
func (sr *SearchResult) Or(other *SearchResult) {
	if sr.frozen {
		panic("search: Or on frozen SearchResult")
	}
	sr.base.Or(other.base)
}

// MakePredicate freezes the receiver and returns a predicate for membership
// tests. Subsequent calls to And or Or panic. Use Clone to obtain a modifiable
// copy. Multiple calls are allowed. The predicate is not safe for concurrent
// use.
func (sr *SearchResult) MakePredicate() RetrievePredicate {
	sr.frozen = true
	return sr.base.Contains
}
