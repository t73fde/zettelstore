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

import "zettelstore.de/z/internal/ast"

// Summary stores the relevant parts of the syntax tree
type Summary struct {
	Links  []*ast.Reference // list of all linked material
	Embeds []*ast.Reference // list of all embedded material
	Cites  []*ast.CiteNode  // list of all referenced citations
}

// References returns all references mentioned in the given zettel. This also
// includes references to images.
func References(zn *ast.ZettelNode) (s Summary) {
	ast.Walk(&s, &zn.BlocksAST)
	return s
}

// Visit all node to collect data for the summary.
func (s *Summary) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.TranscludeNode:
		s.Embeds = append(s.Embeds, n.Ref)
	case *ast.LinkNode:
		s.Links = append(s.Links, n.Ref)
	case *ast.EmbedRefNode:
		s.Embeds = append(s.Embeds, n.Ref)
	case *ast.CiteNode:
		s.Cites = append(s.Cites, n)
	}
	return s
}
