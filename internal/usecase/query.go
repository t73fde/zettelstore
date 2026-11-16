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
	"slices"
	"strings"

	"t73f.de/r/sx"
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsc/webapi"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/collect"
	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/zettel"
)

// QueryPort is the interface used by this use case.
type QueryPort interface {
	GetZettel(context.Context, id.Zid) (zettel.Zettel, error)
	FindMeta(context.Context, id.Zid) *meta.Meta
	SelectMeta(context.Context, []*meta.Meta, *query.Query) ([]*meta.Meta, error)
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
	metaList := uc.getMetaZid(ctx, zids)
	metaList = uc.processDirectives(ctx, metaList, q.GetDirectives())
	if len(metaList) > 0 {
		return uc.port.SelectMeta(ctx, metaList, q)
	}
	return nil, nil
}

func (uc *Query) getMetaZid(ctx context.Context, zids []id.Zid) []*meta.Meta {
	metaList := make([]*meta.Meta, 0, len(zids))
	for _, zid := range zids {
		if m := uc.port.FindMeta(ctx, zid); m != nil {
			metaList = append(metaList, m)
		}
	}
	return metaList
}

func (uc *Query) processDirectives(
	ctx context.Context,
	metaList []*meta.Meta,
	directives []query.Directive,
) []*meta.Meta {
	if len(directives) == 0 {
		return metaList
	}
	for _, dir := range directives {
		if len(metaList) == 0 {
			return nil
		}
		switch ds := dir.(type) {
		case *query.ContextSpec:
			metaList = uc.processContextDirective(ctx, ds, metaList)
		case *query.ThreadSpec:
			metaList = uc.processThreadDirective(ctx, ds, metaList)
		case *query.IdentSpec:
			// Nothing to do.
		case *query.ItemsSpec:
			metaList = uc.processItemsDirective(ctx, ds, metaList)
		case *query.UnlinkedSpec:
			metaList = uc.processUnlinkedDirective(ctx, ds, metaList)
		default:
			// Assume IDENT directive, it will not hurt
		}
	}
	if len(metaList) == 0 {
		return nil
	}
	return metaList
}

func (uc *Query) processContextDirective(
	ctx context.Context,
	spec *query.ContextSpec,
	metaList []*meta.Meta,
) []*meta.Meta {
	return spec.Execute(ctx, metaList, uc.port)
}

func (uc *Query) processThreadDirective(
	ctx context.Context,
	spec *query.ThreadSpec,
	metaList []*meta.Meta,
) []*meta.Meta {
	return spec.Execute(ctx, metaList, uc.port)
}

func (uc *Query) processItemsDirective(
	ctx context.Context,
	_ *query.ItemsSpec,
	metaList []*meta.Meta,
) []*meta.Meta {
	result := make([]*meta.Meta, 0, len(metaList))
	for _, m := range metaList {
		zn, err := uc.ucEvaluate.Run(
			ctx, m.Zid, string(m.GetDefault(meta.KeySyntax, meta.DefaultSyntax)))
		if err != nil {
			continue
		}
		for ln := range collect.Order(zn.Blocks).Pairs() {
			_, ref, _ := zsx.GetLink(ln.Head())
			refSym, refVal := zsx.GetReference(ref)
			if sz.SymRefStateZettel.IsEqualSymbol(refSym) {
				val, _ := sz.SplitFragment(refVal)
				if collectedZid, err2 := id.Parse(val); err2 == nil {
					collectedMeta := uc.port.FindMeta(ctx, collectedZid)
					if collectedMeta != nil {
						result = append(result, collectedMeta)
					}
				}
			}
		}
	}
	return result
}

func (uc *Query) processUnlinkedDirective(
	ctx context.Context,
	spec *query.UnlinkedSpec,
	metaList []*meta.Meta,
) []*meta.Meta {
	phraseWords := spec.GetPhraseWords(metaList)
	if len(phraseWords) == 0 {
		return metaList
	}
	var sb strings.Builder
	for i, word := range phraseWords {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(webapi.SearchOperatorEqual)
		sb.WriteString(word)
	}
	q := query.Parse(sb.String())
	candidates, err := uc.port.SelectMeta(ctx, nil, q)
	if err != nil {
		return nil
	}

	ignoreZids := make(map[id.Zid]struct{}, len(metaList)*4) // Assumption: there are four zids per zettel
	for _, m := range metaList {
		ignoreZids[m.Zid] = struct{}{}
		for key, val := range m.RestAny() {
			switch meta.Type(key) {
			case meta.TypeID:
				if zid, errParse := id.Parse(string(val)); errParse == nil {
					ignoreZids[zid] = struct{}{}
				}
			case meta.TypeIDSet:
				for val := range val.Fields() {
					if zid, errParse := id.Parse(val); errParse == nil {
						ignoreZids[zid] = struct{}{}
					}
				}
			}
		}
	}
	candidates = filterByZid(candidates, ignoreZids)
	return uc.filterCandidates(ctx, candidates, phraseWords)
}

func filterByZid(candidates []*meta.Meta, ignoreZids map[id.Zid]struct{}) []*meta.Meta {
	result := make([]*meta.Meta, 0, len(candidates))
	for _, m := range candidates {
		if _, found := ignoreZids[m.Zid]; !found {
			result = append(result, m)
		}
	}
	return result
}

func (uc *Query) filterCandidates(
	ctx context.Context,
	candidates []*meta.Meta,
	phraseWords []string,
) []*meta.Meta {
	result := make([]*meta.Meta, 0, len(candidates))
	ulv := unlinkedVisitor{phraseWords: phraseWords}
	for _, cand := range candidates {
		zettel, err := uc.port.GetZettel(ctx, cand.Zid)
		if err != nil {
			continue
		}

		syntax := string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax))
		if !parser.IsASTParser(syntax) {
			continue
		}
		zn := uc.ucEvaluate.RunZettel(ctx, zettel, syntax)

		ulv.found = false
		zsx.WalkIt(&ulv, zn.Blocks, nil)
		ulv.checkWords()
		if ulv.found {
			result = append(result, cand)
		}
	}
	return result
}

type unlinkedVisitor struct {
	phraseWords []string
	collWords   []string
	found       bool
}

func (v *unlinkedVisitor) VisitItBefore(node *sx.Pair, _ *sx.Pair) bool {
	if v.found {
		return true
	}
	switch sym := zsx.NodeSymbol(node); sym {
	case zsx.SymText:
		v.collWords = append(v.collWords, zerostrings.SplitWords(zsx.GetText(node))...)
		return true
	case zsx.SymSoft, zsx.SymHard:
		return true
	case zsx.SymHeading,
		zsx.SymLink, zsx.SymEmbed, zsx.SymEmbedBLOB, zsx.SymCite:
		v.checkWords()
		return true // No further search.
	default:
		v.checkWords()
		return false
	}
}
func (*unlinkedVisitor) VisitItAfter(*sx.Pair, *sx.Pair) {}

// checkWords tries to match phrase-words as part of the collected words
func (v *unlinkedVisitor) checkWords() {
	if len(v.collWords) == 0 {
		return
	}
	if v.found {
		v.collWords = v.collWords[:0]
		return
	}

	for i := 0; i+len(v.phraseWords) <= len(v.collWords); i++ {
		if slices.Equal(v.phraseWords, v.collWords[i:i+len(v.phraseWords)]) {
			v.collWords = v.collWords[:0]
			v.found = true
			return
		}
	}
	v.collWords = v.collWords[:0]
}
