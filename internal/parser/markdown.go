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

package parser

// markdown provides a parser for markdown.

import (
	"bytes"
	"strconv"
	"strings"

	gm "github.com/yuin/goldmark"
	gmAst "github.com/yuin/goldmark/ast"
	gmExtension "github.com/yuin/goldmark/extension"
	gmExtAst "github.com/yuin/goldmark/extension/ast"
	gmParser "github.com/yuin/goldmark/parser"
	gmText "github.com/yuin/goldmark/text"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"
	"t73f.de/r/zsx/input"
)

func parseMarkdown(inp *input.Input, _ *meta.Meta, syntax string, alst *sx.Pair) *sx.Pair {
	parser, extended := mdParserFromSyntax(syntax)
	source := inp.Src[inp.Pos:]
	node := parser.Parse(gmText.NewReader(source))
	p := mdP{source: source, extended: extended, docNode: node, allowHTML: alst.Assoc(SymAllowHTML) != nil}
	return p.acceptBlockChildren(p.docNode)
}
func mdParserFromSyntax(syntax string) (gmParser.Parser, bool) {
	if syntax == meta.ValueSyntaxEMark {
		md := gm.New(
			gm.WithExtensions(
				gmExtension.Table,
				gmExtension.Strikethrough,
			),
		)
		return md.Parser(), true
	}
	return gm.DefaultParser(), false
}

type mdP struct {
	source    []byte
	docNode   gmAst.Node
	extended  bool
	allowHTML bool
}

func (p *mdP) acceptBlockChildren(docNode gmAst.Node) *sx.Pair {
	if docNode.Type() != gmAst.TypeDocument {
		return buildErrorText(false, "expected document, but got node type '"+docNode.Kind().String()+"'.")
	}
	var result sx.ListBuilder
	result.Add(zsx.SymBlock)
	for child := docNode.FirstChild(); child != nil; child = child.NextSibling() {
		if block := p.acceptBlock(child); block != nil {
			result.Add(block)
		}
	}
	return result.List()
}

func (p *mdP) acceptBlock(node gmAst.Node) *sx.Pair {
	if node.Type() != gmAst.TypeBlock {
		return buildErrorText(false, "expected block node, but got node type '"+strconv.Itoa((int)(node.Type()))+"'.")
	}
	switch n := node.(type) {
	case *gmAst.Paragraph:
		return p.acceptParagraph(n)
	case *gmAst.TextBlock:
		return p.acceptTextBlock(n)
	case *gmAst.Heading:
		level := min(n.Level, 5)
		return zsx.MakeHeading(nil, level, p.acceptInlineChildren(n))
	case *gmAst.ThematicBreak:
		return zsx.MakeThematic(nil /*TODO*/)
	case *gmAst.CodeBlock:
		return zsx.MakeVerbatim(zsx.SymVerbatimCode, nil /*TODO*/, string(p.acceptRawText(n)))
	case *gmAst.FencedCodeBlock:
		return p.acceptFencedCodeBlock(n)
	case *gmAst.Blockquote:
		return zsx.MakeList(zsx.SymListQuote, nil, sx.MakeList(zsx.MakeListItem(nil, p.acceptItemSlice(n))))
	case *gmAst.List:
		return p.acceptList(n)
	case *gmAst.HTMLBlock:
		return p.acceptHTMLBlock(n)
	case *gmAst.LinkReferenceDefinition:
		return nil
	}
	if p.extended {
		switch n := node.(type) {
		case *gmExtAst.Table:
			return p.acceptTable(n)
		}
	}
	return buildErrorText(false, "unhandled block node of kind '"+node.Kind().String()+"'.")
}

func (p *mdP) acceptParagraph(node *gmAst.Paragraph) *sx.Pair {
	if is := p.acceptInlineChildren(node); is != nil {
		return zsx.MakeParaList(is)
	}
	return nil
}

func (p *mdP) acceptFencedCodeBlock(node *gmAst.FencedCodeBlock) *sx.Pair {
	var a sx.ListBuilder
	if language := node.Language(p.source); len(language) > 0 {
		a.Add(sx.Cons(sx.MakeString("class"), sx.MakeString("language-"+cleanText(language, true))))
	}
	return zsx.MakeVerbatim(zsx.SymVerbatimCode, a.List(), string(p.acceptRawText(node)))
}

func (p *mdP) acceptRawText(node gmAst.Node) []byte {
	lines := node.Lines()
	result := make([]byte, 0, 512)
	for i := range lines.Len() {
		s := lines.At(i)
		line := s.Value(p.source)
		if l := len(line); l > 0 {
			if l > 1 && line[l-2] == '\r' && line[l-1] == '\n' {
				line = line[0 : l-2]
			} else if line[l-1] == '\n' || line[l-1] == '\r' {
				line = line[0 : l-1]
			}
		}
		if i > 0 {
			result = append(result, '\n')
		}
		result = append(result, line...)
	}
	return result
}

func (p *mdP) acceptList(node *gmAst.List) *sx.Pair {
	kind := zsx.SymListUnordered
	var a sx.ListBuilder
	if node.IsOrdered() {
		kind = zsx.SymListOrdered
		if node.Start != 1 {
			a.Add(sx.Cons(sx.MakeString("start"), sx.MakeString(strconv.Itoa(node.Start))))
		}
	}
	var items sx.ListBuilder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		item, ok := child.(*gmAst.ListItem)
		if !ok {
			return buildErrorText(false, "expected list item node, but got '"+child.Kind().String()+"'.")
		}
		items.Add(zsx.MakeListItem(nil, p.acceptItemSlice(item)))
	}
	return zsx.MakeList(kind, a.List(), items.List())
}

func (p *mdP) acceptItemSlice(node gmAst.Node) *sx.Pair {
	var result sx.ListBuilder
	for elem := node.FirstChild(); elem != nil; elem = elem.NextSibling() {
		if item := p.acceptBlock(elem); item != nil {
			result.Add(item)
		}
	}
	return result.List()
}

func (p *mdP) acceptTextBlock(node *gmAst.TextBlock) *sx.Pair {
	if is := p.acceptInlineChildren(node); is != nil {
		return zsx.MakeParaList(is)
	}
	return nil
}

func (p *mdP) acceptHTMLBlock(node *gmAst.HTMLBlock) *sx.Pair {
	content := p.acceptRawText(node)
	if node.HasClosure() {
		closure := node.ClosureLine.Value(p.source)
		if l := len(closure); l > 1 && closure[l-1] == '\n' {
			closure = closure[:l-1]
		}
		if len(content) > 1 {
			content = append(content, '\n')
		}
		content = append(content, closure...)
	}
	if p.allowHTML {
		return zsx.MakeVerbatim(zsx.SymVerbatimHTML, nil, string(content))
	}
	return zsx.MakeVerbatim(zsx.SymVerbatimCode, makeAttrHTML(), string(content))
}

func (p *mdP) acceptTable(table *gmExtAst.Table) *sx.Pair {
	var lb sx.ListBuilder
	child := table.FirstChild()
	if child == nil {
		return sx.Nil()
	}
	header, isHeader := child.(*gmExtAst.TableHeader)
	if !isHeader {
		return sx.Nil()
	}
	lb.AddN(zsx.SymTable, sx.Nil(), zsx.MakeRow(nil, p.acceptCells(header, header.Alignments, table.Alignments)))
	for child = child.NextSibling(); child != nil; child = child.NextSibling() {
		if row, isRow := child.(*gmExtAst.TableRow); isRow {
			lb.Add(zsx.MakeRow(nil, p.acceptCells(row, row.Alignments, table.Alignments)))
		}
	}
	return lb.List()
}
func (p *mdP) acceptCells(parent gmAst.Node, rowAligns, tabAligns []gmExtAst.Alignment) *sx.Pair {
	var lb sx.ListBuilder
	pos := 0
	for cell := parent.FirstChild(); cell != nil; cell = cell.NextSibling() {
		lb.Add(zsx.MakeCell(p.buildAlignment(rowAligns, tabAligns, pos), p.acceptInlineChildren(cell)))
		pos++
	}
	return lb.List()
}
func (*mdP) buildAlignment(rowAligns, tabAligns []gmExtAst.Alignment, pos int) *sx.Pair {
	aligns := tabAligns
	if pos < len(rowAligns) && rowAligns[pos] != gmExtAst.AlignNone {
		aligns = rowAligns
	}

	if pos < len(aligns) {
		switch aligns[pos] {
		case gmExtAst.AlignLeft:
			return sx.MakeList(sx.Cons(zsx.SymAttrAlign, zsx.AttrAlignLeft))
		case gmExtAst.AlignCenter:
			return sx.MakeList(sx.Cons(zsx.SymAttrAlign, zsx.AttrAlignCenter))
		case gmExtAst.AlignRight:
			return sx.MakeList(sx.Cons(zsx.SymAttrAlign, zsx.AttrAlignRight))
		}
	}
	return sx.Nil()
}

func (p *mdP) acceptInlineChildren(node gmAst.Node) *sx.Pair {
	var result sx.ListBuilder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		n1, n2 := p.acceptInline(child)
		if n1 != nil {
			result.Add(n1)
		}
		if n2 != nil {
			result.Add(n2)
		}
	}
	return result.List()
}

func (p *mdP) acceptInline(node gmAst.Node) (*sx.Pair, *sx.Pair) {
	if node.Type() != gmAst.TypeInline {
		return buildErrorText(true, "expected inline node, but got '"+strconv.Itoa((int)(node.Type()))+"'."), nil
	}
	switch n := node.(type) {
	case *gmAst.Text:
		return p.acceptText(n)
	case *gmAst.CodeSpan:
		return p.acceptCodeSpan(n)
	case *gmAst.Emphasis:
		return p.acceptEmphasis(n)
	case *gmAst.Link:
		return p.acceptLink(n)
	case *gmAst.Image:
		return p.acceptImage(n)
	case *gmAst.AutoLink:
		return p.acceptAutoLink(n)
	case *gmAst.RawHTML:
		return p.acceptRawHTML(n)
	}
	if p.extended {
		switch n := node.(type) {
		case *gmExtAst.Strikethrough:
			return p.acceptStrikethrough(n)
		}
	}

	return buildErrorText(true, "unhandled inline node '"+node.Kind().String()+"'."), nil
}

func buildErrorText(inline bool, msg string) *sx.Pair {
	result := zsx.MakeFormat(zsx.SymFormatMark, nil,
		sx.MakeList(
			zsx.MakeFormat(zsx.SymFormatStrong, nil,
				sx.Cons(zsx.MakeText("Error parsing markdown: "), nil)),
			zsx.MakeText(msg),
		))
	if inline {
		return result
	}
	return zsx.MakePara(result)
}

func (p *mdP) acceptText(node *gmAst.Text) (*sx.Pair, *sx.Pair) {
	segment := node.Segment
	text := segment.Value(p.source)
	if text == nil {
		return nil, nil
	}
	if node.IsRaw() {
		return zsx.MakeText(string(text)), nil
	}
	in := zsx.MakeText(cleanText(text, true))
	if node.HardLineBreak() {
		return in, zsx.MakeHard()
	}
	if node.SoftLineBreak() {
		return in, zsx.MakeSoft()
	}
	return in, nil
}

var ignoreAfterBS = map[byte]struct{}{
	'!': {}, '"': {}, '#': {}, '$': {}, '%': {}, '&': {}, '\'': {}, '(': {},
	')': {}, '*': {}, '+': {}, ',': {}, '-': {}, '.': {}, '/': {}, ':': {},
	';': {}, '<': {}, '=': {}, '>': {}, '?': {}, '@': {}, '[': {}, '\\': {},
	']': {}, '^': {}, '_': {}, '`': {}, '{': {}, '|': {}, '}': {}, '~': {},
}

// cleanText removes backslashes from TextNodes and expands entities
func cleanText(text []byte, cleanBS bool) string {
	lastPos := 0
	var sb strings.Builder
	for pos, ch := range text {
		if pos < lastPos {
			continue
		}
		if ch == '&' {
			inp := input.NewInput([]byte(text[pos:]))
			if s, ok := zsx.ScanEntity(inp); ok {
				sb.Write(text[lastPos:pos])
				sb.WriteString(s)
				lastPos = pos + inp.Pos
			}
			continue
		}
		if cleanBS && ch == '\\' && pos < len(text)-1 {
			if _, found := ignoreAfterBS[text[pos+1]]; found {
				sb.Write(text[lastPos:pos])
				sb.WriteByte(text[pos+1])
				lastPos = pos + 2
			}
		}
	}
	if lastPos < len(text) {
		sb.Write(text[lastPos:])
	}
	return sb.String()
}

func (p *mdP) acceptCodeSpan(node *gmAst.CodeSpan) (*sx.Pair, *sx.Pair) {
	var segBuf strings.Builder
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		segment := c.(*gmAst.Text).Segment
		segBuf.Write(segment.Value(p.source))
	}
	content := segBuf.String()

	// Clean code span
	if len(content) > 0 {
		lastPos := 0
		var buf strings.Builder
		for pos, ch := range content {
			if ch == '\n' {
				buf.WriteString(content[lastPos:pos])
				if pos < len(content)-1 {
					buf.WriteByte(' ')
				}
				lastPos = pos + 1
			}
		}
		buf.WriteString(content[lastPos:])
		content = buf.String()
	}
	return zsx.MakeLiteral(zsx.SymLiteralCode, nil /* TODO */, content), nil
}

func (p *mdP) acceptEmphasis(node *gmAst.Emphasis) (*sx.Pair, *sx.Pair) {
	sym := zsx.SymFormatEmph
	if node.Level == 2 {
		sym = zsx.SymFormatStrong
	}
	return zsx.MakeFormat(sym, nil /* TODO */, p.acceptInlineChildren(node)), nil
}

func (p *mdP) acceptStrikethrough(node *gmExtAst.Strikethrough) (*sx.Pair, *sx.Pair) {
	return zsx.MakeFormat(zsx.SymFormatDelete, nil /* TODO */, p.acceptInlineChildren(node)), nil
}

func (p *mdP) acceptLink(node *gmAst.Link) (*sx.Pair, *sx.Pair) {
	ref := sz.ScanReference(cleanText(node.Destination, true))
	var a sx.ListBuilder
	if title := node.Title; len(title) > 0 {
		a.Add(sx.Cons(sx.MakeString("title"), sx.MakeString(cleanText(title, true))))
	}
	return zsx.MakeLink(a.List(), ref, p.acceptInlineChildren(node)), nil
}

func (p *mdP) acceptImage(node *gmAst.Image) (*sx.Pair, *sx.Pair) {
	ref := sz.ScanReference(cleanText(node.Destination, true))
	var a sx.ListBuilder
	if title := node.Title; len(title) > 0 {
		a.Add(sx.Cons(sx.MakeString("title"), sx.MakeString(cleanText(title, true))))
	}
	return zsx.MakeEmbed(a.List(), ref, "", p.acceptInlineChildren(node)), nil
}

func (p *mdP) acceptAutoLink(node *gmAst.AutoLink) (*sx.Pair, *sx.Pair) {
	u := node.URL(p.source)
	if node.AutoLinkType == gmAst.AutoLinkEmail &&
		!bytes.HasPrefix(bytes.ToLower(u), []byte("mailto:")) {
		u = append([]byte("mailto:"), u...)
	}
	return zsx.MakeLink(nil /* TODO */, sz.ScanReference(cleanText(u, false)), nil), nil
}

func (p *mdP) acceptRawHTML(node *gmAst.RawHTML) (*sx.Pair, *sx.Pair) {
	segs := make([][]byte, 0, node.Segments.Len())
	for i := range node.Segments.Len() {
		segment := node.Segments.At(i)
		segs = append(segs, segment.Value(p.source))
	}
	return zsx.MakeLiteral(zsx.SymLiteralCode, makeAttrHTML(), string(bytes.Join(segs, nil))), nil
}

func makeAttrHTML() *sx.Pair {
	return sx.Cons(sx.Cons(sx.MakeString(""), sx.MakeString("html")), sx.Nil())
}
