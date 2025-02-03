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

package meta

import (
	"strings"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/input"
	"t73f.de/r/zsc/maps"
	"zettelstore.de/z/strfun"
)

// NewFromInput parses the meta data of a zettel.
func NewFromInput(zid id.Zid, inp *input.Input) *Meta {
	if inp.Ch == '-' && inp.PeekN(0) == '-' && inp.PeekN(1) == '-' {
		skipToEOL(inp)
		inp.EatEOL()
	}
	meta := New(zid)
	for {
		inp.SkipSpace()
		switch inp.Ch {
		case '\r':
			if inp.Peek() == '\n' {
				inp.Next()
			}
			fallthrough
		case '\n':
			inp.Next()
			return meta
		case input.EOS:
			return meta
		case '%':
			skipToEOL(inp)
			inp.EatEOL()
			continue
		}
		parseHeader(meta, inp)
		if inp.Ch == '-' && inp.PeekN(0) == '-' && inp.PeekN(1) == '-' {
			skipToEOL(inp)
			inp.EatEOL()
			meta.YamlSep = true
			return meta
		}
	}
}

func parseHeader(m *Meta, inp *input.Input) {
	pos := inp.Pos
	for isHeader(inp.Ch) {
		inp.Next()
	}
	key := inp.Src[pos:inp.Pos]
	inp.SkipSpace()
	if inp.Ch == ':' {
		inp.Next()
	}
	var val []byte
	for {
		inp.SkipSpace()
		pos = inp.Pos
		skipToEOL(inp)
		val = append(val, inp.Src[pos:inp.Pos]...)
		inp.EatEOL()
		if !inp.IsSpace() {
			break
		}
		val = append(val, ' ')
	}
	addToMeta(m, string(key), string(val))
}

func skipToEOL(inp *input.Input) {
	for {
		switch inp.Ch {
		case '\n', '\r', input.EOS:
			return
		}
		inp.Next()
	}
}

// Return true iff rune is valid for header key.
func isHeader(ch rune) bool {
	return ('a' <= ch && ch <= 'z') ||
		('0' <= ch && ch <= '9') ||
		ch == '-' ||
		('A' <= ch && ch <= 'Z')
}

type predValidElem func(string) bool

func addToSet(set strfun.Set, elems []string, useElem predValidElem) {
	for _, s := range elems {
		if len(s) > 0 && useElem(s) {
			set.Set(s)
		}
	}
}

func addSet(m *Meta, key, val string, useElem predValidElem) {
	newElems := strings.Fields(val)
	oldElems, ok := m.GetList(key)
	if !ok {
		oldElems = nil
	}

	set := make(strfun.Set, len(newElems)+len(oldElems))
	addToSet(set, newElems, useElem)
	if len(set) == 0 {
		// Nothing to add. Maybe because of rejected elements.
		return
	}
	addToSet(set, oldElems, useElem)
	m.SetList(key, maps.Keys(set))
}

func addData(m *Meta, k, v string) {
	if o, ok := m.Get(k); !ok || o == "" {
		m.Set(k, v)
	} else if v != "" {
		m.Set(k, o+" "+v)
	}
}

func addToMeta(m *Meta, key, val string) {
	v := trimValue(val)
	key = strings.ToLower(key)
	if !KeyIsValid(key) {
		return
	}
	switch key {
	case "", api.KeyID:
		// Empty key and 'id' key will be ignored
		return
	}

	switch Type(key) {
	case TypeTagSet:
		addSet(m, key, strings.ToLower(v), func(s string) bool { return s[0] == '#' && len(s) > 1 })
	case TypeWord:
		m.Set(key, strings.ToLower(v))
	case TypeID:
		if _, err := id.Parse(v); err == nil {
			m.Set(key, v)
		}
	case TypeIDSet:
		addSet(m, key, v, func(s string) bool {
			_, err := id.Parse(s)
			return err == nil
		})
	case TypeTimestamp:
		if _, ok := TimeValue(v); ok {
			m.Set(key, v)
		}
	default:
		addData(m, key, v)
	}
}
