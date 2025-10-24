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

package parser

// cleaner provides functions to clean up the parsed AST.

import (
	"strconv"
	"strings"

	"t73f.de/r/sx"
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/encoder"
)

// Clean the given SZ syntax tree.
func Clean(node *sx.Pair, allowHTML bool) {
	v1 := cleanPhase1{ids: idsNode{}, allowHTML: allowHTML}
	zsx.WalkIt(&v1, node, nil)
	if v1.hasMark {
		v2 := cleanPhase2{ids: v1.ids}
		zsx.WalkIt(&v2, node, nil)
	}
}

type cleanPhase1 struct {
	ids       idsNode
	allowHTML bool
	hasMark   bool // Mark nodes will be cleaned in phase 2 only
}

func (v *cleanPhase1) VisitItBefore(node *sx.Pair, _ *sx.Pair) bool {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymBlock:
			if !v.allowHTML {
				curr, next := node, node.Tail()
				for next != nil {
					sy, ok := sx.GetSymbol(next.Head().Car())
					if !ok || sy != zsx.SymVerbatimHTML {
						curr = next
						next = next.Tail()
					} else {
						next = next.Tail()
						curr.SetCdr(next)
					}
				}
			}

		case zsx.SymMark:
			v.hasMark = true
		}
	}
	return false
}
func (v *cleanPhase1) VisitItAfter(node *sx.Pair, _ *sx.Pair) {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymHeading:
			levelNode := node.Tail()
			attrsNode := levelNode.Tail()
			slugNode := attrsNode.Tail()
			fragmentNode := slugNode.Tail()

			textNode := fragmentNode.Tail()
			var sb strings.Builder
			var textEnc encoder.TextEncoder
			if err := textEnc.WriteSz(&sb, textNode.Cons(zsx.SymPara)); err != nil {
				return
			}

			slugText := zerostrings.Slugify(sb.String())
			slugNode.SetCar(sx.MakeString(slugText))
			fragmentNode.SetCar(sx.MakeString(v.ids.addIdentifier(slugText, node)))
		}
	}
}

type cleanPhase2 struct {
	ids idsNode
}

func (v *cleanPhase2) VisitItBefore(node *sx.Pair, _ *sx.Pair) bool {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymMark:
			stringNode := node.Tail()
			if markString, isString := sx.GetString(stringNode.Car()); isString {
				slugNode := stringNode.Tail()
				fragmentNode := slugNode.Tail()

				slugText := zerostrings.Slugify(markString.GetValue())
				slugNode.SetCar(sx.MakeString(slugText))
				fragmentNode.SetCar(sx.MakeString(v.ids.addIdentifier(slugText, node)))
			}
		}
	}
	return false
}
func (v *cleanPhase2) VisitItAfter(*sx.Pair, *sx.Pair) {}

type idsNode map[string]*sx.Pair

func (ids idsNode) addIdentifier(id string, node *sx.Pair) string {
	if n, ok := ids[id]; ok && n != node {
		prefix := id + "-"
		for count := 1; ; count++ {
			newID := prefix + strconv.Itoa(count)
			if n2, ok2 := ids[newID]; !ok2 || n2 == node {
				ids[newID] = node
				return newID
			}
		}
	}
	ids[id] = node
	return id
}
