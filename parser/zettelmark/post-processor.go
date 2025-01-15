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

package zettelmark

import (
	"strings"

	"zettelstore.de/z/ast"
)

// postProcessBlocks is the entry point for post-processing a list of block nodes.
func postProcessBlocks(bs *ast.BlockSlice) {
	pp := postProcessor{}
	ast.Walk(&pp, bs)
}

// postProcessor is a visitor that cleans the abstract syntax tree.
type postProcessor struct {
	inVerse bool
}

func (pp *postProcessor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.BlockSlice:
		pp.visitBlockSlice(n)
	case *ast.InlineSlice:
		pp.visitInlineSlice(n)
	case *ast.ParaNode:
		return pp
	case *ast.RegionNode:
		pp.visitRegion(n)
	case *ast.HeadingNode:
		return pp
	case *ast.NestedListNode:
		pp.visitNestedList(n)
	case *ast.DescriptionListNode:
		pp.visitDescriptionList(n)
	case *ast.TableNode:
		pp.visitTable(n)
	case *ast.LinkNode:
		return pp
	case *ast.EmbedRefNode:
		return pp
	case *ast.EmbedBLOBNode:
		return pp
	case *ast.CiteNode:
		return pp
	case *ast.FootnoteNode:
		return pp
	case *ast.FormatNode:
		return pp
	}
	return nil
}

func (pp *postProcessor) visitRegion(rn *ast.RegionNode) {
	oldVerse := pp.inVerse
	if rn.Kind == ast.RegionVerse {
		pp.inVerse = true
	}
	pp.visitBlockSlice(&rn.Blocks)
	pp.inVerse = oldVerse
	if len(rn.Inlines) > 0 {
		pp.visitInlineSlice(&rn.Inlines)
	}
}

func (pp *postProcessor) visitNestedList(ln *ast.NestedListNode) {
	for i, item := range ln.Items {
		ln.Items[i] = pp.processItemSlice(item)
	}
	if ln.Kind != ast.NestedListQuote {
		return
	}
	items := []ast.ItemSlice{}
	collectedInlines := ast.InlineSlice{}

	addCollectedParagraph := func() {
		if len(collectedInlines) > 1 {
			items = append(items, []ast.ItemNode{&ast.ParaNode{Inlines: collectedInlines[1:]}})
			collectedInlines = ast.InlineSlice{}
		}
	}

	for _, item := range ln.Items {
		if len(item) != 1 { // i.e. 0 or > 1
			addCollectedParagraph()
			items = append(items, item)
			continue
		}

		// len(item) == 1
		if pn, ok := item[0].(*ast.ParaNode); ok {
			collectedInlines = append(collectedInlines, &ast.BreakNode{})
			collectedInlines = append(collectedInlines, pn.Inlines...)
			continue
		}

		addCollectedParagraph()
		items = append(items, item)
	}
	addCollectedParagraph()
	ln.Items = items
}

func (pp *postProcessor) visitDescriptionList(dn *ast.DescriptionListNode) {
	for i, def := range dn.Descriptions {
		if len(def.Term) > 0 {
			ast.Walk(pp, &dn.Descriptions[i].Term)
		}
		for j, b := range def.Descriptions {
			dn.Descriptions[i].Descriptions[j] = pp.processDescriptionSlice(b)
		}
	}
}

func (pp *postProcessor) visitTable(tn *ast.TableNode) {
	width := tableWidth(tn)
	tn.Align = make([]ast.Alignment, width)
	for i := range width {
		tn.Align[i] = ast.AlignDefault
	}
	if len(tn.Rows) > 0 && isHeaderRow(tn.Rows[0]) {
		tn.Header = tn.Rows[0]
		tn.Rows = tn.Rows[1:]
		pp.visitTableHeader(tn)
	}
	if len(tn.Header) > 0 {
		tn.Header = appendCells(tn.Header, width, tn.Align)
		for i, cell := range tn.Header {
			pp.processCell(cell, tn.Align[i])
		}
	}
	pp.visitTableRows(tn, width)
}

func (*postProcessor) visitTableHeader(tn *ast.TableNode) {
	for pos, cell := range tn.Header {
		ins := cell.Inlines
		if len(ins) == 0 {
			continue
		}
		if textNode, ok := ins[0].(*ast.TextNode); ok {
			textNode.Text = strings.TrimPrefix(textNode.Text, "=")
		}
		if textNode, ok := ins[len(ins)-1].(*ast.TextNode); ok {
			if tnl := len(textNode.Text); tnl > 0 {
				if align := getAlignment(textNode.Text[tnl-1]); align != ast.AlignDefault {
					tn.Align[pos] = align
					textNode.Text = textNode.Text[0 : tnl-1]
				}
			}
		}
	}
}

func (pp *postProcessor) visitTableRows(tn *ast.TableNode, width int) {
	for i, row := range tn.Rows {
		tn.Rows[i] = appendCells(row, width, tn.Align)
		row = tn.Rows[i]
		for i, cell := range row {
			pp.processCell(cell, tn.Align[i])
		}
	}

}

func tableWidth(tn *ast.TableNode) int {
	width := 0
	for _, row := range tn.Rows {
		if width < len(row) {
			width = len(row)
		}
	}
	return width
}

func appendCells(row ast.TableRow, width int, colAlign []ast.Alignment) ast.TableRow {
	for len(row) < width {
		row = append(row, &ast.TableCell{
			Align:   colAlign[len(row)],
			Inlines: nil,
		})
	}
	return row
}

func isHeaderRow(row ast.TableRow) bool {
	for _, cell := range row {
		if is := cell.Inlines; len(is) > 0 {
			if textNode, ok := is[0].(*ast.TextNode); ok {
				if strings.HasPrefix(textNode.Text, "=") {
					return true
				}
			}
		}
	}
	return false
}

func getAlignment(ch byte) ast.Alignment {
	switch ch {
	case ':':
		return ast.AlignCenter
	case '<':
		return ast.AlignLeft
	case '>':
		return ast.AlignRight
	default:
		return ast.AlignDefault
	}
}

// processCell tries to recognize cell formatting.
func (pp *postProcessor) processCell(cell *ast.TableCell, colAlign ast.Alignment) {
	if tn := initialText(cell.Inlines); tn != nil {
		align := getAlignment(tn.Text[0])
		if align == ast.AlignDefault {
			cell.Align = colAlign
		} else {
			tn.Text = tn.Text[1:]
			cell.Align = align
		}
	} else {
		cell.Align = colAlign
	}
	ast.Walk(pp, &cell.Inlines)
}

func initialText(ins ast.InlineSlice) *ast.TextNode {
	if len(ins) == 0 {
		return nil
	}
	if tn, ok := ins[0].(*ast.TextNode); ok && len(tn.Text) > 0 {
		return tn
	}
	return nil
}

func (pp *postProcessor) visitBlockSlice(bs *ast.BlockSlice) {
	if bs == nil {
		return
	}
	if len(*bs) == 0 {
		*bs = nil
		return
	}
	for _, bn := range *bs {
		ast.Walk(pp, bn)
	}
	fromPos, toPos := 0, 0
	for fromPos < len(*bs) {
		(*bs)[toPos] = (*bs)[fromPos]
		fromPos++
		switch bn := (*bs)[toPos].(type) {
		case *ast.ParaNode:
			if len(bn.Inlines) > 0 {
				toPos++
			}
		case *nullItemNode:
		case *nullDescriptionNode:
		default:
			toPos++
		}
	}
	for pos := toPos; pos < len(*bs); pos++ {
		(*bs)[pos] = nil // Allow excess nodes to be garbage collected.
	}
	*bs = (*bs)[:toPos:toPos]
}

// processItemSlice post-processes a slice of items.
// It is one of the working horses for post-processing.
func (pp *postProcessor) processItemSlice(ins ast.ItemSlice) ast.ItemSlice {
	if len(ins) == 0 {
		return nil
	}
	for _, in := range ins {
		ast.Walk(pp, in)
	}
	fromPos, toPos := 0, 0
	for fromPos < len(ins) {
		ins[toPos] = ins[fromPos]
		fromPos++
		switch in := ins[toPos].(type) {
		case *ast.ParaNode:
			if in != nil && len(in.Inlines) > 0 {
				toPos++
			}
		case *nullItemNode:
		case *nullDescriptionNode:
		default:
			toPos++
		}
	}
	for pos := toPos; pos < len(ins); pos++ {
		ins[pos] = nil // Allow excess nodes to be garbage collected.
	}
	return ins[:toPos:toPos]
}

// processDescriptionSlice post-processes a slice of descriptions.
// It is one of the working horses for post-processing.
func (pp *postProcessor) processDescriptionSlice(dns ast.DescriptionSlice) ast.DescriptionSlice {
	if len(dns) == 0 {
		return nil
	}
	for _, dn := range dns {
		ast.Walk(pp, dn)
	}
	fromPos, toPos := 0, 0
	for fromPos < len(dns) {
		dns[toPos] = dns[fromPos]
		fromPos++
		switch dn := dns[toPos].(type) {
		case *ast.ParaNode:
			if len(dn.Inlines) > 0 {
				toPos++
			}
		case *nullDescriptionNode:
		default:
			toPos++
		}
	}
	for pos := toPos; pos < len(dns); pos++ {
		dns[pos] = nil // Allow excess nodes to be garbage collected.
	}
	return dns[:toPos:toPos]
}

func (pp *postProcessor) visitInlineSlice(is *ast.InlineSlice) {
	if is == nil {
		return
	}
	if len(*is) == 0 {
		*is = nil
		return
	}
	for _, in := range *is {
		ast.Walk(pp, in)
	}

	pp.processInlineSliceHead(is)
	toPos := pp.processInlineSliceCopy(is)
	toPos = pp.processInlineSliceTail(is, toPos)
	*is = (*is)[:toPos:toPos]
}

// processInlineSliceHead removes leading spaces and empty text.
func (pp *postProcessor) processInlineSliceHead(is *ast.InlineSlice) {
	ins := *is
	for i, in := range ins {
		switch in := in.(type) {
		case *ast.TextNode:
			if pp.inVerse && len(in.Text) > 0 {
				*is = ins[i:]
				return
			}
			for len(in.Text) > 0 {
				if ch := in.Text[0]; ch != ' ' && ch != '\t' {
					break
				}
				in.Text = in.Text[1:]
			}
			if len(in.Text) > 0 {
				*is = ins[i:]
				return
			}
		default:
			*is = ins[i:]
			return
		}
	}
	*is = ins[0:0]
}

// processInlineSliceCopy goes forward through the slice and tries to eliminate
// elements that follow the current element.
//
// Two text nodes are merged into one.
//
// Two spaces following a break are merged into a hard break.
func (pp *postProcessor) processInlineSliceCopy(is *ast.InlineSlice) int {
	ins := *is
	maxPos := len(ins)
	toPos := pp.processInlineSliceCopyLoop(is, maxPos)
	for pos := toPos; pos < maxPos; pos++ {
		ins[pos] = nil // Allow excess nodes to be garbage collected.
	}
	return toPos
}

func (pp *postProcessor) processInlineSliceCopyLoop(is *ast.InlineSlice, maxPos int) int {
	ins := *is
	fromPos, toPos := 0, 0
	for fromPos < maxPos {
		ins[toPos] = ins[fromPos]
		fromPos++
		switch in := ins[toPos].(type) {
		case *ast.TextNode:
			// Merge following TextNodes
			for fromPos < maxPos {
				if tn, ok := ins[fromPos].(*ast.TextNode); ok {
					in.Text = in.Text + tn.Text
					fromPos++
				} else {
					break
				}
			}
			if in.Text == "" {
				continue
			}
			if ch := in.Text[len(in.Text)-1]; ch == ' ' && fromPos < maxPos {
				switch nn := ins[fromPos].(type) {
				case *ast.BreakNode:
					nn.Hard = true
					in.Text = removeTrailingSpaces(in.Text)
				case *ast.LiteralNode:
					if nn.Kind == ast.LiteralComment {
						in.Text = removeTrailingSpaces(in.Text)
					}
				}
			}
			if pp.inVerse {
				in.Text = strings.ReplaceAll(in.Text, " ", "\u00a0")
			}
		case *ast.BreakNode:
			if pp.inVerse {
				in.Hard = true
			}
		}
		toPos++
	}
	return toPos
}

// processInlineSliceTail removes empty text nodes, breaks and spaces at the end.
func (*postProcessor) processInlineSliceTail(is *ast.InlineSlice, toPos int) int {
	ins := *is
	for toPos > 0 {
		switch n := ins[toPos-1].(type) {
		case *ast.TextNode:
			n.Text = removeTrailingSpaces(n.Text)
			if len(n.Text) > 0 {
				return toPos
			}
		case *ast.BreakNode:
		default:
			return toPos
		}
		toPos--
		ins[toPos] = nil // Kill node to enable garbage collection
	}
	return toPos
}

func removeTrailingSpaces(s string) string {
	for len(s) > 0 {
		if ch := s[len(s)-1]; ch != ' ' && ch != '\t' {
			return s
		}
		s = s[0 : len(s)-1]
	}
	return ""
}
