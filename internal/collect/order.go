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

// Package collect provides functions to collect items from a syntax tree.
package collect

import "zettelstore.de/z/internal/ast"

// Order of internal links within the given zettel.
func Order(zn *ast.ZettelNode) (result []*ast.LinkNode) {
	for _, bn := range zn.BlocksAST {
		ln, ok := bn.(*ast.NestedListNode)
		if !ok {
			continue
		}
		switch ln.Kind {
		case ast.NestedListOrdered, ast.NestedListUnordered:
			for _, is := range ln.Items {
				if ln := firstItemZettelLink(is); ln != nil {
					result = append(result, ln)
				}
			}
		}
	}
	return result
}

func firstItemZettelLink(is ast.ItemSlice) *ast.LinkNode {
	for _, in := range is {
		if pn, ok := in.(*ast.ParaNode); ok {
			if ln := firstInlineZettelLink(pn.Inlines); ln != nil {
				return ln
			}
		}
	}
	return nil
}

func firstInlineZettelLink(is ast.InlineSlice) (result *ast.LinkNode) {
	for _, inl := range is {
		switch in := inl.(type) {
		case *ast.LinkNode:
			return in
		case *ast.EmbedRefNode:
			result = firstInlineZettelLink(in.Inlines)
		case *ast.EmbedBLOBNode:
			result = firstInlineZettelLink(in.Inlines)
		case *ast.CiteNode:
			result = firstInlineZettelLink(in.Inlines)
		case *ast.FootnoteNode:
			// Ignore references in footnotes
			continue
		case *ast.FormatNode:
			result = firstInlineZettelLink(in.Inlines)
		default:
			continue
		}
		if result != nil {
			return result
		}
	}
	return nil
}
