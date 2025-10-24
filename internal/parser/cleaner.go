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

	"zettelstore.de/z/internal/ast"
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

// CleanAST cleans the given block list.
func CleanAST(bs *ast.BlockSlice, allowHTML bool) {
	cv := cleanASTVisitor{
		allowHTML: allowHTML,
		hasMark:   false,
		doMark:    false,
	}
	ast.Walk(&cv, bs)
	if cv.hasMark {
		cv.doMark = true
		ast.Walk(&cv, bs)
	}
}

type cleanASTVisitor struct {
	ids       map[string]ast.Node
	allowHTML bool
	hasMark   bool
	doMark    bool
}

func (cv *cleanASTVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.BlockSlice:
		if !cv.allowHTML {
			cv.visitBlockSlice(n)
			return nil
		}
	case *ast.InlineSlice:
		if !cv.allowHTML {
			cv.visitInlineSlice(n)
			return nil
		}
	case *ast.HeadingNode:
		cv.visitHeading(n)
		return nil
	case *ast.MarkNode:
		cv.visitMark(n)
		return nil
	}
	return cv
}

func (cv *cleanASTVisitor) visitBlockSlice(bs *ast.BlockSlice) {
	if bs == nil {
		return
	}
	if len(*bs) == 0 {
		*bs = nil
		return
	}
	for _, bn := range *bs {
		ast.Walk(cv, bn)
	}

	fromPos, toPos := 0, 0
	for fromPos < len(*bs) {
		(*bs)[toPos] = (*bs)[fromPos]
		fromPos++
		switch bn := (*bs)[toPos].(type) {
		case *ast.VerbatimNode:
			if bn.Kind != ast.VerbatimHTML {
				toPos++
			}
		default:
			toPos++
		}
	}
	for pos := toPos; pos < len(*bs); pos++ {
		(*bs)[pos] = nil // Allow excess nodes to be garbage collected.
	}
	*bs = (*bs)[:toPos:toPos]
}

func (cv *cleanASTVisitor) visitInlineSlice(is *ast.InlineSlice) {
	if is == nil {
		return
	}
	if len(*is) == 0 {
		*is = nil
		return
	}
	for _, bn := range *is {
		ast.Walk(cv, bn)
	}
}

func (cv *cleanASTVisitor) visitHeading(hn *ast.HeadingNode) {
	if cv.doMark || hn == nil || len(hn.Inlines) == 0 {
		return
	}
	if hn.Slug == "" {
		var sb strings.Builder
		var textEnc encoder.TextEncoder
		if err := textEnc.WriteInlines(&sb, &hn.Inlines); err != nil {
			return
		}
		hn.Slug = zerostrings.Slugify(sb.String())
	}
	if hn.Slug != "" {
		hn.Fragment = cv.addIdentifier(hn.Slug, hn)
	}
}

func (cv *cleanASTVisitor) visitMark(mn *ast.MarkNode) {
	if !cv.doMark {
		cv.hasMark = true
		return
	}
	if mn.Mark == "" {
		mn.Slug = ""
		mn.Fragment = cv.addIdentifier("*", mn)
		return
	}
	if mn.Slug == "" {
		mn.Slug = zerostrings.Slugify(mn.Mark)
	}
	mn.Fragment = cv.addIdentifier(mn.Slug, mn)
}

func (cv *cleanASTVisitor) addIdentifier(id string, node ast.Node) string {
	if cv.ids == nil {
		cv.ids = map[string]ast.Node{id: node}
		return id
	}
	if n, ok := cv.ids[id]; ok && n != node {
		prefix := id + "-"
		for count := 1; ; count++ {
			newID := prefix + strconv.Itoa(count)
			if n2, ok2 := cv.ids[newID]; !ok2 || n2 == node {
				cv.ids[newID] = node
				return newID
			}
		}
	}
	cv.ids[id] = node
	return id
}
