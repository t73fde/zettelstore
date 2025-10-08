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

// Package sztrans allows to transform a sz representation of text into an
// abstract syntax tree and vice versa.
package sztrans

import (
	"fmt"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
)

type transformer struct{}

// GetBlockSlice returns the sz representations as a AST BlockSlice
func GetBlockSlice(pair *sx.Pair) (ast.BlockSlice, error) {
	if pair == nil {
		return nil, nil
	}
	var t transformer
	if obj := zsx.Walk(&t, pair, nil); !obj.IsNil() {
		if sxn, isNode := obj.(sxNode); isNode {
			if bs, ok := sxn.node.(*ast.BlockSlice); ok {
				return *bs, nil
			}
			return nil, fmt.Errorf("no BlockSlice AST: %T/%v for %v", sxn.node, sxn.node, pair)
		}
		return nil, fmt.Errorf("no AST for %v: %v", pair, obj)
	}
	return nil, fmt.Errorf("error walking %v", pair)
}

func (t *transformer) VisitBefore(node *sx.Pair, _ *sx.Pair) (sx.Object, bool) {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymText:
			if s := zsx.GetText(node); s != "" {
				return sxNode{&ast.TextNode{Text: s}}, true
			}
		case zsx.SymSoft:
			return sxNode{&ast.BreakNode{Hard: false}}, true
		case zsx.SymHard:
			return sxNode{&ast.BreakNode{Hard: true}}, true
		case zsx.SymLiteralCode:
			return handleLiteral(ast.LiteralCode, node)
		case zsx.SymLiteralComment:
			return handleLiteral(ast.LiteralComment, node)
		case zsx.SymLiteralInput:
			return handleLiteral(ast.LiteralInput, node)
		case zsx.SymLiteralMath:
			return handleLiteral(ast.LiteralMath, node)
		case zsx.SymLiteralOutput:
			return handleLiteral(ast.LiteralOutput, node)
		case zsx.SymThematic:
			return sxNode{&ast.HRuleNode{Attrs: zsx.GetAttributes(node.Tail().Head())}}, true
		case zsx.SymVerbatimComment:
			return handleVerbatim(ast.VerbatimComment, node)
		case zsx.SymVerbatimEval:
			return handleVerbatim(ast.VerbatimEval, node)
		case zsx.SymVerbatimHTML:
			return handleVerbatim(ast.VerbatimHTML, node)
		case zsx.SymVerbatimMath:
			return handleVerbatim(ast.VerbatimMath, node)
		case zsx.SymVerbatimCode:
			return handleVerbatim(ast.VerbatimCode, node)
		case zsx.SymVerbatimZettel:
			return handleVerbatim(ast.VerbatimZettel, node)
		}
	}
	return sx.Nil(), false
}

func handleLiteral(kind ast.LiteralKind, node *sx.Pair) (sx.Object, bool) {
	if sym, attrs, content := zsx.GetLiteral(node); sym != nil {
		return sxNode{&ast.LiteralNode{
			Kind:    kind,
			Attrs:   zsx.GetAttributes(attrs),
			Content: []byte(content)}}, true
	}
	return nil, false
}

func handleVerbatim(kind ast.VerbatimKind, node *sx.Pair) (sx.Object, bool) {
	if sym, attrs, content := zsx.GetVerbatim(node); sym != nil {
		return sxNode{&ast.VerbatimNode{
			Kind:    kind,
			Attrs:   zsx.GetAttributes(attrs),
			Content: []byte(content),
		}}, true
	}
	return nil, false
}

func (t *transformer) VisitAfter(node *sx.Pair, _ *sx.Pair) sx.Object {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymBlock:
			bns := collectBlocks(node.Tail())
			return sxNode{&bns}
		case zsx.SymPara:
			return sxNode{&ast.ParaNode{Inlines: collectInlines(node.Tail())}}
		case zsx.SymHeading:
			return handleHeading(node)
		case zsx.SymListOrdered:
			return handleList(ast.NestedListOrdered, node)
		case zsx.SymListUnordered:
			return handleList(ast.NestedListUnordered, node)
		case zsx.SymListQuote:
			return handleList(ast.NestedListQuote, node)
		case zsx.SymDescription:
			return handleDescription(node)
		case zsx.SymTable:
			return handleTable(node)
		case zsx.SymCell:
			return handleCell(node)
		case zsx.SymRegionBlock:
			return handleRegion(ast.RegionSpan, node)
		case zsx.SymRegionQuote:
			return handleRegion(ast.RegionQuote, node)
		case zsx.SymRegionVerse:
			return handleRegion(ast.RegionVerse, node)
		case zsx.SymTransclude:
			return handleTransclude(node)
		case zsx.SymBLOB:
			return handleBLOB(node)

		case zsx.SymLink:
			return handleLink(node)
		case zsx.SymEmbed:
			return handleEmbed(node)
		case zsx.SymEmbedBLOB:
			return handleEmbedBLOB(node)
		case zsx.SymCite:
			return handleCite(node)
		case zsx.SymMark:
			return handleMark(node)
		case zsx.SymEndnote:
			return handleEndnote(node)
		case zsx.SymFormatDelete:
			return handleFormat(ast.FormatDelete, node)
		case zsx.SymFormatEmph:
			return handleFormat(ast.FormatEmph, node)
		case zsx.SymFormatInsert:
			return handleFormat(ast.FormatInsert, node)
		case zsx.SymFormatMark:
			return handleFormat(ast.FormatMark, node)
		case zsx.SymFormatQuote:
			return handleFormat(ast.FormatQuote, node)
		case zsx.SymFormatSpan:
			return handleFormat(ast.FormatSpan, node)
		case zsx.SymFormatSub:
			return handleFormat(ast.FormatSub, node)
		case zsx.SymFormatSuper:
			return handleFormat(ast.FormatSuper, node)
		case zsx.SymFormatStrong:
			return handleFormat(ast.FormatStrong, node)
		}
	}
	return node
}

func collectBlocks(lst *sx.Pair) (result ast.BlockSlice) {
	for val := range lst.Values() {
		if sxn, isNode := val.(sxNode); isNode {
			if bn, isInline := sxn.node.(ast.BlockNode); isInline {
				result = append(result, bn)
			}
		}
	}
	return result
}

func collectInlines(lst *sx.Pair) (result ast.InlineSlice) {
	for val := range lst.Values() {
		if sxn, isNode := val.(sxNode); isNode {
			if in, isInline := sxn.node.(ast.InlineNode); isInline {
				result = append(result, in)
			}
		}
	}
	return result
}

func handleHeading(node *sx.Pair) sx.Object {
	if level, attrs, inlines, slug, fragment := zsx.GetHeading(node); level > 0 && level < 6 {
		return sxNode{&ast.HeadingNode{
			Level:    level,
			Attrs:    zsx.GetAttributes(attrs),
			Slug:     slug,
			Fragment: fragment,
			Inlines:  collectInlines(inlines),
		}}
	}
	return node
}

func handleList(kind ast.NestedListKind, node *sx.Pair) sx.Object {
	if sym, attrs, items := zsx.GetList(node); sym != nil {
		return sxNode{&ast.NestedListNode{
			Kind:  kind,
			Items: collectItemSlices(items),
			Attrs: zsx.GetAttributes(attrs),
		}}
	}
	return node
}
func collectItemSlices(lst *sx.Pair) (result []ast.ItemSlice) {
	for val := range lst.Values() {
		if sxn, isNode := val.(sxNode); isNode {
			if bns, isBlockSlice := sxn.node.(*ast.BlockSlice); isBlockSlice {
				items := make(ast.ItemSlice, len(*bns))
				for i, bn := range *bns {
					if it, ok := bn.(ast.ItemNode); ok {
						items[i] = it
					}
				}
				result = append(result, items)
			}
			if ins, isInline := sxn.node.(*ast.InlineSlice); isInline {
				items := make(ast.ItemSlice, len(*ins))
				for i, bn := range *ins {
					if it, ok := bn.(ast.ItemNode); ok {
						items[i] = it
					}
				}
				result = append(result, items)
			}
		}
	}
	return result
}

func handleDescription(node *sx.Pair) sx.Object {
	attrs, termsVals := zsx.GetDescription(node)

	var descs []ast.Description
	for curr := termsVals; curr != nil; {
		term := collectInlines(curr.Head())
		curr = curr.Tail()
		if curr == nil {
			descr := ast.Description{Term: term, Descriptions: nil}
			descs = append(descs, descr)
			break
		}

		car := curr.Car()
		if sx.IsNil(car) {
			descs = append(descs, ast.Description{Term: term, Descriptions: nil})
			curr = curr.Tail()
			continue
		}

		sxn, isNode := car.(sxNode)
		if !isNode {
			descs = nil
			break
		}
		blocks, isBlocks := sxn.node.(*ast.BlockSlice)
		if !isBlocks {
			descs = nil
			break
		}

		descSlice := make([]ast.DescriptionSlice, 0, len(*blocks))
		for _, bn := range *blocks {
			bns, isBns := bn.(*ast.BlockSlice)
			if !isBns {
				continue
			}
			ds := make(ast.DescriptionSlice, 0, len(*bns))
			for _, b := range *bns {
				if defNode, isDef := b.(ast.DescriptionNode); isDef {
					ds = append(ds, defNode)
				}
			}
			descSlice = append(descSlice, ds)
		}

		descr := ast.Description{Term: term, Descriptions: descSlice}
		descs = append(descs, descr)

		curr = curr.Tail()
	}
	if len(descs) > 0 {
		return sxNode{&ast.DescriptionListNode{
			Attrs:        zsx.GetAttributes(attrs),
			Descriptions: descs,
		}}
	}
	return node
}

func handleTable(node *sx.Pair) sx.Object {
	_, headerRow, rowList := zsx.GetTable(node)
	header := collectRow(headerRow)
	cols := len(header)

	var rows []ast.TableRow
	for curr := range rowList.Pairs() {
		row := collectRow(curr.Head())
		rows = append(rows, row)
		cols = max(cols, len(row))
	}
	align := make([]ast.Alignment, cols)
	for i := range cols {
		align[i] = ast.AlignDefault
	}

	return sxNode{&ast.TableNode{
		Header: header,
		Align:  align,
		Rows:   rows,
	}}
}
func collectRow(lst *sx.Pair) (row ast.TableRow) {
	for curr := range lst.Values() {
		if sxn, isNode := curr.(sxNode); isNode {
			if cell, isCell := sxn.node.(*ast.TableCell); isCell {
				row = append(row, cell)
			}
		}
	}
	return row
}

func handleCell(node *sx.Pair) sx.Object {
	attrs, inlines := zsx.GetCell(node)
	align := ast.AlignDefault
	if alignPair := attrs.Assoc(zsx.SymAttrAlign); alignPair != nil {
		if alignValue := alignPair.Cdr(); zsx.AttrAlignCenter.IsEqual(alignValue) {
			align = ast.AlignCenter
		} else if zsx.AttrAlignLeft.IsEqual(alignValue) {
			align = ast.AlignLeft
		} else if zsx.AttrAlignRight.IsEqual(alignValue) {
			align = ast.AlignRight
		}
	}
	return sxNode{&ast.TableCell{
		Align:   align,
		Inlines: collectInlines(inlines),
	}}
}

func handleRegion(kind ast.RegionKind, node *sx.Pair) sx.Object {
	if sym, attrs, blocks, inlines := zsx.GetRegion(node); sym != nil {
		return sxNode{&ast.RegionNode{
			Kind:    kind,
			Attrs:   zsx.GetAttributes(attrs),
			Blocks:  collectBlocks(blocks),
			Inlines: collectInlines(inlines),
		}}
	}
	return node
}

func handleTransclude(node *sx.Pair) sx.Object {
	if attrs, reference, inlines := zsx.GetTransclusion(node); reference != nil {
		if ref := collectReference(reference); ref != nil {
			return sxNode{&ast.TranscludeNode{
				Attrs:   zsx.GetAttributes(attrs),
				Ref:     ref,
				Inlines: collectInlines(inlines),
			}}
		}
	}
	return node
}

func handleBLOB(node *sx.Pair) sx.Object {
	if attrs, syntax, data, inlines := zsx.GetBLOB(node); data != nil {
		return sxNode{&ast.BLOBNode{
			Attrs:       zsx.GetAttributes(attrs),
			Description: collectInlines(inlines),
			Syntax:      syntax,
			Blob:        data,
		}}
	}
	return node
}

func handleLink(node *sx.Pair) sx.Object {
	if attrs, reference, inlines := zsx.GetLink(node); reference != nil {
		if ref := collectReference(reference); ref != nil {
			return sxNode{&ast.LinkNode{
				Attrs:   zsx.GetAttributes(attrs),
				Ref:     ref,
				Inlines: collectInlines(inlines),
			}}
		}
	}
	return node
}

func handleEmbed(node *sx.Pair) sx.Object {
	if attrs, reference, syntax, inlines := zsx.GetEmbed(node); reference != nil {
		if ref := collectReference(reference); ref != nil {
			return sxNode{&ast.EmbedRefNode{
				Attrs:   zsx.GetAttributes(attrs),
				Ref:     ref,
				Syntax:  syntax,
				Inlines: collectInlines(inlines),
			}}
		}
	}
	return node
}

func handleEmbedBLOB(node *sx.Pair) sx.Object {
	if attrs, syntax, data, inlines := zsx.GetEmbedBLOB(node); data != nil {
		return sxNode{&ast.EmbedBLOBNode{
			Attrs:   zsx.GetAttributes(attrs),
			Syntax:  syntax,
			Blob:    data,
			Inlines: collectInlines(inlines),
		}}
	}
	return node
}

var mapRefState = map[*sx.Symbol]ast.RefState{
	zsx.SymRefStateInvalid:  ast.RefStateInvalid,
	sz.SymRefStateZettel:    ast.RefStateZettel,
	zsx.SymRefStateSelf:     ast.RefStateSelf,
	sz.SymRefStateFound:     ast.RefStateFound,
	sz.SymRefStateBroken:    ast.RefStateBroken,
	zsx.SymRefStateHosted:   ast.RefStateHosted,
	sz.SymRefStateBased:     ast.RefStateBased,
	sz.SymRefStateQuery:     ast.RefStateQuery,
	zsx.SymRefStateExternal: ast.RefStateExternal,
}

func collectReference(node *sx.Pair) *ast.Reference {
	if sym, val := zsx.GetReference(node); sym != nil {
		ref := ast.ParseReference(val)
		ref.State = mapRefState[sym]
		return ref
	}
	return nil
}

func handleCite(node *sx.Pair) sx.Object {
	if attrs, key, inlines := zsx.GetCite(node); key != "" {
		return sxNode{&ast.CiteNode{
			Attrs:   zsx.GetAttributes(attrs),
			Key:     key,
			Inlines: collectInlines(inlines),
		}}
	}
	return node
}

func handleMark(node *sx.Pair) sx.Object {
	if mark, slug, fragment, inlines := zsx.GetMark(node); mark != "" {
		return sxNode{&ast.MarkNode{
			Mark:     mark,
			Slug:     slug,
			Fragment: fragment,
			Inlines:  collectInlines(inlines),
		}}
	}
	return node
}

func handleEndnote(node *sx.Pair) sx.Object {
	if attrs, inlines := zsx.GetEndnote(node); inlines != nil {
		return sxNode{&ast.FootnoteNode{
			Attrs:   zsx.GetAttributes(attrs),
			Inlines: collectInlines(inlines),
		}}
	}
	return node
}

func handleFormat(kind ast.FormatKind, node *sx.Pair) sx.Object {
	if sym, attrs, inlines := zsx.GetFormat(node); sym != nil {
		return sxNode{&ast.FormatNode{
			Kind:    kind,
			Attrs:   zsx.GetAttributes(attrs),
			Inlines: collectInlines(inlines),
		}}
	}
	return node
}

type sxNode struct {
	node ast.Node
}

func (sxNode) IsNil() bool        { return false }
func (sxNode) IsAtom() bool       { return true }
func (n sxNode) String() string   { return fmt.Sprintf("%T/%v", n.node, n.node) }
func (n sxNode) GoString() string { return n.String() }
func (n sxNode) IsEqual(other sx.Object) bool {
	return n.String() == other.String()
}
