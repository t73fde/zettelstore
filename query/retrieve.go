//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package query

// This file contains helper functions to search within the index.

import (
	"fmt"
	"strings"

	"zettelstore.de/z/strfun"
	"zettelstore.de/z/zettel/id"
)

type searchOp struct {
	s  string
	op compareOp
}
type searchFunc func(string) id.Set
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

func alwaysIncluded(id.Zid) bool { return true }
func neverIncluded(id.Zid) bool  { return false }

func prepareRetrieveCalls(searcher Searcher, search []expValue) (normCalls, plainCalls, negCalls searchCallMap) {
	normCalls = make(searchCallMap, len(search))
	negCalls = make(searchCallMap, len(search))
	for _, val := range search {
		for _, word := range strfun.NormalizeWords(val.value) {
			if cmpOp := val.op; cmpOp.isNegated() {
				cmpOp = cmpOp.negate()
				negCalls.addSearch(word, cmpOp, getSearchFunc(searcher, cmpOp))
			} else {
				normCalls.addSearch(word, cmpOp, getSearchFunc(searcher, cmpOp))
			}
		}
	}

	plainCalls = make(searchCallMap, len(search))
	for _, val := range search {
		word := strings.ToLower(strings.TrimSpace(val.value))
		if cmpOp := val.op; cmpOp.isNegated() {
			cmpOp = cmpOp.negate()
			negCalls.addSearch(word, cmpOp, getSearchFunc(searcher, cmpOp))
		} else {
			plainCalls.addSearch(word, cmpOp, getSearchFunc(searcher, cmpOp))
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

func retrievePositives(normCalls, plainCalls searchCallMap) id.Set {
	if isSuperset(normCalls, plainCalls) {
		var normResult id.Set
		for c, sf := range normCalls {
			normResult = normResult.IntersectOrSet(sf(c.s))
		}
		return normResult
	}

	type searchResults map[searchOp]id.Set
	var cache searchResults
	var plainResult id.Set
	for c, sf := range plainCalls {
		result := sf(c.s)
		if _, found := normCalls[c]; found {
			if cache == nil {
				cache = make(searchResults)
			}
			cache[c] = result
		}
		plainResult = plainResult.IntersectOrSet(result)
	}
	var normResult id.Set
	for c, sf := range normCalls {
		if cache != nil {
			if result, found := cache[c]; found {
				normResult = normResult.IntersectOrSet(result)
				continue
			}
		}
		normResult = normResult.IntersectOrSet(sf(c.s))
	}
	return normResult.Add(plainResult)
}

func isSuperset(normCalls, plainCalls searchCallMap) bool {
	for c := range plainCalls {
		if _, found := normCalls[c]; !found {
			return false
		}
	}
	return true
}

func retrieveNegatives(negCalls searchCallMap) id.Set {
	var negatives id.Set
	for val, sf := range negCalls {
		negatives = negatives.Add(sf(val.s))
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
		panic(fmt.Sprintf("Unexpected value of comparison operation: %v", op))
	}
}
