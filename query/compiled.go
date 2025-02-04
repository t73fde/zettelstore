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
)

// Compiled is a compiled query, to be used in a Box
type Compiled struct {
	hasQuery bool
	seed     int
	pick     int
	order    []sortOrder
	offset   int // <= 0: no offset
	limit    int // <= 0: no limit

	startMeta []*meta.Meta
	PreMatch  MetaMatchFunc // Precondition for Match and Retrieve
	Terms     []CompiledTerm

	sortFunc sortFunc
}

// MetaMatchFunc is a function determine whethe some metadata should be selected or not.
type MetaMatchFunc func(*meta.Meta) bool

func matchAlways(*meta.Meta) bool { return true }
func matchNever(*meta.Meta) bool  { return false }

// CompiledTerm is the preprocessed sequence of conjugated search terms.
type CompiledTerm struct {
	Match    MetaMatchFunc     // Match on metadata
	Retrieve RetrievePredicate // Retrieve from full-text search
}

// RetrievePredicate returns true, if the given Zid is contained in the (full-text) search.
type RetrievePredicate func(id.Zid) bool

// AlwaysIncluded is a RetrievePredicate that always returns true.
func AlwaysIncluded(id.Zid) bool { return true }
func neverIncluded(id.Zid) bool  { return false }

func (c *Compiled) isDeterministic() bool { return c.seed > 0 }

// Result returns a result of the compiled search, that is achievable without iterating through a box.
func (c *Compiled) Result() []*meta.Meta {
	if len(c.startMeta) == 0 {
		// nil -> no directive
		// empty slice -> nothing found
		return c.startMeta
	}
	result := make([]*meta.Meta, 0, len(c.startMeta))
	for _, m := range c.startMeta {
		for _, term := range c.Terms {
			if term.Match(m) && term.Retrieve(m.Zid) {
				result = append(result, m)
				break
			}
		}
	}
	result = c.pickElements(result)
	c.ensureSortFunc()
	result = c.sortElements(result)
	result = c.offsetElements(result)
	return limitElements(result, c.limit)
}

func (c *Compiled) ensureSortFunc() {
	if c.sortFunc == nil {
		c.sortFunc = buildSortFunc(c.order)
	}
}

// AfterSearch applies all terms to the metadata list that was searched.
//
// This includes sorting, offset, limit, and picking.
func (c *Compiled) AfterSearch(metaList []*meta.Meta) []*meta.Meta {
	if len(metaList) == 0 {
		return metaList
	}

	if !c.hasQuery {
		slices.SortFunc(metaList, defaultMetaSort)
		return metaList
	}

	if c.isDeterministic() {
		// We need to sort to make it deterministic
		if len(c.order) == 0 || c.order[0].isRandom() {
			slices.SortFunc(metaList, defaultMetaSort)
		} else {
			c.ensureSortFunc()
			slices.SortFunc(metaList, c.sortFunc)
		}
	}
	metaList = c.pickElements(metaList)
	if c.isDeterministic() {
		if len(c.order) > 0 && c.order[0].isRandom() {
			metaList = c.sortRandomly(metaList)
		}
	} else {
		metaList = c.sortElements(metaList)
	}
	metaList = c.offsetElements(metaList)
	return limitElements(metaList, c.limit)
}

func (c *Compiled) sortElements(metaList []*meta.Meta) []*meta.Meta {
	if len(c.order) > 0 {
		if c.order[0].isRandom() {
			metaList = c.sortRandomly(metaList)
		} else {
			c.ensureSortFunc()
			slices.SortFunc(metaList, c.sortFunc)
		}
	}
	return metaList
}

func (c *Compiled) offsetElements(metaList []*meta.Meta) []*meta.Meta {
	if c.offset == 0 {
		return metaList
	}
	if c.offset > len(metaList) {
		return nil
	}
	return metaList[c.offset:]
}

func (c *Compiled) pickElements(metaList []*meta.Meta) []*meta.Meta {
	count := c.pick
	if count <= 0 || count >= len(metaList) {
		return metaList
	}
	if limit := c.limit; limit > 0 && limit < count {
		count = limit
		c.limit = 0
	}

	order := make([]int, len(metaList))
	for i := range len(metaList) {
		order[i] = i
	}
	rnd := c.newRandom()
	picked := make([]int, count)
	for i := range count {
		last := len(order) - i
		n := rnd.IntN(last)
		picked[i] = order[n]
		order[n] = order[last-1]
	}
	order = nil
	slices.Sort(picked)
	result := make([]*meta.Meta, count)
	for i, p := range picked {
		result[i] = metaList[p]
	}
	return result
}

func (c *Compiled) sortRandomly(metaList []*meta.Meta) []*meta.Meta {
	rnd := c.newRandom()
	rnd.Shuffle(
		len(metaList),
		func(i, j int) { metaList[i], metaList[j] = metaList[j], metaList[i] },
	)
	return metaList
}

func (c *Compiled) newRandom() *rand.Rand {
	seed := c.seed
	if seed <= 0 {
		seed = rand.IntN(10000) + 10001
	}
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
}

func limitElements(metaList []*meta.Meta, limit int) []*meta.Meta {
	if limit > 0 && limit < len(metaList) {
		return metaList[:limit]
	}
	return metaList
}
