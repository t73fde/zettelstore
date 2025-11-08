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

// Package evaluator interprets and evaluates the AST.
package evaluator

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/zettel"
)

// Port contains all methods to retrieve zettel (or part of it) to evaluate a zettel.
type Port interface {
	GetZettel(context.Context, id.Zid) (zettel.Zettel, error)
	QueryMeta(ctx context.Context, q *query.Query) ([]*meta.Meta, error)
}

// EvaluateZettel evaluates the given zettel in the given context, with the
// given ports, and the given environment.
func EvaluateZettel(ctx context.Context, port Port, rtConfig config.Config, zn *ast.Zettel) {
	switch zn.Syntax {
	case meta.ValueSyntaxNone:
		// AST is empty, evaluate to a description list of metadata.
		zn.Blocks = evaluateMetadata(zn.Meta)
	case meta.ValueSyntaxSxn:
		zn.Blocks = evaluateSxn(zn.Blocks)
	default:
		zn.Blocks = EvaluateBlock(ctx, port, rtConfig, zn.Blocks)
	}
}

// EvaluateBlock evaluates the given block list in the given context, with
// the given ports, and the given environment.
func EvaluateBlock(ctx context.Context, port Port, rtConfig config.Config, block *sx.Pair) *sx.Pair {
	e := evaluator{
		ctx:             ctx,
		port:            port,
		rtConfig:        rtConfig,
		transcludeMax:   rtConfig.GetMaxTransclusions(),
		transcludeCount: 0,
		costMap:         map[id.Zid]transcludeCost{},
		embedMap:        map[string]*sx.Pair{},
		marker:          &ast.Zettel{},
	}
	return mustPair(zsx.Walk(&e, block, nil))
}

type evaluator struct {
	ctx             context.Context
	port            Port
	rtConfig        config.Config
	transcludeMax   int
	transcludeCount int
	costMap         map[id.Zid]transcludeCost
	marker          *ast.Zettel
	embedMap        map[string]*sx.Pair
}

type transcludeCost struct {
	zn *ast.Zettel
	ec int
}

func (e *evaluator) VisitBefore(_ *sx.Pair, _ *sx.Pair) (sx.Object, bool) {
	return sx.Nil(), false
}
func (e *evaluator) VisitAfter(node *sx.Pair, _ *sx.Pair) sx.Object {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymLink:
			return e.evalLink(node)
		case zsx.SymEmbed:
			return e.evalEmbed(node)
		case zsx.SymVerbatimEval:
			return e.evalVerbatimEval(node)
		case zsx.SymTransclude:
			return e.evalTransclusion(node)
		case zsx.SymVerbatimZettel:
			return e.evalVerbatimZettel(node)
		}
	}
	return node
}

func (e *evaluator) evaluateEmbeddedZettel(zettel zettel.Zettel) *ast.Zettel {
	zn := parser.ParseZettel(e.ctx, zettel, string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax)), e.rtConfig)
	parser.Clean(zn.Blocks)
	zn.Blocks = mustPair(zsx.Walk(e, zn.Blocks, nil))
	return zn
}

func setMetadataFromAttributes(m *meta.Meta, attrs *sx.Pair) {
	for obj := range attrs.Values() {
		if pair, isPair := sx.GetPair(obj); isPair {
			if key, isKey := sx.GetString(pair.Car()); isKey && meta.KeyIsValid(key.GetValue()) {
				if val, isVal := sx.GetString(pair.Cdr()); isVal {
					m.Set(key.GetValue(), meta.Value(val.GetValue()))
				}
			}
		}
	}
}

func mustPair(obj sx.Object) *sx.Pair {
	p, isPair := sx.GetPair(obj)
	if !isPair {
		panic(fmt.Sprintf("not a pair after evaluate: %T/%v", obj, obj))
	}
	return p
}

func mustParseZid(ref *sx.Pair, refVal string) id.Zid {
	baseVal, _ := sz.SplitFragment(refVal)
	zid, err := id.Parse(baseVal)
	if err == nil {
		return zid
	}
	refState, _ := zsx.GetReference(ref)
	panic(fmt.Sprintf("%v: %q (state %v) -> %v", err, refVal, refState, ref))
}

func getSyntax(attrs *sx.Pair, defSyntax meta.Value) meta.Value {
	for a := range attrs.Values() {
		if pair, isPair := sx.GetPair(a); isPair {
			car := pair.Car()
			if car.IsEqual(sx.MakeString(meta.KeySyntax)) || car.IsEqual(sx.MakeString("")) {
				if val, isString := sx.GetString(pair.Cdr()); isString {
					return meta.Value(val.GetValue())
				}
			}
		}
	}
	return defSyntax
}

func (e *evaluator) checkMaxTransclusions(ref *sx.Pair) *sx.Pair {
	if maxTrans := e.transcludeMax; e.transcludeCount > maxTrans {
		e.transcludeCount = maxTrans + 1
		return createInlineErrorText(ref,
			"Too many transclusions (must be at most "+strconv.Itoa(maxTrans)+
				", see runtime configuration key max-transclusions)")
	}
	return nil
}

func createInlineErrorText(ref *sx.Pair, message string) *sx.Pair {
	text := message
	if ref != nil {
		text += ": " + sz.ReferenceString(ref) + "."
	}
	ln := zsx.MakeLiteral(zsx.SymLiteralOutput, nil, text)
	fn := zsx.MakeFormat(zsx.SymFormatStrong,
		sx.MakeList(sx.Cons(sx.MakeString("class"), sx.MakeString("error"))),
		sx.MakeList(ln))
	return fn
}

func createInlineErrorImage(attrs *sx.Pair, text *sx.Pair) *sx.Pair {
	ref := sz.ScanReference(id.ZidEmoji.String())
	if text == nil {
		text = sx.MakeList(zsx.MakeText("Error placeholder"))
	}
	return zsx.MakeEmbed(attrs, ref, "", text)
}

func styleAttr(attrs *sx.Pair, keys ...string) *sx.Pair {
	if attrs == nil {
		return attrs
	}
	a := zsx.GetAttributes(attrs)
	style := strings.TrimSpace(a["style"])
	var sb strings.Builder
	sb.WriteString(style)
	if style != "" && style[len(style)-1] != ';' {
		sb.WriteByte(';')
	}
	found := false
	for _, key := range keys {
		if val, ok := a[key]; ok {
			if found || style != "" {
				sb.WriteByte(' ')
			}
			sb.WriteString(key)
			sb.WriteString(": ")
			sb.WriteString(val)
			sb.WriteByte(';')
			delete(a, key)
			found = true
		}
	}

	if found {
		a["style"] = sb.String()
		return a.AsAssoc()
	}
	return attrs
}
