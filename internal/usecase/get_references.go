//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

package usecase

import (
	"slices"

	zeroiter "t73f.de/r/zero/iter"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/collect"
)

// GetReferences is the usecase to retrieve references that occur in a zettel.
type GetReferences struct{}

// NewGetReferences creates a new usecase object.
func NewGetReferences() GetReferences { return GetReferences{} }

// RunByAST returns all references of a zettel, separating them according to
// there occurence in the abstract syntax tree: link, embed, cite.
func (uc GetReferences) RunByAST(zn *ast.ZettelNode) (links, embeds []*ast.Reference, cites []*ast.CiteNode) {
	s := collect.References(zn)
	return s.Links, s.Embeds, s.Cites
}

// RunByState returns all references of a zettel, sparated by their state:
// local, external, query. No zettel references are returned.
func (uc GetReferences) RunByState(zn *ast.ZettelNode) (local, ext, query []*ast.Reference) {
	s := collect.References(zn)
	for ref := range zeroiter.CatSeq(slices.Values(s.Links), slices.Values(s.Embeds)) {
		switch ref.State {
		case ast.RefStateHosted, ast.RefStateBased: // Local
			local = append(local, ref)
		case ast.RefStateExternal:
			ext = append(ext, ref)
		case ast.RefStateQuery:
			query = append(query, ref)
		}
	}
	return local, ext, query
}

// RunByMeta returns all URLs that are stored in the metadata.
func (uc GetReferences) RunByMeta(m *meta.Meta) (urls []string) {
	for key, val := range m.All() {
		if meta.Type(key) == meta.TypeURL {
			urls = append(urls, string(val))
		}
	}
	return urls
}
