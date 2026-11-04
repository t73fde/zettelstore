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

	"zettelstore.de/z/internal/index"
)

type searchOp struct {
	s  string
	op compareOp
}
type searchFunc func(string) *index.Result
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

func prepareRetrieveCalls(searcher index.Searcher, search []expValue) (normCalls, plainCalls, negCalls searchCallMap) {
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

func retrievePositives(normCalls, plainCalls searchCallMap) *index.Result {
	if isSuperset(normCalls, plainCalls) {
		var normResult *index.Result
		for c, sf := range normCalls {
			as := sf(c.s)
			if normResult == nil {
				normResult = as.Clone()
			} else {
				normResult.And(as)
			}
		}
		return normResult
	}

	cache := make(map[searchOp]*index.Result)

	var plainResult *index.Result
	for c, sf := range plainCalls {
		result := sf(c.s)
		if _, found := normCalls[c]; found {
			cache[c] = result
		}
		if plainResult == nil {
			plainResult = result.Clone()
		} else {
			plainResult.And(result)
		}
	}

	var normResult *index.Result
	for c, sf := range normCalls {
		if result, found := cache[c]; found {
			if normResult == nil {
				normResult = result.Clone()
			} else {
				normResult.And(result)
			}
		} else {
			as := sf(c.s)
			if normResult == nil {
				normResult = as
			} else {
				normResult.And(as)
			}
		}
	}

	switch {
	case normResult == nil:
		return plainResult
	case plainResult == nil:
		return normResult
	default:
		normResult.Or(plainResult)
		return normResult
	}
}

func isSuperset(normCalls, plainCalls searchCallMap) bool {
	for c := range plainCalls {
		if _, found := normCalls[c]; !found {
			return false
		}
	}
	return true
}

func retrieveNegatives(negCalls searchCallMap) (negatives *index.Result) {
	for val, sf := range negCalls {
		as := sf(val.s)
		if negatives == nil {
			negatives = as.Clone()
		} else {
			negatives.Or(as)
		}
	}
	return negatives
}

func getSearchFunc(searcher index.Searcher, op compareOp) searchFunc {
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

func nilSearchFunc(string) *index.Result { return nil }
