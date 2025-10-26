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
	"slices"
	"strings"

	"t73f.de/r/sx"
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

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
		for ln := range collect.Order(zn.Blocks).Pairs() {
			_, ref, _ := zsx.GetLink(ln.Head())
			if refSym, refVal := zsx.GetReference(ref); sz.SymRefStateZettel.IsEqualSymbol(refSym) {
				val, _ := sz.SplitFragment(refVal)
				if collectedZid, err2 := id.Parse(val); err2 == nil {
					if collectedMeta, err3 := uc.port.GetMeta(ctx, collectedZid); err3 == nil {
						result = append(result, collectedMeta)
					}
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
	ulv := unlinkedVisitor{words: words}
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
		if ulv.found {
			result = append(result, cand)
		}
	}
	return result
}

type unlinkedVisitor struct {
	words []string
	found bool
}

func (v *unlinkedVisitor) VisitItBefore(node *sx.Pair, _ *sx.Pair) bool {
	if v.found {
		return true
	}
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymHeading,
			zsx.SymLink, zsx.SymEmbed, zsx.SymEmbedBLOB, zsx.SymCite:
			// No further search.
			return true
		case zsx.SymText:
			textWords := zerostrings.SplitWords(zsx.GetText(node))
			if len(v.words) <= len(textWords) {
				v.found = slices.Equal(v.words, textWords[:len(v.words)])
			}
		}
	}
	return false
}
func (*unlinkedVisitor) VisitItAfter(*sx.Pair, *sx.Pair) {}
