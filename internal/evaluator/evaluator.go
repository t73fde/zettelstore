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
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strconv"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/ast/sztrans"
	"zettelstore.de/z/internal/box"
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

	obj := zsx.Walk(&e, block, nil)
	evalBlock, isPair := sx.GetPair(obj)
	if !isPair {
		panic(fmt.Sprintf("not a pair after evaluate: %T/%v", obj, obj))
	}
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
		case zsx.SymVerbatimEval:
			return e.evalVerbatimEval(node)
		}
	}
	return node
}

func (e *evaluator) evalLink(node *sx.Pair) *sx.Pair {
	attrs, ref, inlines := zsx.GetLink(node)
	refState, refVal := zsx.GetReference(ref)
	newInlines := inlines
	if inlines == nil {
		newInlines = sx.MakeList(zsx.MakeText(refVal))
	}
	if !sz.SymRefStateZettel.IsEqualSymbol(refState) {
		if newInlines != inlines {
			return zsx.MakeLink(attrs, ref, newInlines)
		}
		return node
	}

	zid := mustParseZid(ref, refVal)
	_, err := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
	if errors.Is(err, &box.ErrNotAllowed{}) {
		return zsx.MakeFormat(zsx.SymFormatSpan, attrs, newInlines)
	}
	if err != nil {
		return zsx.MakeLink(attrs, zsx.MakeReference(sz.SymRefStateBroken, refVal), newInlines)
	}

	if newInlines != inlines {
		return zsx.MakeLink(attrs, ref, newInlines)
	}
	return node
}

func (e *evaluator) evalVerbatimEval(node *sx.Pair) *sx.Pair {
	_, attrs, content := zsx.GetVerbatim(node)
	if p := attrs.Assoc(sx.MakeString("")); p != nil {
		if s, isString := sx.GetString(p.Cdr()); isString && s.GetValue() == meta.ValueSyntaxDraw {
			return parser.ParseDrawBlock(attrs, []byte(content))
		}
	}
	return node
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

// AST-based code, deprecated.

func (e *evaluator) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.BlockSlice:
		e.visitBlockSliceAST(n)
	case *ast.InlineSlice:
		e.visitInlineSliceAST(n)
	default:
		return e
	}
	return nil
}

func (e *evaluator) visitBlockSliceAST(bs *ast.BlockSlice) {
	for i := 0; i < len(*bs); i++ {
		bn := (*bs)[i]
		ast.Walk(e, bn)
		switch n := bn.(type) {
		case *ast.VerbatimNode:
			if n.Kind == ast.VerbatimZettel {
				i += transcludeNodeAST(bs, i, e.evalVerbatimZettelNodeAST(n))
			}
		case *ast.TranscludeNode:
			i += transcludeNodeAST(bs, i, e.evalTransclusionNodeAST(n))
		}
	}
}

func transcludeNodeAST(bln *ast.BlockSlice, i int, bn ast.BlockNode) int {
	if ln, ok := bn.(*ast.BlockSlice); ok {
		*bln = replaceWithBlockNodesAST(*bln, i, *ln)
		return len(*ln) - 1
	}
	if bn == nil {
		(*bln) = (*bln)[:i+copy((*bln)[i:], (*bln)[i+1:])]
		return -1
	}
	(*bln)[i] = bn
	return 0
}

func replaceWithBlockNodesAST(bns []ast.BlockNode, i int, replaceBns []ast.BlockNode) []ast.BlockNode {
	if len(replaceBns) == 1 {
		bns[i] = replaceBns[0]
		return bns
	}
	newIns := make([]ast.BlockNode, 0, len(bns)+len(replaceBns)-1)
	if i > 0 {
		newIns = append(newIns, bns[:i]...)
	}
	if len(replaceBns) > 0 {
		newIns = append(newIns, replaceBns...)
	}
	if i+1 < len(bns) {
		newIns = append(newIns, bns[i+1:]...)
	}
	return newIns
}

func (e *evaluator) evalVerbatimZettelNodeAST(vn *ast.VerbatimNode) ast.BlockNode {
	m := meta.New(id.Invalid)
	m.Set(meta.KeySyntax, getSyntaxAST(vn.Attrs, meta.ValueSyntaxText))
	zettel := zettel.Zettel{
		Meta:    m,
		Content: zettel.NewContent(vn.Content),
	}
	e.transcludeCount++
	zn := e.evaluateEmbeddedZettelAST(zettel)
	return &zn.BlocksAST
}

func getSyntaxAST(a zsx.Attributes, defSyntax meta.Value) meta.Value {
	if a != nil {
		if val, ok := a.Get(meta.KeySyntax); ok {
			return meta.Value(val)
		}
		if val, ok := a.Get(""); ok {
			return meta.Value(val)
		}
	}
	return defSyntax
}

func (e *evaluator) evalTransclusionNodeAST(tn *ast.TranscludeNode) ast.BlockNode {
	ref := tn.Ref

	// To prevent e.embedCount from counting
	if errText := e.checkMaxTransclusionsAST(ref); errText != nil {
		return makeBlockNodeAST(errText)
	}
	switch ref.State {
	case ast.RefStateZettel:
		// Only zettel references will be evaluated.
	case ast.RefStateInvalid, ast.RefStateBroken:
		e.transcludeCount++
		return makeBlockNodeAST(createInlineErrorTextAST(ref, "Invalid or broken transclusion reference"))
	case ast.RefStateSelf:
		e.transcludeCount++
		return makeBlockNodeAST(createInlineErrorTextAST(ref, "Self transclusion reference"))
	case ast.RefStateFound, ast.RefStateExternal:
		return tn
	case ast.RefStateHosted, ast.RefStateBased:
		if n := createEmbeddedNodeLocalAST(ref); n != nil {
			n.Attrs = tn.Attrs
			return makeBlockNodeAST(n)
		}
		return tn
	case ast.RefStateQuery:
		e.transcludeCount++
		return e.evalQueryTransclusionAST(tn.Ref.Value)
	default:
		return makeBlockNodeAST(createInlineErrorTextAST(ref, "Illegal block state "+strconv.Itoa(int(ref.State))))
	}

	zid, err := id.Parse(ref.URL.Path)
	if err != nil {
		panic(err)
	}

	cost, ok := e.costMap[zid]
	zn := cost.zn
	if zn == e.marker {
		e.transcludeCount++
		return makeBlockNodeAST(createInlineErrorTextAST(ref, "Recursive transclusion"))
	}
	if !ok {
		zettel, err1 := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
		if err1 != nil {
			if errors.Is(err1, &box.ErrNotAllowed{}) {
				return nil
			}
			e.transcludeCount++
			return makeBlockNodeAST(createInlineErrorTextAST(ref, "Unable to get zettel"))
		}
		setMetadataFromAttributesAST(zettel.Meta, tn.Attrs)
		ec := e.transcludeCount
		e.costMap[zid] = transcludeCost{zn: e.marker, ec: ec}
		zn = e.evaluateEmbeddedZettelAST(zettel)
		e.costMap[zid] = transcludeCost{zn: zn, ec: e.transcludeCount - ec}
		e.transcludeCount = 0 // No stack needed, because embedding is done left-recursive, depth-first.
	}
	e.transcludeCount++
	if ec := cost.ec; ec > 0 {
		e.transcludeCount += cost.ec
	}
	return &zn.BlocksAST
}

func (e *evaluator) evalQueryTransclusionAST(expr string) ast.BlockNode {
	q := query.Parse(expr)
	ml, err := e.port.QueryMeta(e.ctx, q)
	if err != nil {
		if errors.Is(err, &box.ErrNotAllowed{}) {
			return nil
		}
		return makeBlockNodeAST(createInlineErrorTextAST(nil, "Unable to search zettel"))
	}
	result := QueryActionAST(e.ctx, q, ml)
	if result != nil {
		ast.Walk(e, result)
	}
	return result
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

func makeBlockNodeAST(in ast.InlineNode) ast.BlockNode { return ast.CreateParaNode(in) }

func setMetadataFromAttributesAST(m *meta.Meta, a zsx.Attributes) {
	for aKey, aVal := range a {
		if meta.KeyIsValid(aKey) {
			m.Set(aKey, meta.Value(aVal))
		}
	}
}

func (e *evaluator) visitInlineSliceAST(is *ast.InlineSlice) {
	for i := 0; i < len(*is); i++ {
		in := (*is)[i]
		ast.Walk(e, in)
		switch n := in.(type) {
		case *ast.EmbedRefNode:
			i += embedNodeAST(is, i, e.evalEmbedRefNodeAST(n))
		}
	}
}

func embedNodeAST(is *ast.InlineSlice, i int, in ast.InlineNode) int {
	if ln, ok := in.(*ast.InlineSlice); ok {
		*is = replaceWithInlineNodesAST(*is, i, *ln)
		return len(*ln) - 1
	}
	if in == nil {
		(*is) = (*is)[:i+copy((*is)[i:], (*is)[i+1:])]
		return -1
	}
	(*is)[i] = in
	return 0
}

func replaceWithInlineNodesAST(ins ast.InlineSlice, i int, replaceIns ast.InlineSlice) ast.InlineSlice {
	if len(replaceIns) == 1 {
		ins[i] = replaceIns[0]
		return ins
	}
	newIns := make(ast.InlineSlice, 0, len(ins)+len(replaceIns)-1)
	if i > 0 {
		newIns = append(newIns, ins[:i]...)
	}
	if len(replaceIns) > 0 {
		newIns = append(newIns, replaceIns...)
	}
	if i+1 < len(ins) {
		newIns = append(newIns, ins[i+1:]...)
	}
	return newIns
}

func (e *evaluator) evalEmbedRefNodeAST(en *ast.EmbedRefNode) ast.InlineNode {
	ref := en.Ref

	// To prevent e.embedCount from counting
	if errText := e.checkMaxTransclusionsAST(ref); errText != nil {
		return errText
	}

	switch ref.State {
	case ast.RefStateZettel:
		// Only zettel references will be evaluated.
	case ast.RefStateInvalid, ast.RefStateBroken:
		e.transcludeCount++
		return createInlineErrorImageAST(en)
	case ast.RefStateSelf:
		e.transcludeCount++
		return createInlineErrorTextAST(ref, "Self embed reference")
	case ast.RefStateFound, ast.RefStateExternal:
		return en
	case ast.RefStateHosted, ast.RefStateBased:
		if n := createEmbeddedNodeLocalAST(ref); n != nil {
			n.Attrs = en.Attrs
			n.Inlines = en.Inlines
			return n
		}
		return en
	default:
		return createInlineErrorTextAST(ref, "Illegal inline state"+strconv.Itoa(int(ref.State)))
	}

	zid := mustParseZidAST(ref)
	zettel, err := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
	if err != nil {
		if errors.Is(err, &box.ErrNotAllowed{}) {
			return nil
		}
		e.transcludeCount++
		return createInlineErrorImageAST(en)
	}

	if syntax := string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax)); parser.IsImageFormat(syntax) {
		e.updateImageRefNodeAST(en, zettel.Meta, syntax)
		return en
	} else if !parser.IsASTParser(syntax) {
		// Not embeddable.
		e.transcludeCount++
		return createInlineErrorTextAST(ref, "Not embeddable (syntax="+syntax+")")
	}

	cost, ok := e.costMap[zid]
	zn := cost.zn
	if zn == e.marker {
		e.transcludeCount++
		return createInlineErrorTextAST(ref, "Recursive transclusion")
	}
	if !ok {
		ec := e.transcludeCount
		e.costMap[zid] = transcludeCost{zn: e.marker, ec: ec}
		zn = e.evaluateEmbeddedZettelAST(zettel)
		e.costMap[zid] = transcludeCost{zn: zn, ec: e.transcludeCount - ec}
		e.transcludeCount = 0 // No stack needed, because embedding is done left-recursive, depth-first.
	}
	e.transcludeCount++

	result, ok := e.embedMapAST[ref.Value]
	if !ok {
		// Search for text to be embedded.
		result = findInlineSliceAST(&zn.BlocksAST, ref.URL.Fragment)
		e.embedMapAST[ref.Value] = result
	}
	if len(result) == 0 {
		return &ast.LiteralNode{
			Kind:    ast.LiteralComment,
			Attrs:   map[string]string{"-": ""},
			Content: append([]byte("Nothing to transclude: "), ref.String()...),
		}
	}

	if ec := cost.ec; ec > 0 {
		e.transcludeCount += cost.ec
	}
	return &result
}

func mustParseZidAST(ref *ast.Reference) id.Zid {
	zid, err := id.Parse(ref.URL.Path)
	if err != nil {
		panic(fmt.Sprintf("%v: %q (state %v) -> %v", err, ref.URL.Path, ref.State, ref))
	}
	return zid
}

func (e *evaluator) updateImageRefNodeAST(en *ast.EmbedRefNode, m *meta.Meta, syntax string) {
	en.Syntax = syntax
	if len(en.Inlines) == 0 {
		is := parseDescriptionAST(m)
		if len(is) > 0 {
			ast.Walk(e, &is)
			if len(is) > 0 {
				en.Inlines = is
			}
		}
	}
}
func parseDescriptionAST(m *meta.Meta) ast.InlineSlice {
	// Non-AST function in package parser.
	if m == nil {
		return nil
	}
	if summary, found := m.Get(meta.KeySummary); found {
		return ast.InlineSlice{&ast.TextNode{Text: sz.NormalizedSpacedText(string(summary))}}
	}
	if title, found := m.Get(meta.KeyTitle); found {
		return ast.InlineSlice{&ast.TextNode{Text: sz.NormalizedSpacedText(string(title))}}
	}
	return ast.InlineSlice{&ast.TextNode{Text: "Zettel without title/summary: " + m.Zid.String()}}
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

func createEmbeddedNodeLocalAST(ref *ast.Reference) *ast.EmbedRefNode {
	ext := path.Ext(ref.Value)
	if ext != "" && ext[0] == '.' {
		ext = ext[1:]
	}
	pinfo := parser.Get(ext)
	if pinfo == nil || !pinfo.IsImageFormat {
		return nil
	}
	return &ast.EmbedRefNode{
		Ref:    ref,
		Syntax: ext,
	}
}

func (e *evaluator) evaluateEmbeddedZettelAST(zettel zettel.Zettel) *ast.Zettel {
	zn := parser.ParseZettel(e.ctx, zettel, string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax)), e.rtConfig)
	ast.Walk(e, &zn.BlocksAST)
	return zn
}

func findInlineSliceAST(bs *ast.BlockSlice, fragment string) ast.InlineSlice {
	if fragment == "" {
		return firstInlinesToEmbedAST(*bs)
	}
	fs := fragmentSearcherAST{fragment: fragment}
	ast.Walk(&fs, bs)
	return fs.result
}

func firstInlinesToEmbedAST(bs ast.BlockSlice) ast.InlineSlice {
	if ins := bs.FirstParagraphInlines(); ins != nil {
		return ins
	}
	if len(bs) == 0 {
		return nil
	}
	if bn, ok := bs[0].(*ast.BLOBNode); ok {
		return ast.InlineSlice{&ast.EmbedBLOBNode{
			Blob:    bn.Blob,
			Syntax:  bn.Syntax,
			Inlines: bn.Description,
		}}
	}
	return nil
}

type fragmentSearcherAST struct {
	fragment string
	result   ast.InlineSlice
}

func (fs *fragmentSearcherAST) Visit(node ast.Node) ast.Visitor {
	if len(fs.result) > 0 {
		return nil
	}
	switch n := node.(type) {
	case *ast.BlockSlice:
		fs.visitBlockSliceAST(n)
	case *ast.InlineSlice:
		fs.visitInlineSliceAST(n)
	default:
		return fs
	}
	return nil
}

func (fs *fragmentSearcherAST) visitBlockSliceAST(bs *ast.BlockSlice) {
	for i, bn := range *bs {
		if hn, ok := bn.(*ast.HeadingNode); ok && hn.Fragment == fs.fragment {
			fs.result = (*bs)[i+1:].FirstParagraphInlines()
			return
		}
		ast.Walk(fs, bn)
	}
}

func (fs *fragmentSearcherAST) visitInlineSliceAST(is *ast.InlineSlice) {
	for i, in := range *is {
		if mn, ok := in.(*ast.MarkNode); ok && mn.Fragment == fs.fragment {
			ris := skipBreakeNodesAST((*is)[i+1:])
			if len(mn.Inlines) > 0 {
				fs.result = slices.Clone(mn.Inlines)
				fs.result = append(fs.result, &ast.TextNode{Text: " "})
				fs.result = append(fs.result, ris...)
			} else {
				fs.result = ris
			}
			return
		}
		ast.Walk(fs, in)
	}
}

func skipBreakeNodesAST(ins ast.InlineSlice) ast.InlineSlice {
	for i, in := range ins {
		switch in.(type) {
		case *ast.BreakNode:
		default:
			return ins[i:]
		}
	}
	return nil
}
