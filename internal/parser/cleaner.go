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

	zerostrings "t73f.de/r/zero/strings"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/encoder"
)

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
	textEnc   encoder.TextEncoder
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
		if err := cv.textEnc.WriteInlines(&sb, &hn.Inlines); err != nil {
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
