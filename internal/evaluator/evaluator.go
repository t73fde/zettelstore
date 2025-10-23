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
	"net/url"
	"strconv"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/ast/sztrans"
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
		zn.BlocksAST = EvaluateBlock(ctx, port, rtConfig, zn.Blocks)
		return
	}

	if blk, err := sztrans.GetBlockSlice(zn.Blocks); err == nil {
		zn.BlocksAST = blk
	}
}

// EvaluateBlock evaluates the given block list in the given context, with
// the given ports, and the given environment.
func EvaluateBlock(ctx context.Context, port Port, rtConfig config.Config, block *sx.Pair) ast.BlockSlice {
	e := evaluator{
		ctx:             ctx,
		port:            port,
		rtConfig:        rtConfig,
		transcludeMax:   rtConfig.GetMaxTransclusions(),
		transcludeCount: 0,
		costMap:         map[id.Zid]transcludeCost{},
		embedMap:        map[string]*sx.Pair{},
		embedMapAST:     map[string]ast.InlineSlice{},
		marker:          &ast.Zettel{},
	}

	evalBlock := mustPair(zsx.Walk(&e, block, nil))
	bns, err := sztrans.GetBlockSlice(evalBlock)
	if err != nil {
		panic(err)
	}

	// Now evaluate everything that was not evaluated by SZ-walker.

	ast.Walk(&e, &bns)
	parser.CleanAST(&bns, true)
	return bns
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
	embedMapAST     map[string]ast.InlineSlice
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
	u, err := url.Parse(refVal)
	var zid id.Zid
	if err == nil {
		zid, err = id.Parse(u.Path)
	}
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

func splicedBlocks(block *sx.Pair) *sx.Pair {
	blocks := zsx.GetBlock(block)
	if blocks.Tail() == nil {
		return blocks.Head()
	}
	return blocks.Cons(zsx.SymSpecialSplice)
}

// ---------------------------------------------------------------------------
// AST-based code, deprecated.

func (e *evaluator) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.InlineSlice:
		e.visitInlineSliceAST(n)
	default:
		return e
	}
	return nil
}

func (e *evaluator) checkMaxTransclusionsAST(ref *ast.Reference) ast.InlineNode {
	if maxTrans := e.transcludeMax; e.transcludeCount > maxTrans {
		e.transcludeCount = maxTrans + 1
		return createInlineErrorTextAST(ref,
			"Too many transclusions (must be at most "+strconv.Itoa(maxTrans)+
				", see runtime configuration key max-transclusions)")
	}
	return nil
}

func mustParseZidAST(ref *ast.Reference) id.Zid {
	zid, err := id.Parse(ref.URL.Path)
	if err != nil {
		panic(fmt.Sprintf("%v: %q (state %v) -> %v", err, ref.URL.Path, ref.State, ref))
	}
	return zid
}

func createInlineErrorImageAST(en *ast.EmbedRefNode) *ast.EmbedRefNode {
	errorZid := id.ZidEmoji
	en.Ref = ast.ParseReference(errorZid.String())
	if len(en.Inlines) == 0 {
		en.Inlines = ast.InlineSlice{&ast.TextNode{Text: "Error placeholder"}}
	}
	return en
}

func createInlineErrorTextAST(ref *ast.Reference, message string) ast.InlineNode {
	text := message
	if ref != nil {
		text += ": " + ref.String() + "."
	}
	ln := &ast.LiteralNode{
		Kind:    ast.LiteralInput,
		Content: []byte(text),
	}
	fn := &ast.FormatNode{
		Kind:    ast.FormatStrong,
		Inlines: ast.InlineSlice{ln},
	}
	fn.Attrs = fn.Attrs.AddClass("error")
	return fn
}

func (e *evaluator) evaluateEmbeddedZettelAST(zettel zettel.Zettel) *ast.Zettel {
	zn := parser.ParseZettel(e.ctx, zettel, string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax)), e.rtConfig)
	ast.Walk(e, &zn.BlocksAST)
	return zn
}
