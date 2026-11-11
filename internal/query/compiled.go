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
	return c.pickSortWindow(result)
}

// AfterSearch applies all terms to the metadata list that was searched.
//
// This includes sorting, offset, limit, and picking.
func (c *Compiled) AfterSearch(metaList []*meta.Meta) []*meta.Meta {
	if len(metaList) == 0 {
		return metaList
	}
	return c.pickSortWindow(metaList)
}

// pickSortWindow picks, sorts, and windows (offset, limit) metaList.
// It takes ownership of metaList.
func (c *Compiled) pickSortWindow(metaList []*meta.Meta) []*meta.Meta {
	if c.isDeterministic() {
		// A reproducible input order is needed, so that picking and
		// random sorting with a seed yield the same result every time.
		if len(c.order) == 0 || c.order[0].isRandom() {
			slices.SortFunc(metaList, defaultMetaSort)
		} else {
			c.sort(metaList)
		}
	}

	picking := c.pick > 0 && c.pick < len(metaList)
	if picking {
		// Partial Fisher-Yates shuffle
		rnd := c.newRandom()
		for i := range c.pick {
			j := i + rnd.IntN(len(metaList)-i)
			metaList[i], metaList[j] = metaList[j], metaList[i]
		}
		clear(metaList[c.pick:])
		metaList = metaList[:c.pick:c.pick]
	}

	// Establish the final order. Picking destroyed the previous one.
	switch {
	case len(c.order) > 0 && c.order[0].isRandom():
		rnd := c.newRandom()
		rnd.Shuffle(
			len(metaList),
			func(i, j int) { metaList[i], metaList[j] = metaList[j], metaList[i] },
		)

	case len(c.order) > 0:
		// A deterministic query without picking is already sorted.
		if picking || !c.isDeterministic() {
			c.sort(metaList)
		}

	case picking && c.isDeterministic():
		slices.SortFunc(metaList, defaultMetaSort)
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
