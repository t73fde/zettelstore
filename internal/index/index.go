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

// Package index manages the search index.
package index

import (
	"context"

	"t73f.de/r/zero/roster"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/zettel"
)

// Indexer provides methods to work with a search index.
type Indexer interface {
	Index

	// Start the indexer.
	Start()

	// Stop the indexer.
	Stop()
}

// Fetcher declares all methods an Indexer needs to operate.
type Fetcher interface {
	// FetchZids returns the set of all zettel identifer.
	FetchZids(ctx context.Context) (roster.Roster[id.Zid], error)

	// GetZettel retrieves a specific zettel.
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)
}

// Enqueuer allows to operate with the search index.
type Enqueuer interface {
	// Enqueue signals that one or many zettel has been changed and the index
	// should be updated to reflect the changes. Updates zettel data can be
	// retrieved by the given Fetcher.
	// Enqueue is called in the context of a separate goroutine. It must
	// not take too long to process the enqueue signal.
	Enqueue(Fetcher, Reason, id.Zid)
}

// Reason states why `Enqueuer.Enqueue` was called.
type Reason int

// Values for Reason.
const (
	_        Reason = iota
	OnReady         // Index user is ready.
	OnReset         // All zettel should be (re-) indexed.
	OnZettel        // Something with a zettel happened
)

// Index describes the interface to use the search index on both sides:
// An Enqueuer singals zettel changes. Via query.Searcher you can retrieve
// sets of zettel identifier based on a search. A box.Enricher adds some
// metadata to a zettel.
type Index interface {
	Enqueuer
	Searcher

	// GetMeta returns metadata of the given zettel. It signals an error if
	// the zettel is not found or for other retrieval errors.
	GetMeta(zid id.Zid) (*meta.Meta, error)

	// Entrich metadata with data from search index.
	Enrich(context.Context, *meta.Meta)

	// ReadStats populates st with index statistics.
	ReadStats(st *Stats)
}

// Stats records statistics about the box.
type Stats struct {
	NumZettel  int
	NumWords   int
	Vocabulary int

	UpdatedZettel uint64
	RemovedZettel uint64
	NumResets     uint64
}

// Searcher is used to select zettel identifier based on search criteria.
type Searcher interface {
	// Select all zettel that contains the given exact word.
	// The word must be normalized through Unicode NFKD, trimmed and not empty.
	SearchEqual(word string) *SearchResult

	// Select all zettel that have a word with the given prefix.
	// The prefix must be normalized through Unicode NFKD, trimmed and not empty.
	SearchPrefix(prefix string) *SearchResult

	// Select all zettel that have a word with the given suffix.
	// The suffix must be normalized through Unicode NFKD, trimmed and not empty.
	SearchSuffix(suffix string) *SearchResult

	// Select all zettel that contains the given string.
	// The string must be normalized through Unicode NFKD, trimmed and not empty.
	SearchContains(s string) *SearchResult
}

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

// Equal returns true if both sets are identical.
func (sr *SearchResult) Equal(other *SearchResult) bool { return sr.base.Equal(other.base) }

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
