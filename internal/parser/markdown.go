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
	"fmt"
	"strconv"
	"strings"

	gm "github.com/yuin/goldmark"
	gmAst "github.com/yuin/goldmark/ast"
	gmText "github.com/yuin/goldmark/text"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"
	"t73f.de/r/zsc/sz"

	"zettelstore.de/z/internal/encoder"
)

func init() {
	register(&Info{
		Name:          meta.ValueSyntaxMarkdown,
		AltNames:      []string{meta.ValueSyntaxMD},
		IsASTParser:   true,
		IsTextFormat:  true,
		IsImageFormat: false,
		Parse:         parseMarkdown,
	})
}

func parseMarkdown(inp *input.Input, _ *meta.Meta, _ string) *sx.Pair {
	source := []byte(inp.Src[inp.Pos:])
	parser := gm.DefaultParser()
	node := parser.Parse(gmText.NewReader(source))
	textEnc := encoder.Create(api.EncoderText, nil)
	p := mdP{source: source, docNode: node, textEnc: textEnc}
	return p.acceptBlockChildren(p.docNode)
}

type mdP struct {
	source  []byte
	docNode gmAst.Node
	textEnc encoder.Encoder
}

func (p *mdP) acceptBlockChildren(docNode gmAst.Node) *sx.Pair {
	if docNode.Type() != gmAst.TypeDocument {
		panic(fmt.Sprintf("Expected document, but got node type %v", docNode.Type()))
	}
	var result sx.ListBuilder
	result.Add(sz.SymBlock)
	for child := docNode.FirstChild(); child != nil; child = child.NextSibling() {
		if block := p.acceptBlock(child); block != nil {
			result.Add(block)
		}
	}
	return result.List()
}

func (p *mdP) acceptBlock(node gmAst.Node) *sx.Pair {
	if node.Type() != gmAst.TypeBlock {
		panic(fmt.Sprintf("Expected block node, but got node type %v", node.Type()))
	}
	switch n := node.(type) {
	case *gmAst.Paragraph:
		return p.acceptParagraph(n)
	case *gmAst.TextBlock:
		return p.acceptTextBlock(n)
	case *gmAst.Heading:
		return p.acceptHeading(n)
	case *gmAst.ThematicBreak:
		return p.acceptThematicBreak()
	case *gmAst.CodeBlock:
		return p.acceptCodeBlock(n)
	case *gmAst.FencedCodeBlock:
		return p.acceptFencedCodeBlock(n)
	case *gmAst.Blockquote:
		return p.acceptBlockquote(n)
	case *gmAst.List:
		return p.acceptList(n)
	case *gmAst.HTMLBlock:
		return p.acceptHTMLBlock(n)
	}
	panic(fmt.Sprintf("Unhandled block node of kind %v", node.Kind()))
}

func (p *mdP) acceptParagraph(node *gmAst.Paragraph) *sx.Pair {
	if is := p.acceptInlineChildren(node); is != nil {
		return sz.MakePara(is)
	}
	return nil
}

func (p *mdP) acceptHeading(node *gmAst.Heading) *sx.Pair {
	return sz.MakeHeading(node.Level, nil, p.acceptInlineChildren(node), "", "")
}

func (*mdP) acceptThematicBreak() *sx.Pair {
	return sz.MakeThematic(nil /*TODO*/)
}

func (p *mdP) acceptCodeBlock(node *gmAst.CodeBlock) *sx.Pair {
	return sz.MakeVerbatim(sz.SymVerbatimCode, nil /*TODO*/, string(p.acceptRawText(node)))
}

func (p *mdP) acceptFencedCodeBlock(node *gmAst.FencedCodeBlock) *sx.Pair {
	var a sx.ListBuilder
	if language := node.Language(p.source); len(language) > 0 {
		a.Add(sx.Cons(sx.MakeString("class"), sx.MakeString("language-"+cleanText(language, true))))
	}
	return sz.MakeVerbatim(sz.SymVerbatimCode, a.List(), string(p.acceptRawText(node)))
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

func (p *mdP) acceptBlockquote(node *gmAst.Blockquote) *sx.Pair {
	return p.acceptItemSlice(node).Cons(sz.SymListQuote)
}

func (p *mdP) acceptList(node *gmAst.List) *sx.Pair {
	kind := sz.SymListUnordered
	var a sx.ListBuilder
	if node.IsOrdered() {
		kind = sz.SymListOrdered
		if node.Start != 1 {
			a.Add(sx.Cons(sx.MakeString("start"), sx.MakeString(strconv.Itoa(node.Start))))
		}
	}
	var result sx.ListBuilder
	result.Add(kind)
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		item, ok := child.(*gmAst.ListItem)
		if !ok {
			panic(fmt.Sprintf("Expected list item node, but got %v", child.Kind()))
		}
		result.Add(p.acceptItemSlice(item))
	}
	return result.List()
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
		return sz.MakePara(is)
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
	return sz.MakeVerbatim(sz.SymVerbatimHTML, nil, string(content))
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
		panic(fmt.Sprintf("Expected inline node, but got %v", node.Type()))
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
	panic(fmt.Sprintf("Unhandled inline node %v", node.Kind()))
}

func (p *mdP) acceptText(node *gmAst.Text) (*sx.Pair, *sx.Pair) {
	segment := node.Segment
	text := segment.Value(p.source)
	if text == nil {
		return nil, nil
	}
	if node.IsRaw() {
		return sz.MakeText(string(text)), nil
	}
	in := sz.MakeText(cleanText(text, true))
	if node.HardLineBreak() {
		return in, sz.MakeHard()
	}
	if node.SoftLineBreak() {
		return in, sz.MakeSoft()
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
			if s, ok := inp.ScanEntity(); ok {
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
	return sz.MakeLiteral(sz.SymLiteralCode, nil /* TODO */, content), nil
}

func (p *mdP) acceptEmphasis(node *gmAst.Emphasis) (*sx.Pair, *sx.Pair) {
	sym := sz.SymFormatEmph
	if node.Level == 2 {
		sym = sz.SymFormatStrong
	}
	return sz.MakeFormat(sym, nil /* TODO */, p.acceptInlineChildren(node)), nil
}

func (p *mdP) acceptLink(node *gmAst.Link) (*sx.Pair, *sx.Pair) {
	ref := sz.ScanReference(cleanText(node.Destination, true))
	var a sx.ListBuilder
	if title := node.Title; len(title) > 0 {
		a.Add(sx.Cons(sx.MakeString("title"), sx.MakeString(cleanText(title, true))))
	}
	return sz.MakeLink(a.List(), ref, p.acceptInlineChildren(node)), nil
}

func (p *mdP) acceptImage(node *gmAst.Image) (*sx.Pair, *sx.Pair) {
	ref := sz.ScanReference(cleanText(node.Destination, true))
	var a sx.ListBuilder
	if title := node.Title; len(title) > 0 {
		a.Add(sx.Cons(sx.MakeString("title"), sx.MakeString(cleanText(title, true))))
	}
	return sz.MakeEmbed(a.List(), ref, "", p.acceptInlineChildren(node)), nil
}

func (p *mdP) acceptAutoLink(node *gmAst.AutoLink) (*sx.Pair, *sx.Pair) {
	u := node.URL(p.source)
	if node.AutoLinkType == gmAst.AutoLinkEmail &&
		!bytes.HasPrefix(bytes.ToLower(u), []byte("mailto:")) {
		u = append([]byte("mailto:"), u...)
	}
	return sz.MakeLink(nil /* TODO */, sz.ScanReference(cleanText(u, false)), nil), nil
}

func (p *mdP) acceptRawHTML(node *gmAst.RawHTML) (*sx.Pair, *sx.Pair) {
	segs := make([][]byte, 0, node.Segments.Len())
	for i := range node.Segments.Len() {
		segment := node.Segments.At(i)
		segs = append(segs, segment.Value(p.source))
	}
	return sz.MakeLiteral(
		sz.SymLiteralCode,
		sx.Cons(sx.Cons(sx.MakeString(""), sx.MakeString("html")), sx.Nil()),
		string(bytes.Join(segs, nil)),
	), nil
}
