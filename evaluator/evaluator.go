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
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"t73f.de/r/sx/sxbuiltins"
	"t73f.de/r/sx/sxreader"
	"t73f.de/r/zsc/attrs"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/box"
	"zettelstore.de/z/config"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/parser/cleaner"
	"zettelstore.de/z/parser/draw"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel"
)

// Port contains all methods to retrieve zettel (or part of it) to evaluate a zettel.
type Port interface {
	GetZettel(context.Context, id.Zid) (zettel.Zettel, error)
	QueryMeta(ctx context.Context, q *query.Query) ([]*meta.Meta, error)
}

// EvaluateZettel evaluates the given zettel in the given context, with the
// given ports, and the given environment.
func EvaluateZettel(ctx context.Context, port Port, rtConfig config.Config, zn *ast.ZettelNode) {
	switch zn.Syntax {
	case meta.ValueSyntaxNone:
		// AST is empty, evaluate to a description list of metadata.
		zn.Ast = evaluateMetadata(zn.Meta)
	case meta.ValueSyntaxSxn:
		zn.Ast = evaluateSxn(zn.Ast)
	default:
		EvaluateBlock(ctx, port, rtConfig, &zn.Ast)
	}
}

func evaluateSxn(bs ast.BlockSlice) ast.BlockSlice {
	// Check for structure made in parser/plain/plain.go:parseSxnBlocks
	if len(bs) == 1 {
		// If len(bs) > 1 --> an error was found during parsing
		if vn, isVerbatim := bs[0].(*ast.VerbatimNode); isVerbatim && vn.Kind == ast.VerbatimCode {
			if classAttr, hasClass := vn.Attrs.Get(""); hasClass && classAttr == meta.ValueSyntaxSxn {
				rd := sxreader.MakeReader(bytes.NewReader(vn.Content))
				if objs, err := rd.ReadAll(); err == nil {
					result := make(ast.BlockSlice, len(objs))
					for i, obj := range objs {
						var buf bytes.Buffer
						sxbuiltins.Print(&buf, obj)
						result[i] = &ast.VerbatimNode{
							Kind:    ast.VerbatimCode,
							Attrs:   attrs.Attributes{"": classAttr},
							Content: buf.Bytes(),
						}
					}
					return result
				}
			}
		}
	}
	return bs
}

// EvaluateBlock evaluates the given block list in the given context, with
// the given ports, and the given environment.
func EvaluateBlock(ctx context.Context, port Port, rtConfig config.Config, bns *ast.BlockSlice) {
	evaluateNode(ctx, port, rtConfig, bns)
	cleaner.CleanBlockSlice(bns, true)
}

// EvaluateInline evaluates the given inline list in the given context, with
// the given ports, and the given environment.
func EvaluateInline(ctx context.Context, port Port, rtConfig config.Config, is *ast.InlineSlice) {
	evaluateNode(ctx, port, rtConfig, is)
	cleaner.CleanInlineSlice(is)
}

func evaluateNode(ctx context.Context, port Port, rtConfig config.Config, n ast.Node) {
	e := evaluator{
		ctx:             ctx,
		port:            port,
		rtConfig:        rtConfig,
		transcludeMax:   rtConfig.GetMaxTransclusions(),
		transcludeCount: 0,
		costMap:         map[id.Zid]transcludeCost{},
		embedMap:        map[string]ast.InlineSlice{},
		marker:          &ast.ZettelNode{},
	}
	ast.Walk(&e, n)
}

type evaluator struct {
	ctx             context.Context
	port            Port
	rtConfig        config.Config
	transcludeMax   int
	transcludeCount int
	costMap         map[id.Zid]transcludeCost
	marker          *ast.ZettelNode
	embedMap        map[string]ast.InlineSlice
}

type transcludeCost struct {
	zn *ast.ZettelNode
	ec int
}

func (e *evaluator) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.BlockSlice:
		e.visitBlockSlice(n)
	case *ast.InlineSlice:
		e.visitInlineSlice(n)
	default:
		return e
	}
	return nil
}

func (e *evaluator) visitBlockSlice(bs *ast.BlockSlice) {
	for i := 0; i < len(*bs); i++ {
		bn := (*bs)[i]
		ast.Walk(e, bn)
		switch n := bn.(type) {
		case *ast.VerbatimNode:
			i += transcludeNode(bs, i, e.evalVerbatimNode(n))
		case *ast.TranscludeNode:
			i += transcludeNode(bs, i, e.evalTransclusionNode(n))
		}
	}
}

func transcludeNode(bln *ast.BlockSlice, i int, bn ast.BlockNode) int {
	if ln, ok := bn.(*ast.BlockSlice); ok {
		*bln = replaceWithBlockNodes(*bln, i, *ln)
		return len(*ln) - 1
	}
	if bn == nil {
		(*bln) = (*bln)[:i+copy((*bln)[i:], (*bln)[i+1:])]
		return -1
	}
	(*bln)[i] = bn
	return 0
}

func replaceWithBlockNodes(bns []ast.BlockNode, i int, replaceBns []ast.BlockNode) []ast.BlockNode {
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

func (e *evaluator) evalVerbatimNode(vn *ast.VerbatimNode) ast.BlockNode {
	switch vn.Kind {
	case ast.VerbatimZettel:
		return e.evalVerbatimZettel(vn)
	case ast.VerbatimEval:
		if syntax, found := vn.Attrs.Get(""); found && syntax == meta.ValueSyntaxDraw {
			return draw.ParseDrawBlock(vn)
		}
	}
	return vn
}

func (e *evaluator) evalVerbatimZettel(vn *ast.VerbatimNode) ast.BlockNode {
	m := meta.New(id.Invalid)
	m.Set(meta.KeySyntax, getSyntax(vn.Attrs, meta.ValueSyntaxText))
	zettel := zettel.Zettel{
		Meta:    m,
		Content: zettel.NewContent(vn.Content),
	}
	e.transcludeCount++
	zn := e.evaluateEmbeddedZettel(zettel)
	return &zn.Ast
}

func getSyntax(a attrs.Attributes, defSyntax meta.Value) meta.Value {
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

func (e *evaluator) evalTransclusionNode(tn *ast.TranscludeNode) ast.BlockNode {
	ref := tn.Ref

	// To prevent e.embedCount from counting
	if errText := e.checkMaxTransclusions(ref); errText != nil {
		return makeBlockNode(errText)
	}
	switch ref.State {
	case ast.RefStateZettel:
		// Only zettel references will be evaluated.
	case ast.RefStateInvalid, ast.RefStateBroken:
		e.transcludeCount++
		return makeBlockNode(createInlineErrorText(ref, "Invalid", "or", "broken", "transclusion", "reference"))
	case ast.RefStateSelf:
		e.transcludeCount++
		return makeBlockNode(createInlineErrorText(ref, "Self", "transclusion", "reference"))
	case ast.RefStateFound, ast.RefStateExternal:
		return tn
	case ast.RefStateHosted, ast.RefStateBased:
		if n := createEmbeddedNodeLocal(ref); n != nil {
			n.Attrs = tn.Attrs
			return makeBlockNode(n)
		}
		return tn
	case ast.RefStateQuery:
		e.transcludeCount++
		return e.evalQueryTransclusion(tn.Ref.Value)
	default:
		return makeBlockNode(createInlineErrorText(ref, "Illegal", "block", "state", strconv.Itoa(int(ref.State))))
	}

	zid, err := id.Parse(ref.URL.Path)
	if err != nil {
		panic(err)
	}

	cost, ok := e.costMap[zid]
	zn := cost.zn
	if zn == e.marker {
		e.transcludeCount++
		return makeBlockNode(createInlineErrorText(ref, "Recursive", "transclusion"))
	}
	if !ok {
		zettel, err1 := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
		if err1 != nil {
			if errors.Is(err1, &box.ErrNotAllowed{}) {
				return nil
			}
			e.transcludeCount++
			return makeBlockNode(createInlineErrorText(ref, "Unable", "to", "get", "zettel"))
		}
		setMetadataFromAttributes(zettel.Meta, tn.Attrs)
		ec := e.transcludeCount
		e.costMap[zid] = transcludeCost{zn: e.marker, ec: ec}
		zn = e.evaluateEmbeddedZettel(zettel)
		e.costMap[zid] = transcludeCost{zn: zn, ec: e.transcludeCount - ec}
		e.transcludeCount = 0 // No stack needed, because embedding is done left-recursive, depth-first.
	}
	e.transcludeCount++
	if ec := cost.ec; ec > 0 {
		e.transcludeCount += cost.ec
	}
	return &zn.Ast
}

func (e *evaluator) evalQueryTransclusion(expr string) ast.BlockNode {
	q := query.Parse(expr)
	ml, err := e.port.QueryMeta(e.ctx, q)
	if err != nil {
		if errors.Is(err, &box.ErrNotAllowed{}) {
			return nil
		}
		return makeBlockNode(createInlineErrorText(nil, "Unable", "to", "search", "zettel"))
	}
	result, _ := QueryAction(e.ctx, q, ml)
	if result != nil {
		ast.Walk(e, result)
	}
	return result
}

func (e *evaluator) checkMaxTransclusions(ref *ast.Reference) ast.InlineNode {
	if maxTrans := e.transcludeMax; e.transcludeCount > maxTrans {
		e.transcludeCount = maxTrans + 1
		return createInlineErrorText(ref,
			"Too", "many", "transclusions", "(must", "be", "at", "most", strconv.Itoa(maxTrans)+",",
			"see", "runtime", "configuration", "key", "max-transclusions)")
	}
	return nil
}

func makeBlockNode(in ast.InlineNode) ast.BlockNode { return ast.CreateParaNode(in) }

func setMetadataFromAttributes(m *meta.Meta, a attrs.Attributes) {
	for aKey, aVal := range a {
		if meta.KeyIsValid(aKey) {
			m.Set(aKey, meta.Value(aVal))
		}
	}
}

func (e *evaluator) visitInlineSlice(is *ast.InlineSlice) {
	for i := 0; i < len(*is); i++ {
		in := (*is)[i]
		ast.Walk(e, in)
		switch n := in.(type) {
		case *ast.LinkNode:
			(*is)[i] = e.evalLinkNode(n)
		case *ast.EmbedRefNode:
			i += embedNode(is, i, e.evalEmbedRefNode(n))
		}
	}
}

func embedNode(is *ast.InlineSlice, i int, in ast.InlineNode) int {
	if ln, ok := in.(*ast.InlineSlice); ok {
		*is = replaceWithInlineNodes(*is, i, *ln)
		return len(*ln) - 1
	}
	if in == nil {
		(*is) = (*is)[:i+copy((*is)[i:], (*is)[i+1:])]
		return -1
	}
	(*is)[i] = in
	return 0
}

func replaceWithInlineNodes(ins ast.InlineSlice, i int, replaceIns ast.InlineSlice) ast.InlineSlice {
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

func (e *evaluator) evalLinkNode(ln *ast.LinkNode) ast.InlineNode {
	if len(ln.Inlines) == 0 {
		ln.Inlines = ast.InlineSlice{&ast.TextNode{Text: ln.Ref.Value}}
	}
	ref := ln.Ref
	if ref == nil || ref.State != ast.RefStateZettel {
		return ln
	}

	zid := mustParseZid(ref)
	_, err := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
	if errors.Is(err, &box.ErrNotAllowed{}) {
		return &ast.FormatNode{
			Kind:    ast.FormatSpan,
			Attrs:   ln.Attrs,
			Inlines: getLinkInline(ln),
		}
	} else if err != nil {
		ln.Ref.State = ast.RefStateBroken
		return ln
	}

	ln.Ref.State = ast.RefStateZettel
	return ln
}

func getLinkInline(ln *ast.LinkNode) ast.InlineSlice {
	if ln.Inlines != nil {
		return ln.Inlines
	}
	return ast.InlineSlice{&ast.TextNode{Text: ln.Ref.Value}}
}

func (e *evaluator) evalEmbedRefNode(en *ast.EmbedRefNode) ast.InlineNode {
	ref := en.Ref

	// To prevent e.embedCount from counting
	if errText := e.checkMaxTransclusions(ref); errText != nil {
		return errText
	}

	switch ref.State {
	case ast.RefStateZettel:
		// Only zettel references will be evaluated.
	case ast.RefStateInvalid, ast.RefStateBroken:
		e.transcludeCount++
		return createInlineErrorImage(en)
	case ast.RefStateSelf:
		e.transcludeCount++
		return createInlineErrorText(ref, "Self", "embed", "reference")
	case ast.RefStateFound, ast.RefStateExternal:
		return en
	case ast.RefStateHosted, ast.RefStateBased:
		if n := createEmbeddedNodeLocal(ref); n != nil {
			n.Attrs = en.Attrs
			n.Inlines = en.Inlines
			return n
		}
		return en
	default:
		return createInlineErrorText(ref, "Illegal", "inline", "state", strconv.Itoa(int(ref.State)))
	}

	zid := mustParseZid(ref)
	zettel, err := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
	if err != nil {
		if errors.Is(err, &box.ErrNotAllowed{}) {
			return nil
		}
		e.transcludeCount++
		return createInlineErrorImage(en)
	}

	if syntax := string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax)); parser.IsImageFormat(syntax) {
		e.updateImageRefNode(en, zettel.Meta, syntax)
		return en
	} else if !parser.IsASTParser(syntax) {
		// Not embeddable.
		e.transcludeCount++
		return createInlineErrorText(ref, "Not", "embeddable (syntax="+syntax+")")
	}

	cost, ok := e.costMap[zid]
	zn := cost.zn
	if zn == e.marker {
		e.transcludeCount++
		return createInlineErrorText(ref, "Recursive", "transclusion")
	}
	if !ok {
		ec := e.transcludeCount
		e.costMap[zid] = transcludeCost{zn: e.marker, ec: ec}
		zn = e.evaluateEmbeddedZettel(zettel)
		e.costMap[zid] = transcludeCost{zn: zn, ec: e.transcludeCount - ec}
		e.transcludeCount = 0 // No stack needed, because embedding is done left-recursive, depth-first.
	}
	e.transcludeCount++

	result, ok := e.embedMap[ref.Value]
	if !ok {
		// Search for text to be embedded.
		result = findInlineSlice(&zn.Ast, ref.URL.Fragment)
		e.embedMap[ref.Value] = result
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

func mustParseZid(ref *ast.Reference) id.Zid {
	zid, err := id.Parse(ref.URL.Path)
	if err != nil {
		panic(fmt.Sprintf("%v: %q (state %v) -> %v", err, ref.URL.Path, ref.State, ref))
	}
	return zid
}

func (e *evaluator) updateImageRefNode(en *ast.EmbedRefNode, m *meta.Meta, syntax string) {
	en.Syntax = syntax
	if len(en.Inlines) == 0 {
		is := parser.ParseDescription(m)
		if len(is) > 0 {
			ast.Walk(e, &is)
			if len(is) > 0 {
				en.Inlines = is
			}
		}
	}
}

func createInlineErrorImage(en *ast.EmbedRefNode) *ast.EmbedRefNode {
	errorZid := id.ZidEmoji
	en.Ref = ast.ParseReference(errorZid.String())
	if len(en.Inlines) == 0 {
		en.Inlines = ast.InlineSlice{&ast.TextNode{Text: "Error placeholder"}}
	}
	return en
}

func createInlineErrorText(ref *ast.Reference, msgWords ...string) ast.InlineNode {
	text := strings.Join(msgWords, " ")
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

func createEmbeddedNodeLocal(ref *ast.Reference) *ast.EmbedRefNode {
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

func (e *evaluator) evaluateEmbeddedZettel(zettel zettel.Zettel) *ast.ZettelNode {
	zn := parser.ParseZettel(e.ctx, zettel, string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax)), e.rtConfig)
	ast.Walk(e, &zn.Ast)
	return zn
}

func findInlineSlice(bs *ast.BlockSlice, fragment string) ast.InlineSlice {
	if fragment == "" {
		return firstInlinesToEmbed(*bs)
	}
	fs := fragmentSearcher{fragment: fragment}
	ast.Walk(&fs, bs)
	return fs.result
}

func firstInlinesToEmbed(bs ast.BlockSlice) ast.InlineSlice {
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

type fragmentSearcher struct {
	fragment string
	result   ast.InlineSlice
}

func (fs *fragmentSearcher) Visit(node ast.Node) ast.Visitor {
	if len(fs.result) > 0 {
		return nil
	}
	switch n := node.(type) {
	case *ast.BlockSlice:
		fs.visitBlockSlice(n)
	case *ast.InlineSlice:
		fs.visitInlineSlice(n)
	default:
		return fs
	}
	return nil
}

func (fs *fragmentSearcher) visitBlockSlice(bs *ast.BlockSlice) {
	for i, bn := range *bs {
		if hn, ok := bn.(*ast.HeadingNode); ok && hn.Fragment == fs.fragment {
			fs.result = (*bs)[i+1:].FirstParagraphInlines()
			return
		}
		ast.Walk(fs, bn)
	}
}

func (fs *fragmentSearcher) visitInlineSlice(is *ast.InlineSlice) {
	for i, in := range *is {
		if mn, ok := in.(*ast.MarkNode); ok && mn.Fragment == fs.fragment {
			ris := skipBreakeNodes((*is)[i+1:])
			if len(mn.Inlines) > 0 {
				fs.result = append(ast.InlineSlice{}, mn.Inlines...)
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

func skipBreakeNodes(ins ast.InlineSlice) ast.InlineSlice {
	for i, in := range ins {
		switch in.(type) {
		case *ast.BreakNode:
		default:
			return ins[i:]
		}
	}
	return nil
}
