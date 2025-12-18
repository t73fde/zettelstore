//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

// Package query provides a query for zettel.
package query

import (
	"context"
	"maps"
	"math/rand/v2"
	"slices"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"
)

// Searcher is used to select zettel identifier based on search criteria.
type Searcher interface {
	// Select all zettel that contains the given exact word.
	// The word must be normalized through Unicode NKFD, trimmed and not empty.
	SearchEqual(word string) *idset.Set

	// Select all zettel that have a word with the given prefix.
	// The prefix must be normalized through Unicode NKFD, trimmed and not empty.
	SearchPrefix(prefix string) *idset.Set

	// Select all zettel that have a word with the given suffix.
	// The suffix must be normalized through Unicode NKFD, trimmed and not empty.
	SearchSuffix(suffix string) *idset.Set

	// Select all zettel that contains the given string.
	// The string must be normalized through Unicode NKFD, trimmed and not empty.
	SearchContains(s string) *idset.Set
}

// Query specifies a mechanism for querying zettel.
type Query struct {
	// Präfixed zettel identifier.
	zids []id.Zid

	// Querydirectives, like CONTEXT, ...
	directives []Directive

	// Fields to be used for selecting
	preMatch MetaMatchFunc // Match that must be true
	terms    []conjTerms

	// Create predictable randomness
	seed int

	pick int // Randomly pick elements, <= 0: no pick

	// Fields to be used for sorting
	order  []sortOrder
	offset int // <= 0: no offset
	limit  int // <= 0: no limit

	// Execute specification
	actions []string
}

// GetZids returns a slide of all specified zettel identifier.
func (q *Query) GetZids() []id.Zid {
	if q == nil || len(q.zids) == 0 {
		return nil
	}
	return slices.Clone(q.zids)
}

// Directive are executed to process the list of metadata.
type Directive interface {
	Print(*PrintEnv)
}

// GetDirectives returns the slice of query directives.
func (q *Query) GetDirectives() []Directive {
	if q == nil || len(q.directives) == 0 {
		return nil
	}
	result := make([]Directive, len(q.directives))
	copy(result, q.directives)
	return result
}

type keyExistMap map[string]compareOp
type expMetaValues map[string][]expValue

type conjTerms struct {
	keys   keyExistMap
	mvals  expMetaValues // Expected values for a meta datum
	search []expValue    // Search string
}

func (ct *conjTerms) isEmpty() bool {
	return len(ct.keys) == 0 && len(ct.mvals) == 0 && len(ct.search) == 0
}
func (ct *conjTerms) addKey(key string, op compareOp) {
	if ct.keys == nil {
		ct.keys = map[string]compareOp{key: op}
		return
	}
	if prevOp, found := ct.keys[key]; found {
		if prevOp != op {
			ct.keys[key] = cmpUnknown
		}
		return
	}
	ct.keys[key] = op
}
func (ct *conjTerms) addSearch(val expValue) { ct.search = append(ct.search, val) }

type sortOrder struct {
	key        string
	descending bool
}

func (so *sortOrder) isRandom() bool { return so.key == "" }

func createIfNeeded(q *Query) *Query {
	if q == nil {
		return &Query{
			terms: []conjTerms{{}},
		}
	}
	return q
}

// Clone the query value.
func (q *Query) Clone() *Query {
	if q == nil {
		return nil
	}
	c := new(Query)
	c.zids = q.GetZids()
	c.directives = q.GetDirectives()

	c.preMatch = q.preMatch
	c.terms = make([]conjTerms, len(q.terms))
	for i, term := range q.terms {
		c.terms[i].keys = maps.Clone(term.keys)
		c.terms[i].mvals = maps.Clone(term.mvals)
		c.terms[i].search = slices.Clone(term.search)
	}
	c.seed = q.seed
	c.pick = q.pick
	c.order = slices.Clone(q.order)
	c.offset = q.offset
	c.limit = q.limit
	c.actions = q.actions
	return c
}

type compareOp uint8

const (
	cmpUnknown compareOp = iota
	cmpExist
	cmpNotExist
	cmpEqual
	cmpNotEqual
	cmpHas
	cmpHasNot
	cmpPrefix
	cmpNoPrefix
	cmpSuffix
	cmpNoSuffix
	cmpMatch
	cmpNoMatch
	cmpLess
	cmpNoLess
	cmpGreater
	cmpNoGreater
)

var negateMap = map[compareOp]compareOp{
	cmpUnknown:   cmpUnknown,
	cmpExist:     cmpNotExist,
	cmpEqual:     cmpNotEqual,
	cmpNotEqual:  cmpEqual,
	cmpHas:       cmpHasNot,
	cmpHasNot:    cmpHas,
	cmpPrefix:    cmpNoPrefix,
	cmpNoPrefix:  cmpPrefix,
	cmpSuffix:    cmpNoSuffix,
	cmpNoSuffix:  cmpSuffix,
	cmpMatch:     cmpNoMatch,
	cmpNoMatch:   cmpMatch,
	cmpLess:      cmpNoLess,
	cmpNoLess:    cmpLess,
	cmpGreater:   cmpNoGreater,
	cmpNoGreater: cmpGreater,
}

func (op compareOp) negate() compareOp { return negateMap[op] }

var negativeMap = map[compareOp]bool{
	cmpNotExist:  true,
	cmpNotEqual:  true,
	cmpHasNot:    true,
	cmpNoPrefix:  true,
	cmpNoSuffix:  true,
	cmpNoMatch:   true,
	cmpNoLess:    true,
	cmpNoGreater: true,
}

func (op compareOp) isNegated() bool { return negativeMap[op] }

type expValue struct {
	value meta.Value
	op    compareOp
}

func (q *Query) addSearch(val expValue) { q.terms[len(q.terms)-1].addSearch(val) }

func (q *Query) addKey(key string, op compareOp) *Query {
	q = createIfNeeded(q)
	q.terms[len(q.terms)-1].addKey(key, op)
	return q
}

var missingMap = map[compareOp]bool{
	cmpNotExist: true,
	cmpNotEqual: true,
	cmpHasNot:   true,
	cmpNoMatch:  true,
}

// GetMetaValues returns the slice of all values specified for a given metadata key.
// If `withMissing` is true, all values are returned. Otherwise only those,
// where the comparison operator will positively search for a value.
func (q *Query) GetMetaValues(key string, withMissing bool) (vals []meta.Value) {
	if q == nil {
		return nil
	}
	for _, term := range q.terms {
		if mvs, hasMv := term.mvals[key]; hasMv {
			for _, ev := range mvs {
				if withMissing || !missingMap[ev.op] {
					vals = append(vals, ev.value)
				}
			}
		}
	}
	slices.Sort(vals)
	return slices.Compact(vals)
}

// SetPreMatch sets the pre-selection predicate.
func (q *Query) SetPreMatch(preMatch MetaMatchFunc) *Query {
	q = createIfNeeded(q)
	if q.preMatch != nil {
		panic("search PreMatch already set")
	}
	q.preMatch = preMatch
	return q
}

// SetSeed sets a seed value.
func (q *Query) SetSeed(seed int) *Query {
	q = createIfNeeded(q)
	q.seed = seed
	return q
}

// GetSeed returns the seed value if one was set.
func (q *Query) GetSeed() (int, bool) {
	if q == nil {
		return 0, false
	}
	return q.seed, q.seed > 0
}

// SetDeterministic signals that the result should be the same if the seed is the same.
func (q *Query) SetDeterministic() *Query {
	q = createIfNeeded(q)
	if q.seed <= 0 {
		q.seed = int(rand.IntN(10000) + 1)
	}
	return q
}

// Actions returns the slice of action specifications
func (q *Query) Actions() []string {
	if q == nil {
		return nil
	}
	return q.actions
}

// RemoveActions will remove the action part of a query.
func (q *Query) RemoveActions() {
	if q != nil {
		q.actions = nil
	}
}

// EnrichNeeded returns true, if the query references a metadata key that
// is calculated via metadata enrichments.
func (q *Query) EnrichNeeded() bool {
	if q == nil {
		return false
	}
	if len(q.zids) > 0 {
		return true
	}
	if len(q.actions) > 0 {
		// Unknown, what an action may use. For examples: KEYS action uses all metadata.
		return true
	}
	for _, term := range q.terms {
		for key := range term.keys {
			if meta.IsProperty(key) {
				return true
			}
		}
		for key := range term.mvals {
			if meta.IsProperty(key) {
				return true
			}
		}
	}
	for _, o := range q.order {
		if meta.IsProperty(o.key) {
			return true
		}
	}
	return false
}

// RetrieveAndCompile queries the search index and returns a predicate
// for its results and returns a matching predicate.
func (q *Query) RetrieveAndCompile(_ context.Context, searcher Searcher, metaSeq []*meta.Meta) Compiled {
	if q == nil {
		return Compiled{
			PreMatch: matchAlways,
			Terms: []CompiledTerm{{
				Match:    matchAlways,
				Retrieve: AlwaysIncluded,
			}}}
	}
	q = q.Clone()

	preMatch := q.preMatch
	if preMatch == nil {
		preMatch = matchAlways
	}

	startSet := metaList2idSet(metaSeq)
	result := Compiled{
		hasQuery:  true,
		seed:      q.seed,
		pick:      q.pick,
		order:     q.order,
		offset:    q.offset,
		limit:     q.limit,
		startMeta: metaSeq,
		PreMatch:  preMatch,
		Terms:     []CompiledTerm{},
	}

	for _, term := range q.terms {
		cTerm := term.retrieveAndCompileTerm(searcher, startSet)
		if cTerm.Retrieve == nil {
			if cTerm.Match == nil {
				// no restriction on match/retrieve -> all will match
				result.Terms = []CompiledTerm{{
					Match:    matchAlways,
					Retrieve: AlwaysIncluded,
				}}
				break
			}
			cTerm.Retrieve = AlwaysIncluded
		}
		if cTerm.Match == nil {
			cTerm.Match = matchAlways
		}
		result.Terms = append(result.Terms, cTerm)
	}
	return result
}

func metaList2idSet(ml []*meta.Meta) *idset.Set {
	if ml == nil {
		return nil
	}
	result := idset.NewCap(len(ml))
	for _, m := range ml {
		result = result.Add(m.Zid)
	}
	return result
}

func (ct *conjTerms) retrieveAndCompileTerm(searcher Searcher, startSet *idset.Set) CompiledTerm {
	match := ct.compileMeta() // Match might add some searches
	var pred RetrievePredicate
	if searcher != nil {
		pred = ct.retrieveIndex(searcher)
		if startSet != nil {
			if pred == nil {
				pred = startSet.ContainsOrNil
			} else {
				predSet := idset.NewCap(startSet.Length())
				startSet.ForEach(func(zid id.Zid) {
					if pred(zid) {
						predSet = predSet.Add(zid)
					}
				})
				pred = predSet.ContainsOrNil
			}
		}
	}
	return CompiledTerm{Match: match, Retrieve: pred}
}

// retrieveIndex and return a predicate to ask for results.
func (ct *conjTerms) retrieveIndex(searcher Searcher) RetrievePredicate {
	if len(ct.search) == 0 {
		return nil
	}
	normCalls, plainCalls, negCalls := prepareRetrieveCalls(searcher, ct.search)
	if hasConflictingCalls(normCalls, plainCalls, negCalls) {
		return neverIncluded
	}

	positives := retrievePositives(normCalls, plainCalls)
	if positives == nil {
		// No positive search for words, must contain only words for a negative search.
		// Otherwise len(search) == 0 (see above)
		negatives := retrieveNegatives(negCalls)
		return func(zid id.Zid) bool { return !negatives.ContainsOrNil(zid) }
	}
	if positives.IsEmpty() {
		// Positive search didn't found anything. We can omit the negative search.
		return neverIncluded
	}
	if len(negCalls) == 0 {
		// Positive search found something, but there is no negative search.
		return positives.Contains
	}
	negatives := retrieveNegatives(negCalls)
	if negatives == nil {
		return positives.Contains
	}
	return func(zid id.Zid) bool {
		return positives.Contains(zid) && !negatives.ContainsOrNil(zid)
	}
}

// Limit returns only s.GetLimit() elements of the given list.
func (q *Query) Limit(metaList []*meta.Meta) []*meta.Meta {
	if q == nil {
		return metaList
	}
	return limitElements(metaList, q.limit)
}
