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

package query

// This file contains helper functions to search within the index.

import (
	"strings"

	zerostrings "t73f.de/r/zero/strings"

	"zettelstore.de/z/internal/idset"
)

type searchOp struct {
	s  string
	op compareOp
}
type searchFunc func(string) idset.ArraySet
type searchCallMap map[searchOp]searchFunc

var cmpPred = map[compareOp]func(string, string) bool{
	cmpEqual:   stringEqual,
	cmpPrefix:  strings.HasPrefix,
	cmpSuffix:  strings.HasSuffix,
	cmpMatch:   strings.Contains,
	cmpHas:     strings.Contains, // the "has" operator have string semantics here in a index search
	cmpLess:    strings.Contains, // in index search there is no "less", only "has"
	cmpGreater: strings.Contains, // in index search there is no "greater", only "has"
}

func (scm searchCallMap) addSearch(s string, op compareOp, sf searchFunc) {
	pred := cmpPred[op]
	for k := range scm {
		if op == cmpMatch {
			if strings.Contains(k.s, s) {
				return
			}
			if strings.Contains(s, k.s) {
				delete(scm, k)
				break
			}
		}
		if k.op != op {
			continue
		}
		if pred(k.s, s) {
			return
		}
		if pred(s, k.s) {
			delete(scm, k)
		}
	}
	scm[searchOp{s: s, op: op}] = sf
}

func prepareRetrieveCalls(searcher Searcher, search []expValue) (normCalls, plainCalls, negCalls searchCallMap) {
	normCalls = make(searchCallMap, len(search))
	negCalls = make(searchCallMap, len(search))
	for _, val := range search {
		for word := range zerostrings.NormalizeWordSeq(string(val.value)) {
			if cmpOp := val.op; cmpOp.isNegated() {
				cmpOp = cmpOp.negate()
				negCalls.addSearch(string(word), cmpOp, getSearchFunc(searcher, cmpOp))
			} else {
				normCalls.addSearch(string(word), cmpOp, getSearchFunc(searcher, cmpOp))
			}
		}
	}

	plainCalls = make(searchCallMap, len(search))
	for _, val := range search {
		word := val.value.TrimSpace().ToLower()
		if cmpOp := val.op; cmpOp.isNegated() {
			cmpOp = cmpOp.negate()
			negCalls.addSearch(string(word), cmpOp, getSearchFunc(searcher, cmpOp))
		} else {
			plainCalls.addSearch(string(word), cmpOp, getSearchFunc(searcher, cmpOp))
		}
	}
	return normCalls, plainCalls, negCalls
}

func hasConflictingCalls(normCalls, plainCalls, negCalls searchCallMap) bool {
	for val := range negCalls {
		if _, found := normCalls[val]; found {
			return true
		}
		if _, found := plainCalls[val]; found {
			return true
		}
	}
	return false
}

func retrievePositives(normCalls, plainCalls searchCallMap) *idset.ArraySet {
	if isSuperset(normCalls, plainCalls) {
		var normResult idset.ArraySet
		first := true
		for c, sf := range normCalls {
			as := sf(c.s)
			if first {
				normResult = as.Clone()
				first = false
			} else {
				normResult.Intersection(as)
			}
		}
		return &normResult
	}

	cache := make(map[searchOp]*idset.ArraySet)

	var plainResult idset.ArraySet
	first := true
	for c, sf := range plainCalls {
		result := sf(c.s)
		if _, found := normCalls[c]; found {
			cache[c] = &result
		}
		if first {
			plainResult = result.Clone()
			first = false
		} else {
			plainResult.Intersection(result)
		}
	}

	var normResult *idset.ArraySet
	first = true
	for c, sf := range normCalls {
		if result, found := cache[c]; found {
			if first {
				tmp := result.Clone()
				normResult = &tmp
				first = false
			} else {
				normResult.Intersection(*result)
			}
		} else {
			as := sf(c.s)
			if first {
				tmp := as.Clone()
				normResult = &tmp
				first = false
			} else {
				normResult.Intersection(as)
			}
		}
	}
	return normResult.IUnion(&plainResult)
}

func isSuperset(normCalls, plainCalls searchCallMap) bool {
	for c := range plainCalls {
		if _, found := normCalls[c]; !found {
			return false
		}
	}
	return true
}

func retrieveNegatives(negCalls searchCallMap) *idset.ArraySet {
	var negatives *idset.ArraySet
	for val, sf := range negCalls {
		as := sf(val.s)
		negatives = negatives.IUnion(&as)
	}
	return negatives
}

func getSearchFunc(searcher Searcher, op compareOp) searchFunc {
	switch op {
	case cmpEqual:
		return searcher.SearchEqual
	case cmpPrefix:
		return searcher.SearchPrefix
	case cmpSuffix:
		return searcher.SearchSuffix
	case cmpMatch, cmpHas, cmpLess, cmpGreater: // for index search we assume string semantics
		return searcher.SearchContains
	default:
		return nilSearchFunc
	}
}

func nilSearchFunc(string) idset.ArraySet { return idset.ArraySet{} }
