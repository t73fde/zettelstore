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

// Package zettelmark provides a parser for zettelmarkup.
package zettelmark

import (
	"log"
	"strings"
	"unicode"

	"t73f.de/r/zsc/attrs"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"
	"t73f.de/r/zsc/sz/zmk"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/ast/sztrans"
	"zettelstore.de/z/parser"
)

func init() {
	parser.Register(&parser.Info{
		Name:          meta.ValueSyntaxZmk,
		AltNames:      nil,
		IsASTParser:   true,
		IsTextFormat:  true,
		IsImageFormat: false,
		ParseBlocks:   parseBlocks,
	})
	// parser.Register(&parser.Info{
	// 	Name:          meta.ValueSyntaxZmk,
	// 	AltNames:      nil,
	// 	IsASTParser:   true,
	// 	IsTextFormat:  true,
	// 	IsImageFormat: false,
	// 	ParseBlocks:   parseZmkBlocks,
	// })
}

func parseBlocks(inp *input.Input, _ *meta.Meta, _ string) ast.BlockSlice {
	parser := &zmkP{inp: inp}
	bs := parser.parseBlockSlice()
	postProcessBlocks(&bs)
	return bs
}

func parseZmkBlocks(inp *input.Input, _ *meta.Meta, _ string) ast.BlockSlice {
	if obj := zmk.ParseBlocks(inp); obj != nil {
		bs, err := sztrans.GetBlockSlice(obj)
		if err == nil {
			return bs
		}
		log.Printf("sztrans error: %v, for %v\n", err, obj)
	}
	return nil
}

type zmkP struct {
	inp          *input.Input             // Input stream
	lists        []*ast.NestedListNode    // Stack of lists
	table        *ast.TableNode           // Current table
	descrl       *ast.DescriptionListNode // Current description list
	nestingLevel int                      // Count nesting of block and inline elements
}

// runeModGrave is Unicode code point U+02CB (715) called "MODIFIER LETTER
// GRAVE ACCENT". On the iPad it is much more easier to type in this code point
// than U+0060 (96) "Grave accent" (aka backtick). Therefore, U+02CB will be
// considered equivalent to U+0060.
const runeModGrave = 'ˋ' // This is NOT '`'!

const maxNestingLevel = 50

// clearStacked removes all multi-line nodes from parser.
func (cp *zmkP) clearStacked() {
	cp.lists = nil
	cp.table = nil
	cp.descrl = nil
}

func (cp *zmkP) parseNormalAttribute(attrs map[string]string) bool {
	inp := cp.inp
	posK := inp.Pos
	for isNameRune(inp.Ch) {
		inp.Next()
	}
	if posK == inp.Pos {
		return false
	}
	key := string(inp.Src[posK:inp.Pos])
	if inp.Ch != '=' {
		attrs[key] = ""
		return true
	}
	return cp.parseAttributeValue(key, attrs)
}

func (cp *zmkP) parseAttributeValue(key string, attrs map[string]string) bool {
	inp := cp.inp
	inp.Next()
	if inp.Ch == '"' {
		return cp.parseQuotedAttributeValue(key, attrs)
	}
	posV := inp.Pos
	for {
		switch inp.Ch {
		case input.EOS:
			return false
		case '\n', '\r', ' ', '}':
			updateAttrs(attrs, key, string(inp.Src[posV:inp.Pos]))
			return true
		}
		inp.Next()
	}
}

func (cp *zmkP) parseQuotedAttributeValue(key string, attrs map[string]string) bool {
	inp := cp.inp
	inp.Next()
	var sb strings.Builder
	for {
		switch inp.Ch {
		case input.EOS:
			return false
		case '"':
			updateAttrs(attrs, key, sb.String())
			inp.Next()
			return true
		case '\\':
			inp.Next()
			switch inp.Ch {
			case input.EOS, '\n', '\r':
				return false
			}
			fallthrough
		default:
			sb.WriteRune(inp.Ch)
			inp.Next()
		}
	}

}

func updateAttrs(attrs map[string]string, key, val string) {
	if prevVal := attrs[key]; len(prevVal) > 0 {
		attrs[key] = prevVal + " " + val
	} else {
		attrs[key] = val
	}
}

func (cp *zmkP) parseBlockAttributes() attrs.Attributes {
	inp := cp.inp
	pos := inp.Pos
	for isNameRune(inp.Ch) {
		inp.Next()
	}
	if pos < inp.Pos {
		return attrs.Attributes{"": string(inp.Src[pos:inp.Pos])}
	}

	// No immediate name: skip spaces
	inp.SkipSpace()
	return cp.parseInlineAttributes()
}

func (cp *zmkP) parseInlineAttributes() attrs.Attributes {
	inp := cp.inp
	pos := inp.Pos
	if attrs, success := cp.doParseAttributes(); success {
		return attrs
	}
	inp.SetPos(pos)
	return nil
}

// doParseAttributes reads attributes.
func (cp *zmkP) doParseAttributes() (res attrs.Attributes, success bool) {
	inp := cp.inp
	if inp.Ch != '{' {
		return nil, false
	}
	inp.Next()
	a := attrs.Attributes{}
	if !cp.parseAttributeValues(a) {
		return nil, false
	}
	inp.Next()
	return a, true
}

func (cp *zmkP) parseAttributeValues(a attrs.Attributes) bool {
	inp := cp.inp
	for {
		cp.skipSpaceLine()
		switch inp.Ch {
		case input.EOS:
			return false
		case '}':
			return true
		case '.':
			inp.Next()
			posC := inp.Pos
			for isNameRune(inp.Ch) {
				inp.Next()
			}
			if posC == inp.Pos {
				return false
			}
			updateAttrs(a, "class", string(inp.Src[posC:inp.Pos]))
		case '=':
			delete(a, "")
			if !cp.parseAttributeValue("", a) {
				return false
			}
		default:
			if !cp.parseNormalAttribute(a) {
				return false
			}
		}

		switch inp.Ch {
		case '}':
			return true
		case '\n', '\r':
		case ' ', ',':
			inp.Next()
		default:
			return false
		}
	}
}

func (cp *zmkP) skipSpaceLine() {
	for inp := cp.inp; ; {
		switch inp.Ch {
		case ' ':
			inp.Next()
		case '\n', '\r':
			inp.EatEOL()
		default:
			return
		}
	}
}

func isNameRune(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '-' || ch == '_'
}
