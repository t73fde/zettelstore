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
	"time"

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

// Retriever declares all methods an Indexer needs to retrieve some zettel
// related content.
type Retriever interface {
	// GetZettel retrieves a specific zettel.
	GetZettel(context.Context, id.Zid) (zettel.Zettel, error)

	// FetchZids returns the set of all zettel identifer.
	FetchZids(context.Context) (map[id.Zid]struct{}, error)
}

// Enqueuer allows to operate with the search index.
type Enqueuer interface {
	// Enqueue signals that one or many zettel has been changed and the index
	// should be updated to reflect the changes. Updates zettel data can be
	// retrieved by the given Retriever.
	// Enqueue is called in the context of a separate goroutine. It must
	// not take too long to process the enqueue signal.
	Enqueue(Reason, id.Zid)
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

	// RegisterRetriever allows to set a Retriever to be used by the Index.
	RegisterRetriever(Retriever)

	// GetMeta returns metadata of the given zettel. It signals an error if
	// the zettel is not found or for other retrieval errors.
	GetMeta(zid id.Zid) (*meta.Meta, error)

	// Entrich metadata with data from search index.
	Enrich(*meta.Meta)

	// ReadStats populates st with index statistics.
	ReadStats(st *Stats)
}

// Stats records statistics about the indexer.
type Stats struct {
	NumZettel  int
	NumWords   int
	Vocabulary int
	WordCount  map[uint16]int

	UpdatedZettel uint64
	RemovedZettel uint64
	EnrichCount   uint64 // Number of successful metadata enrichments
	EnrichMissed  uint64 // Number of missing zettel for enrichments
	NumRebuild    uint64

	Ready         bool          // initial build is complete
	Building      bool          // a rebuild is in progress
	BuildDuration time.Duration // duration of the last complete build
	BuildFailed   int           // zettel of the last complete build that failed

	DroppedZettel  uint64 // elements dropped due to errors since the start of the active instance
	Failure        string // not empty: the index failed and is unusable
	RebuildFailure string // not empty: the last rebuild failed; the previous index is kept
}

// Searcher is used to select zettel identifier based on search criteria.
type Searcher interface {
	// Select all zettel that contains the given exact word.
	// The word must be normalized through Unicode NFKD, trimmed and not empty.
	SearchEqual(word string) *Result

	// Select all zettel that have a word with the given prefix.
	// The prefix must be normalized through Unicode NFKD, trimmed and not empty.
	SearchPrefix(prefix string) *Result

	// Select all zettel that have a word with the given suffix.
	// The suffix must be normalized through Unicode NFKD, trimmed and not empty.
	SearchSuffix(suffix string) *Result

	// Select all zettel that contains the given string.
	// The string must be normalized through Unicode NFKD, trimmed and not empty.
	SearchContains(s string) *Result
}

// RetrievePredicate reports whether the given zettel identifier is part of a
// search result.
type RetrievePredicate func(id.Zid) bool

// Result is a set of zettel identifiers that is the result of an index
// search. It hides the concrete set representation, so that the representation
// can be changed depending on the size of the set.
//
// A Result is incomplete, if it was computed while the initial build of the
// index was still running. And and Or modify the receiver. They panic, if the
// receiver is frozen by a call to MakePredicate. A Result is not safe for
// concurrent use.
type Result struct {
	base    roster.Roster[id.Zid]
	frozen  bool
	partial bool
}

// NewResult creates a Result that takes ownership of base.
func NewResult(base roster.Roster[id.Zid]) *Result {
	return &Result{base: base}
}

// NewPartialResult creates a Result that takes ownership of base and that is
// marked as possibly incomplete.
func NewPartialResult(base roster.Roster[id.Zid]) *Result {
	return &Result{base: base, partial: true}
}

// Incomplete reports whether the result may miss zettel, because it was
// computed before the initial build of the index was complete.
func (sr *Result) Incomplete() bool { return sr.partial }

// IsEmpty reports whether the result contains no identifier.
func (sr *Result) IsEmpty() bool { return sr.base.IsEmpty() }

// Count returns the number of identifiers in the result.
func (sr *Result) Count() int { return sr.base.Count() }

// Equal returns true if both sets are identical.
func (sr *Result) Equal(other *Result) bool { return sr.base.Equal(other.base) }

// Clone returns an independent, not frozen copy of the result.
func (sr *Result) Clone() *Result {
	return &Result{base: sr.base.Clone(), partial: sr.partial}
}

// And restricts the result to identifiers that are also contained in other.
// It panics, if the receiver is frozen. The argument other is only read and
// may be frozen.
func (sr *Result) And(other *Result) {
	if sr.frozen {
		panic("And operation on frozen Result")
	}
	sr.base.And(other.base)
	sr.partial = sr.partial || other.partial
}

// Or extends the result by all identifiers contained in other. It panics, if
// the receiver is frozen. The argument other is only read and may be frozen.
func (sr *Result) Or(other *Result) {
	if sr.frozen {
		panic("Or operation on frozen Result")
	}
	sr.base.Or(other.base)
	sr.partial = sr.partial || other.partial
}

// MakePredicate freezes the receiver and returns a predicate for membership
// tests. Subsequent calls to And or Or panic. Use Clone to obtain a modifiable
// copy. Multiple calls are allowed. The predicate is not safe for concurrent
// use.
func (sr *Result) MakePredicate() RetrievePredicate {
	sr.frozen = true
	return sr.base.Contains
}
