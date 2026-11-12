//-----------------------------------------------------------------------------
// Copyright (c) 2023-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2023-present Detlef Stern
//-----------------------------------------------------------------------------

package query

import (
	"math/rand/v2"
	"slices"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/internal/index"
)

// Compiled is a compiled query, to be used in a box.
type Compiled struct {
	seed   int
	pick   int
	order  []sortOrder
	offset int // <= 0: no offset
	limit  int // <= 0: no limit

	startMeta []*meta.Meta
	PreMatch  MetaMatchFunc // Precondition for Match and Retrieve
	Terms     []CompiledTerm

	sortFunc sortFunc
}

// AlwaysIncluded is a RetrievePredicate that always returns true.
func AlwaysIncluded(id.Zid) bool { return true }
func neverIncluded(id.Zid) bool  { return false }

// MetaMatchFunc is a function determine whether some metadata should be
// selected or not.
type MetaMatchFunc func(*meta.Meta) bool

func matchAlways(*meta.Meta) bool { return true }
func matchNever(*meta.Meta) bool  { return false }

// CompiledTerm is the preprocessed sequence of conjugated search terms.
type CompiledTerm struct {
	Retrieve index.RetrievePredicate // Retrieve from full-text search
	Match    MetaMatchFunc           // Match on metadata
}

func (c *Compiled) isDeterministic() bool { return c.seed > 0 }

// Result returns a result of the compiled search, that is achievable without
// iterating through boxes.
func (c *Compiled) Result() []*meta.Meta {
	if len(c.startMeta) == 0 {
		// nil -> no directive
		// empty slice -> nothing found
		return c.startMeta
	}
	result := make([]*meta.Meta, 0, len(c.startMeta))
	for _, m := range c.startMeta {
		for _, term := range c.Terms {
			if c.PreMatch(m) && term.Retrieve(m.Zid) && term.Match(m) {
				result = append(result, m)
				break
			}
		}
	}
	return c.pickSortWindow(result, true)
}

// AfterSearch applies all terms to the metadata list that was searched.
//
// This includes sorting, offset, limit, and picking.
func (c *Compiled) AfterSearch(metaList []*meta.Meta) []*meta.Meta {
	if len(metaList) == 0 {
		return metaList
	}
	return c.pickSortWindow(metaList, false)
}

// pickSortWindow picks, sorts, and windows (offset, limit) metaList.
// It takes ownership of metaList.
//
// If alreadySorted is true, metaList is already sorted (in an order defined
// by the caller). That order is kept, unless the query specifies an explicit
// order: then the result is sorted according to it. Picking keeps the
// relative order of the picked elements.
//
// If alreadySorted is false, the order of metaList is arbitrary. Then a
// deterministic query sorts it first, so that picking and random ordering
// with a seed yield the same result every time.
func (c *Compiled) pickSortWindow(metaList []*meta.Meta, alreadySorted bool) []*meta.Meta {
	hasOrder := len(c.order) > 0
	randomOrder := hasOrder && c.order[0].isRandom()
	explicitOrder := hasOrder && !randomOrder
	picking := c.pick > 0 && c.pick < len(metaList)

	// Establish a reproducible input order.
	sortedByOrder := false
	if !alreadySorted && c.isDeterministic() {
		if explicitOrder && !picking {
			// Nothing is picked, so this is already the final order.
			c.sort(metaList)
			sortedByOrder = true
		} else {
			// Without an explicit order, this is also the final order.
			slices.SortFunc(metaList, defaultMetaSort)
		}
	}

	if picking {
		metaList = pickInOrder(metaList, c.pick, c.newRandom())
	}

	// Establish the final order, if the query requires one.
	switch {
	case randomOrder:
		rnd := c.newRandom()
		rnd.Shuffle(
			len(metaList),
			func(i, j int) { metaList[i], metaList[j] = metaList[j], metaList[i] },
		)
	case explicitOrder && !sortedByOrder:
		c.sort(metaList)
	}

	// Handle offset / limit
	start := min(max(c.offset, 0), len(metaList))
	end := len(metaList)
	if c.limit > 0 && c.limit < end-start {
		end = start + c.limit
	}
	clear(metaList[:start])
	clear(metaList[end:])
	return metaList[start:end:end]
}

// pickInOrder randomly picks k elements of metaList and returns them in their
// original relative order (selection sampling, Knuth's Algorithm S). Every
// subset of size k is equally likely. The list is modified in place; it
// requires 0 < k <= len(metaList).
func pickInOrder(metaList []*meta.Meta, k int, rnd interface{ IntN(int) int }) []*meta.Meta {
	n := len(metaList)
	w := 0 // number of picked elements, also the write position
	for i, m := range metaList {
		if rnd.IntN(n-i) < k-w {
			metaList[w] = m
			w++
			if w == k {
				break
			}
		}
	}
	clear(metaList[w:])
	return metaList[:w:w]
}

func (c *Compiled) sort(metaList []*meta.Meta) {
	if c.sortFunc == nil {
		c.sortFunc = buildSortFunc(c.order)
	}
	slices.SortFunc(metaList, c.sortFunc)
}

func (c *Compiled) newRandom() *rand.Rand {
	seed := c.seed
	if seed <= 0 {
		seed = rand.IntN(10000) + 10001
	}
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
}
