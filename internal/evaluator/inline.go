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
	"errors"
	"path"
	"slices"
	"strconv"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/parser"
)

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

func (e *evaluator) evalEmbed(en *sx.Pair) *sx.Pair { return en }

// ----------------------------------------------------------------------------
// AST-based evaluation. Deprecated.

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
