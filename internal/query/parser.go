//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package query

import (
	"strconv"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"
)

// Parse the query specification and return a Query object.
func Parse(spec string) (q *Query) { return q.Parse(spec) }

// Parse the query string and update the Query object.
func (q *Query) Parse(spec string) *Query {
	state := parserState{
		inp: input.NewInput([]byte(spec)),
	}
	q = state.parse(q)
	if q != nil {
		for len(q.terms) > 1 && q.terms[len(q.terms)-1].isEmpty() {
			q.terms = q.terms[:len(q.terms)-1]
		}
	}
	return q
}

type parserState struct {
	inp *input.Input
}

func (ps *parserState) mustStop() bool { return ps.inp.Ch == input.EOS }
func (ps *parserState) acceptSingleKw(s string) bool {
	inp := ps.inp
	if inp.Accept(s) && (inp.IsSpace() || ps.isActionSep() || ps.mustStop()) {
		return true
	}
	return false
}
func (ps *parserState) acceptKwArgs(s string) bool {
	inp := ps.inp
	if inp.Accept(s) && inp.IsSpace() {
		inp.SkipSpace()
		return true
	}
	return false
}

const (
	actionSeparatorChar       = '|'
	existOperatorChar         = '?'
	searchOperatorNotChar     = '!'
	searchOperatorEqualChar   = '='
	searchOperatorHasChar     = ':'
	searchOperatorPrefixChar  = '['
	searchOperatorSuffixChar  = ']'
	searchOperatorMatchChar   = '~'
	searchOperatorLessChar    = '<'
	searchOperatorGreaterChar = '>'
)

func (ps *parserState) parse(q *Query) *Query {
	inp := ps.inp
	inp.SkipSpace()
	if ps.mustStop() {
		return q
	}
	firstPos := inp.Pos
	zidSet := idset.New()
	for {
		pos := inp.Pos
		zid, found := ps.scanZid()
		if !found {
			inp.SetPos(pos)
			break
		}
		if !zidSet.Contains(zid) {
			zidSet.Add(zid)
			q = createIfNeeded(q)
			q.zids = append(q.zids, zid)
		}
		inp.SkipSpace()
		if ps.mustStop() {
			q.zids = nil
			break
		}
	}

	hasContext := false
	for {
		inp.SkipSpace()
		if ps.mustStop() {
			break
		}
		pos := inp.Pos
		if ps.acceptSingleKw(api.ContextDirective) {
			if hasContext {
				inp.SetPos(pos)
				break
			}
			q = ps.parseContext(q)
			hasContext = true
			continue
		}
		inp.SetPos(pos)
		if q == nil || len(q.zids) == 0 {
			break
		}
		if ps.acceptSingleKw(api.IdentDirective) {
			q.directives = append(q.directives, &IdentSpec{})
			continue
		}
		inp.SetPos(pos)
		if ps.acceptSingleKw(api.ItemsDirective) {
			q.directives = append(q.directives, &ItemsSpec{})
			continue
		}
		inp.SetPos(pos)
		if ps.acceptSingleKw(api.UnlinkedDirective) {
			q = ps.parseUnlinked(q)
			continue
		}
		inp.SetPos(pos)
		break
	}
	if q != nil && len(q.directives) == 0 {
		inp.SetPos(firstPos) // No directive -> restart at beginning
		q.zids = nil
	}

	for {
		inp.SkipSpace()
		if ps.mustStop() {
			break
		}
		pos := inp.Pos
		if ps.acceptSingleKw(api.OrDirective) {
			q = createIfNeeded(q)
			if !q.terms[len(q.terms)-1].isEmpty() {
				q.terms = append(q.terms, conjTerms{})
			}
			continue
		}
		inp.SetPos(pos)
		if ps.acceptSingleKw(api.RandomDirective) {
			q = createIfNeeded(q)
			if len(q.order) == 0 {
				q.order = []sortOrder{{"", false}}
			}
			continue
		}
		inp.SetPos(pos)
		if ps.acceptKwArgs(api.PickDirective) {
			if s, ok := ps.parsePick(q); ok {
				q = s
				continue
			}
		}
		inp.SetPos(pos)
		if ps.acceptKwArgs(api.OrderDirective) {
			if s, ok := ps.parseOrder(q); ok {
				q = s
				continue
			}
		}
		inp.SetPos(pos)
		if ps.acceptKwArgs(api.OffsetDirective) {
			if s, ok := ps.parseOffset(q); ok {
				q = s
				continue
			}
		}
		inp.SetPos(pos)
		if ps.acceptKwArgs(api.LimitDirective) {
			if s, ok := ps.parseLimit(q); ok {
				q = s
				continue
			}
		}
		inp.SetPos(pos)
		if ps.isActionSep() {
			q = ps.parseActions(q)
			break
		}
		q = ps.parseText(q)
	}
	return q
}

func (ps *parserState) parseContext(q *Query) *Query {
	inp := ps.inp
	spec := &ContextSpec{}
	for {
		inp.SkipSpace()
		if ps.mustStop() {
			break
		}
		pos := inp.Pos
		if ps.acceptSingleKw(api.FullDirective) {
			spec.Full = true
			continue
		}
		inp.SetPos(pos)
		if ps.acceptSingleKw(api.BackwardDirective) {
			spec.Direction = ContextDirBackward
			continue
		}
		inp.SetPos(pos)
		if ps.acceptSingleKw(api.ForwardDirective) {
			spec.Direction = ContextDirForward
			continue
		}
		inp.SetPos(pos)
		if ps.acceptKwArgs(api.CostDirective) {
			if ps.parseCost(spec) {
				continue
			}
		}
		inp.SetPos(pos)
		if ps.acceptKwArgs(api.MaxDirective) {
			if ps.parseMaxCount(spec) {
				continue
			}
		}
		inp.SetPos(pos)
		if ps.acceptKwArgs(api.MinDirective) {
			if ps.parseMinCount(spec) {
				continue
			}
		}

		inp.SetPos(pos)
		break
	}
	q = createIfNeeded(q)
	q.directives = append(q.directives, spec)
	return q
}
func (ps *parserState) parseCost(spec *ContextSpec) bool {
	num, ok := ps.scanPosInt()
	if !ok {
		return false
	}
	if spec.MaxCost == 0 || spec.MaxCost >= num {
		spec.MaxCost = num
	}
	return true
}
func (ps *parserState) parseMaxCount(spec *ContextSpec) bool {
	num, ok := ps.scanPosInt()
	if !ok {
		return false
	}
	if spec.MaxCount == 0 || spec.MaxCount >= num {
		spec.MaxCount = num
	}
	return true
}
func (ps *parserState) parseMinCount(spec *ContextSpec) bool {
	num, ok := ps.scanPosInt()
	if !ok {
		return false
	}
	if spec.MinCount == 0 || spec.MinCount <= num {
		spec.MinCount = num
	}
	return true
}

func (ps *parserState) parseUnlinked(q *Query) *Query {
	inp := ps.inp

	spec := &UnlinkedSpec{}
	for {
		inp.SkipSpace()
		if ps.mustStop() {
			break
		}
		pos := inp.Pos
		if ps.acceptKwArgs(api.PhraseDirective) {
			if word := ps.scanWord(); len(word) > 0 {
				spec.words = append(spec.words, string(word))
				continue
			}
		}

		inp.SetPos(pos)
		break
	}
	q.directives = append(q.directives, spec)
	return q
}

func (ps *parserState) parsePick(q *Query) (*Query, bool) {
	num, ok := ps.scanPosInt()
	if !ok {
		return q, false
	}
	q = createIfNeeded(q)
	if q.pick == 0 || q.pick >= num {
		q.pick = num
	}
	return q, true
}

func (ps *parserState) parseOrder(q *Query) (*Query, bool) {
	reverse := false
	if ps.acceptKwArgs(api.ReverseDirective) {
		reverse = true
	}
	word := ps.scanWord()
	if len(word) == 0 {
		return q, false
	}
	if sWord := string(word); meta.KeyIsValid(sWord) {
		q = createIfNeeded(q)
		if len(q.order) == 1 && q.order[0].isRandom() {
			q.order = nil
		}
		q.order = append(q.order, sortOrder{sWord, reverse})
		return q, true
	}
	return q, false
}

func (ps *parserState) parseOffset(q *Query) (*Query, bool) {
	num, ok := ps.scanPosInt()
	if !ok {
		return q, false
	}
	q = createIfNeeded(q)
	if q.offset <= num {
		q.offset = num
	}
	return q, true
}

func (ps *parserState) parseLimit(q *Query) (*Query, bool) {
	num, ok := ps.scanPosInt()
	if !ok {
		return q, false
	}
	q = createIfNeeded(q)
	if q.limit == 0 || q.limit >= num {
		q.limit = num
	}
	return q, true
}

func (ps *parserState) parseActions(q *Query) *Query {
	inp := ps.inp
	inp.Next()
	var words []string
	for {
		inp.SkipSpace()
		word := ps.scanWord()
		if len(word) == 0 {
			break
		}
		words = append(words, string(word))
	}
	if len(words) > 0 {
		q = createIfNeeded(q)
		q.actions = words
	}
	return q
}

func (ps *parserState) parseText(q *Query) *Query {
	inp := ps.inp
	pos := inp.Pos
	op, hasOp := ps.scanSearchOp()
	if hasOp && (op == cmpExist || op == cmpNotExist) {
		inp.SetPos(pos)
		hasOp = false
	}
	text, key := ps.scanSearchTextOrKey(hasOp)
	if len(key) > 0 {
		// Assert: hasOp == false
		op, hasOp = ps.scanSearchOp()
		// Assert hasOp == true
		if op == cmpExist || op == cmpNotExist {
			if inp.IsSpace() || ps.isActionSep() || ps.mustStop() {
				return q.addKey(string(key), op)
			}
			ps.inp.SetPos(pos)
			hasOp = false
			text = ps.scanWord()
			key = nil
		} else {
			text = ps.scanWord()
		}
	} else if len(text) == 0 {
		// Only an empty search operation is found -> ignore it
		return q
	}
	q = createIfNeeded(q)
	if hasOp {
		if key == nil {
			q.addSearch(expValue{meta.Value(string(text)), op})
		} else {
			last := len(q.terms) - 1
			if q.terms[last].mvals == nil {
				q.terms[last].mvals = expMetaValues{string(key): {expValue{meta.Value(string(text)), op}}}
			} else {
				sKey := string(key)
				q.terms[last].mvals[sKey] = append(q.terms[last].mvals[sKey], expValue{meta.Value(string(text)), op})
			}
		}
	} else {
		// Assert key == nil
		q.addSearch(expValue{meta.Value(string(text)), cmpMatch})
	}
	return q
}

func (ps *parserState) scanSearchTextOrKey(hasOp bool) ([]byte, []byte) {
	inp := ps.inp
	pos := inp.Pos
	allowKey := !hasOp

	for !inp.IsSpace() && !ps.isActionSep() && !ps.mustStop() {
		if allowKey {
			switch inp.Ch {
			case searchOperatorNotChar, existOperatorChar,
				searchOperatorEqualChar, searchOperatorHasChar,
				searchOperatorPrefixChar, searchOperatorSuffixChar, searchOperatorMatchChar,
				searchOperatorLessChar, searchOperatorGreaterChar:
				allowKey = false
				if key := inp.Src[pos:inp.Pos]; meta.KeyIsValid(string(key)) {
					return nil, key
				}
			}
		}
		inp.Next()
	}
	return inp.Src[pos:inp.Pos], nil
}

func (ps *parserState) scanWord() []byte {
	inp := ps.inp
	pos := inp.Pos
	for !inp.IsSpace() && !ps.isActionSep() && !ps.mustStop() {
		inp.Next()
	}
	return inp.Src[pos:inp.Pos]
}

func (ps *parserState) scanPosInt() (int, bool) {
	word := ps.scanWord()
	if len(word) == 0 {
		return 0, false
	}
	uval, err := strconv.ParseUint(string(word), 10, 63)
	if err != nil {
		return 0, false
	}
	return int(uval), true
}

func (ps *parserState) scanZid() (id.Zid, bool) {
	word := ps.scanWord()
	if len(word) == 0 {
		return id.Invalid, false
	}
	uval, err := id.ParseUint(string(word))
	if err != nil {
		return id.Invalid, false
	}
	zid := id.Zid(uval)
	return zid, zid.IsValid()
}

func (ps *parserState) scanSearchOp() (compareOp, bool) {
	inp := ps.inp
	ch := inp.Ch
	negate := false
	if ch == searchOperatorNotChar {
		ch = inp.Next()
		negate = true
	}
	op := cmpUnknown
	switch ch {
	case existOperatorChar:
		inp.Next()
		op = cmpExist
	case searchOperatorEqualChar:
		inp.Next()
		op = cmpEqual
	case searchOperatorHasChar:
		inp.Next()
		op = cmpHas
	case searchOperatorSuffixChar:
		inp.Next()
		op = cmpSuffix
	case searchOperatorPrefixChar:
		inp.Next()
		op = cmpPrefix
	case searchOperatorMatchChar:
		inp.Next()
		op = cmpMatch
	case searchOperatorLessChar:
		inp.Next()
		op = cmpLess
	case searchOperatorGreaterChar:
		inp.Next()
		op = cmpGreater
	default:
		if negate {
			return cmpNoMatch, true
		}
		return cmpUnknown, false
	}
	if negate {
		return op.negate(), true
	}
	return op, true
}

func (ps *parserState) isActionSep() bool {
	return ps.inp.Ch == actionSeparatorChar
}
