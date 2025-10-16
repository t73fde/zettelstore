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

// OrderAST of internal links within the given zettel.
func OrderAST(bns *ast.BlockSlice) (result []*ast.LinkNode) {
	for _, bn := range *bns {
		ln, ok := bn.(*ast.NestedListNode)
		if !ok {
			continue
		}
		switch ln.Kind {
		case ast.NestedListOrdered, ast.NestedListUnordered:
			for _, is := range ln.Items {
				if ln := firstItemZettelLinkAST(is); ln != nil {
					result = append(result, ln)
				}
			}
		}
	}
	return result
}

func firstItemZettelLinkAST(is ast.ItemSlice) *ast.LinkNode {
	for _, in := range is {
		if pn, ok := in.(*ast.ParaNode); ok {
			if ln := firstInlineZettelLinkAST(pn.Inlines); ln != nil {
				return ln
			}
		}
	}
	return nil
}

func firstInlineZettelLinkAST(is ast.InlineSlice) (result *ast.LinkNode) {
	for _, inl := range is {
		switch in := inl.(type) {
		case *ast.LinkNode:
			return in
		case *ast.EmbedRefNode:
			result = firstInlineZettelLinkAST(in.Inlines)
		case *ast.EmbedBLOBNode:
			result = firstInlineZettelLinkAST(in.Inlines)
		case *ast.CiteNode:
			result = firstInlineZettelLinkAST(in.Inlines)
		case *ast.FootnoteNode:
			// Ignore references in footnotes
			continue
		case *ast.FormatNode:
			result = firstInlineZettelLinkAST(in.Inlines)
		default:
			continue
		}
		if result != nil {
			return result
		}
	}
	return nil
}
