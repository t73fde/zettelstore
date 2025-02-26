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

package ast_test

import (
	"testing"

	"t73f.de/r/zsc/attrs"
	"zettelstore.de/z/ast"
)

func BenchmarkWalk(b *testing.B) {
	root := ast.BlockSlice{
		&ast.HeadingNode{
			Inlines: ast.InlineSlice{&ast.TextNode{Text: "A Simple Heading"}},
		},
		&ast.ParaNode{
			Inlines: ast.InlineSlice{&ast.TextNode{Text: "This is the introduction."}},
		},
		&ast.NestedListNode{
			Kind: ast.NestedListUnordered,
			Items: []ast.ItemSlice{
				[]ast.ItemNode{
					&ast.ParaNode{
						Inlines: ast.InlineSlice{&ast.TextNode{Text: "Item 1"}},
					},
				},
				[]ast.ItemNode{
					&ast.ParaNode{
						Inlines: ast.InlineSlice{&ast.TextNode{Text: "Item 2"}},
					},
				},
			},
		},
		&ast.ParaNode{
			Inlines: ast.InlineSlice{&ast.TextNode{Text: "This is some intermediate text."}},
		},
		ast.CreateParaNode(
			&ast.FormatNode{
				Kind: ast.FormatEmph,
				Attrs: attrs.Attributes(map[string]string{
					"":      "class",
					"color": "green",
				}),
				Inlines: ast.InlineSlice{&ast.TextNode{Text: "This is some emphasized text."}},
			},
			&ast.TextNode{Text: " "},
			&ast.LinkNode{
				Ref:     &ast.Reference{Value: "http://zettelstore.de"},
				Inlines: ast.InlineSlice{&ast.TextNode{Text: "URL text."}},
			},
		),
	}
	v := benchVisitor{}

	for b.Loop() {
		ast.Walk(&v, &root)
	}
}

type benchVisitor struct{}

func (bv *benchVisitor) Visit(ast.Node) ast.Visitor { return bv }
