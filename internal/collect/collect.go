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

// Package collect provides functions to collect items from a syntax tree.
package collect

import (
	"iter"

	"zettelstore.de/z/internal/ast"
)

type refYielder struct {
	yield func(*ast.Reference) bool
	stop  bool
}

// ReferenceSeq returns an iterator of all references mentioned in the given
// zettel. This also includes references to images.
func ReferenceSeq(zn *ast.Zettel) iter.Seq[*ast.Reference] {
	return func(yield func(*ast.Reference) bool) {
		yielder := refYielder{yield, false}
		ast.Walk(&yielder, &zn.BlocksAST)
	}
}

// Visit all node to collect data for the summary.
func (y *refYielder) Visit(node ast.Node) ast.Visitor {
	if y.stop {
		return nil
	}
	var stop bool
	switch n := node.(type) {
	case *ast.TranscludeNode:
		stop = !y.yield(n.Ref)
	case *ast.LinkNode:
		stop = !y.yield(n.Ref)
	case *ast.EmbedRefNode:
		stop = !y.yield(n.Ref)
	}
	if stop {
		y.stop = true
		return nil
	}
	return y
}
