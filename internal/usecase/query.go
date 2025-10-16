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

package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/collect"
	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/zettel"
)

// QueryPort is the interface used by this use case.
type QueryPort interface {
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)
	GetMeta(ctx context.Context, zid id.Zid) (*meta.Meta, error)
	SelectMeta(ctx context.Context, metaSeq []*meta.Meta, q *query.Query) ([]*meta.Meta, error)
}

// Query is the data for this use case.
type Query struct {
	port       QueryPort
	ucEvaluate Evaluate
}

// NewQuery creates a new use case.
func NewQuery(port QueryPort) Query {
	return Query{port: port}
}

// SetEvaluate sets the usecase Evaluate, because of circular dependencies.
func (uc *Query) SetEvaluate(ucEvaluate *Evaluate) { uc.ucEvaluate = *ucEvaluate }

// Run executes the use case.
func (uc *Query) Run(ctx context.Context, q *query.Query) ([]*meta.Meta, error) {
	zids := q.GetZids()
	if zids == nil {
		return uc.port.SelectMeta(ctx, nil, q)
	}
	if len(zids) == 0 {
		return nil, nil
	}
	metaSeq, err := uc.getMetaZid(ctx, zids)
	if err != nil {
		return metaSeq, err
	}
	if metaSeq = uc.processDirectives(ctx, metaSeq, q.GetDirectives()); len(metaSeq) > 0 {
		return uc.port.SelectMeta(ctx, metaSeq, q)
	}
	return nil, nil
}

func (uc *Query) getMetaZid(ctx context.Context, zids []id.Zid) ([]*meta.Meta, error) {
	metaSeq := make([]*meta.Meta, 0, len(zids))
	for _, zid := range zids {
		m, err := uc.port.GetMeta(ctx, zid)
		if err == nil {
			metaSeq = append(metaSeq, m)
			continue
		}
		if errors.Is(err, &box.ErrNotAllowed{}) {
			continue
		}
		return metaSeq, err
	}
	return metaSeq, nil
}

func (uc *Query) processDirectives(ctx context.Context, metaSeq []*meta.Meta, directives []query.Directive) []*meta.Meta {
	if len(directives) == 0 {
		return metaSeq
	}
	for _, dir := range directives {
		if len(metaSeq) == 0 {
			return nil
		}
		switch ds := dir.(type) {
		case *query.ContextSpec:
			metaSeq = uc.processContextDirective(ctx, ds, metaSeq)
		case *query.ThreadSpec:
			metaSeq = uc.processThreadDirective(ctx, ds, metaSeq)
		case *query.IdentSpec:
			// Nothing to do.
		case *query.ItemsSpec:
			metaSeq = uc.processItemsDirective(ctx, ds, metaSeq)
		case *query.UnlinkedSpec:
			metaSeq = uc.processUnlinkedDirective(ctx, ds, metaSeq)
		default:
			panic(fmt.Sprintf("Unknown directive %T", ds))
		}
	}
	if len(metaSeq) == 0 {
		return nil
	}
	return metaSeq
}

func (uc *Query) processContextDirective(ctx context.Context, spec *query.ContextSpec, metaSeq []*meta.Meta) []*meta.Meta {
	return spec.Execute(ctx, metaSeq, uc.port)
}

func (uc *Query) processThreadDirective(ctx context.Context, spec *query.ThreadSpec, metaSeq []*meta.Meta) []*meta.Meta {
	return spec.Execute(ctx, metaSeq, uc.port)
}

func (uc *Query) processItemsDirective(ctx context.Context, _ *query.ItemsSpec, metaSeq []*meta.Meta) []*meta.Meta {
	result := make([]*meta.Meta, 0, len(metaSeq))
	for _, m := range metaSeq {
		zn, err := uc.ucEvaluate.Run(ctx, m.Zid, string(m.GetDefault(meta.KeySyntax, meta.DefaultSyntax)))
		if err != nil {
			continue
		}
		for _, ln := range collect.OrderAST(zn) {
			ref := ln.Ref
			if !ref.IsZettel() {
				continue
			}

			if collectedZid, err2 := id.Parse(ref.URL.Path); err2 == nil {
				if z, err3 := uc.port.GetZettel(ctx, collectedZid); err3 == nil {
					result = append(result, z.Meta)
				}
			}
		}
	}
	return result
}

func (uc *Query) processUnlinkedDirective(ctx context.Context, spec *query.UnlinkedSpec, metaSeq []*meta.Meta) []*meta.Meta {
	words := spec.GetWords(metaSeq)
	if len(words) == 0 {
		return metaSeq
	}
	var sb strings.Builder
	for _, word := range words {
		sb.WriteString(" :")
		sb.WriteString(word)
	}
	q := (*query.Query)(nil).Parse(sb.String())
	candidates, err := uc.port.SelectMeta(ctx, nil, q)
	if err != nil {
		return nil
	}
	metaZids := idset.NewCap(len(metaSeq))
	refZids := idset.NewCap(len(metaSeq) * 4) // Assumption: there are four zids per zettel
	for _, m := range metaSeq {
		metaZids.Add(m.Zid)
		refZids.Add(m.Zid)
		for key, val := range m.ComputedRest() {
			switch meta.Type(key) {
			case meta.TypeID:
				if zid, errParse := id.Parse(string(val)); errParse == nil {
					refZids.Add(zid)
				}
			case meta.TypeIDSet:
				for val := range val.Fields() {
					if zid, errParse := id.Parse(val); errParse == nil {
						refZids.Add(zid)
					}
				}
			}
		}
	}
	candidates = filterByZid(candidates, refZids)
	return uc.filterCandidates(ctx, candidates, words)
}

func filterByZid(candidates []*meta.Meta, ignoreSeq *idset.Set) []*meta.Meta {
	result := make([]*meta.Meta, 0, len(candidates))
	for _, m := range candidates {
		if !ignoreSeq.Contains(m.Zid) {
			result = append(result, m)
		}
	}
	return result
}

func (uc *Query) filterCandidates(ctx context.Context, candidates []*meta.Meta, words []string) []*meta.Meta {
	result := make([]*meta.Meta, 0, len(candidates))
	for _, cand := range candidates {
		zettel, err := uc.port.GetZettel(ctx, cand.Zid)
		if err != nil {
			continue
		}
		v := unlinkedVisitorAST{
			words: words,
			found: false,
		}
		v.text = v.joinWords(words)

		syntax := string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax))
		if !parser.IsASTParser(syntax) {
			continue
		}
		zn := uc.ucEvaluate.RunZettel(ctx, zettel, syntax)
		ast.Walk(&v, &zn.BlocksAST)
		if v.found {
			result = append(result, cand)
		}
	}
	return result
}

func (*unlinkedVisitorAST) joinWords(words []string) string {
	return " " + strings.ToLower(strings.Join(words, " ")) + " "
}

type unlinkedVisitorAST struct {
	words []string
	text  string
	found bool
}

func (v *unlinkedVisitorAST) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.InlineSlice:
		v.checkWordsAST(n)
		return nil
	case *ast.HeadingNode:
		return nil
	case *ast.LinkNode, *ast.EmbedRefNode, *ast.EmbedBLOBNode, *ast.CiteNode:
		return nil
	}
	return v
}

func (v *unlinkedVisitorAST) checkWordsAST(is *ast.InlineSlice) {
	if len(*is) < 2*len(v.words)-1 {
		return
	}
	for _, text := range v.splitInlineTextListAST(is) {
		if strings.Contains(text, v.text) {
			v.found = true
		}
	}
}

func (v *unlinkedVisitorAST) splitInlineTextListAST(is *ast.InlineSlice) []string {
	var result []string
	var curList []string
	for _, in := range *is {
		switch n := in.(type) {
		case *ast.TextNode:
			curList = append(curList, zerostrings.MakeWords(n.Text)...)
		default:
			if curList != nil {
				result = append(result, v.joinWords(curList))
				curList = nil
			}
		}
	}
	if curList != nil {
		result = append(result, v.joinWords(curList))
	}
	return result
}
