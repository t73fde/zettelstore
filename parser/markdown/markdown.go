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

// Package markdown provides a parser for markdown.
package markdown

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	gm "github.com/yuin/goldmark"
	gmAst "github.com/yuin/goldmark/ast"
	gmText "github.com/yuin/goldmark/text"

	"t73f.de/r/zsc/attrs"
	"t73f.de/r/zsc/input"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/encoder/textenc"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/zettel/meta"
)

func init() {
	parser.Register(&parser.Info{
		Name:          meta.SyntaxMarkdown,
		AltNames:      []string{meta.SyntaxMD},
		IsASTParser:   true,
		IsTextFormat:  true,
		IsImageFormat: false,
		ParseBlocks:   parseBlocks,
	})
}

func parseBlocks(inp *input.Input, _ *meta.Meta, _ string) ast.BlockSlice {
	p := parseMarkdown(inp)
	return p.acceptBlockChildren(p.docNode)
}

func parseMarkdown(inp *input.Input) *mdP {
	source := []byte(inp.Src[inp.Pos:])
	parser := gm.DefaultParser()
	node := parser.Parse(gmText.NewReader(source))
	textEnc := textenc.Create()
	return &mdP{source: source, docNode: node, textEnc: textEnc}
}

type mdP struct {
	source  []byte
	docNode gmAst.Node
	textEnc *textenc.Encoder
}

func (p *mdP) acceptBlockChildren(docNode gmAst.Node) ast.BlockSlice {
	if docNode.Type() != gmAst.TypeDocument {
		panic(fmt.Sprintf("Expected document, but got node type %v", docNode.Type()))
	}
	result := make(ast.BlockSlice, 0, docNode.ChildCount())
	for child := docNode.FirstChild(); child != nil; child = child.NextSibling() {
		if block := p.acceptBlock(child); block != nil {
			result = append(result, block)
		}
	}
	return result
}

func (p *mdP) acceptBlock(node gmAst.Node) ast.ItemNode {
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

func (p *mdP) acceptParagraph(node *gmAst.Paragraph) ast.ItemNode {
	if is := p.acceptInlineChildren(node); len(is) > 0 {
		return &ast.ParaNode{Inlines: is}
	}
	return nil
}

func (p *mdP) acceptHeading(node *gmAst.Heading) *ast.HeadingNode {
	return &ast.HeadingNode{
		Level:   node.Level,
		Inlines: p.acceptInlineChildren(node),
		Attrs:   nil,
	}
}

func (*mdP) acceptThematicBreak() *ast.HRuleNode {
	return &ast.HRuleNode{
		Attrs: nil, //TODO
	}
}

func (p *mdP) acceptCodeBlock(node *gmAst.CodeBlock) *ast.VerbatimNode {
	return &ast.VerbatimNode{
		Kind:    ast.VerbatimProg,
		Attrs:   nil, //TODO
		Content: p.acceptRawText(node),
	}
}

func (p *mdP) acceptFencedCodeBlock(node *gmAst.FencedCodeBlock) *ast.VerbatimNode {
	var a attrs.Attributes
	if language := node.Language(p.source); len(language) > 0 {
		a = a.Set("class", "language-"+cleanText(language, true))
	}
	return &ast.VerbatimNode{
		Kind:    ast.VerbatimProg,
		Attrs:   a,
		Content: p.acceptRawText(node),
	}
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

func (p *mdP) acceptBlockquote(node *gmAst.Blockquote) *ast.NestedListNode {
	return &ast.NestedListNode{
		Kind: ast.NestedListQuote,
		Items: []ast.ItemSlice{
			p.acceptItemSlice(node),
		},
	}
}

func (p *mdP) acceptList(node *gmAst.List) ast.ItemNode {
	kind := ast.NestedListUnordered
	var a attrs.Attributes
	if node.IsOrdered() {
		kind = ast.NestedListOrdered
		if node.Start != 1 {
			a = a.Set("start", strconv.Itoa(node.Start))
		}
	}
	items := make([]ast.ItemSlice, 0, node.ChildCount())
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		item, ok := child.(*gmAst.ListItem)
		if !ok {
			panic(fmt.Sprintf("Expected list item node, but got %v", child.Kind()))
		}
		items = append(items, p.acceptItemSlice(item))
	}
	return &ast.NestedListNode{
		Kind:  kind,
		Items: items,
		Attrs: a,
	}
}

func (p *mdP) acceptItemSlice(node gmAst.Node) ast.ItemSlice {
	result := make(ast.ItemSlice, 0, node.ChildCount())
	for elem := node.FirstChild(); elem != nil; elem = elem.NextSibling() {
		if item := p.acceptBlock(elem); item != nil {
			result = append(result, item)
		}
	}
	return result
}

func (p *mdP) acceptTextBlock(node *gmAst.TextBlock) ast.ItemNode {
	if is := p.acceptInlineChildren(node); len(is) > 0 {
		return &ast.ParaNode{Inlines: is}
	}
	return nil
}

func (p *mdP) acceptHTMLBlock(node *gmAst.HTMLBlock) *ast.VerbatimNode {
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
	return &ast.VerbatimNode{
		Kind:    ast.VerbatimHTML,
		Content: content,
	}
}

func (p *mdP) acceptInlineChildren(node gmAst.Node) ast.InlineSlice {
	result := make(ast.InlineSlice, 0, node.ChildCount())
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if inlines := p.acceptInline(child); inlines != nil {
			result = append(result, inlines...)
		}
	}
	return result
}

func (p *mdP) acceptInline(node gmAst.Node) ast.InlineSlice {
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

func (p *mdP) acceptText(node *gmAst.Text) ast.InlineSlice {
	segment := node.Segment
	text := segment.Value(p.source)
	if text == nil {
		return nil
	}
	if node.IsRaw() {
		return ast.InlineSlice{&ast.TextNode{Text: string(text)}}
	}
	result := make(ast.InlineSlice, 0, 2)
	in := &ast.TextNode{Text: cleanText(text, true)}
	result = append(result, in)
	if node.HardLineBreak() {
		result = append(result, &ast.BreakNode{Hard: true})
	} else if node.SoftLineBreak() {
		result = append(result, &ast.BreakNode{Hard: false})
	}
	return result
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

func (p *mdP) acceptCodeSpan(node *gmAst.CodeSpan) ast.InlineSlice {
	var segBuf bytes.Buffer
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		segment := c.(*gmAst.Text).Segment
		segBuf.Write(segment.Value(p.source))
	}
	content := segBuf.Bytes()

	// Clean code span
	if len(content) == 0 {
		content = nil
	} else {
		lastPos := 0
		var buf bytes.Buffer
		for pos, ch := range content {
			if ch == '\n' {
				buf.Write(content[lastPos:pos])
				if pos < len(content)-1 {
					buf.WriteByte(' ')
				}
				lastPos = pos + 1
			}
		}
		buf.Write(content[lastPos:])
		content = buf.Bytes()
	}

	return ast.InlineSlice{
		&ast.LiteralNode{
			Kind:    ast.LiteralProg,
			Attrs:   nil, //TODO
			Content: content,
		},
	}
}

func (p *mdP) acceptEmphasis(node *gmAst.Emphasis) ast.InlineSlice {
	kind := ast.FormatEmph
	if node.Level == 2 {
		kind = ast.FormatStrong
	}
	return ast.InlineSlice{
		&ast.FormatNode{
			Kind:    kind,
			Attrs:   nil, //TODO
			Inlines: p.acceptInlineChildren(node),
		},
	}
}

func (p *mdP) acceptLink(node *gmAst.Link) ast.InlineSlice {
	ref := ast.ParseReference(cleanText(node.Destination, true))
	var a attrs.Attributes
	if title := node.Title; len(title) > 0 {
		a = a.Set("title", cleanText(title, true))
	}
	return ast.InlineSlice{
		&ast.LinkNode{
			Ref:     ref,
			Inlines: p.acceptInlineChildren(node),
			Attrs:   a,
		},
	}
}

func (p *mdP) acceptImage(node *gmAst.Image) ast.InlineSlice {
	ref := ast.ParseReference(cleanText(node.Destination, true))
	var a attrs.Attributes
	if title := node.Title; len(title) > 0 {
		a = a.Set("title", cleanText(title, true))
	}
	return ast.InlineSlice{
		&ast.EmbedRefNode{
			Ref:     ref,
			Inlines: p.flattenInlineSlice(node),
			Attrs:   a,
		},
	}
}

func (p *mdP) flattenInlineSlice(node gmAst.Node) ast.InlineSlice {
	is := p.acceptInlineChildren(node)
	var sb strings.Builder
	_, err := p.textEnc.WriteInlines(&sb, &is)
	if err != nil {
		panic(err)
	}
	if sb.Len() == 0 {
		return nil
	}
	return ast.InlineSlice{&ast.TextNode{Text: sb.String()}}
}

func (p *mdP) acceptAutoLink(node *gmAst.AutoLink) ast.InlineSlice {
	u := node.URL(p.source)
	if node.AutoLinkType == gmAst.AutoLinkEmail &&
		!bytes.HasPrefix(bytes.ToLower(u), []byte("mailto:")) {
		u = append([]byte("mailto:"), u...)
	}
	return ast.InlineSlice{
		&ast.LinkNode{
			Ref:     ast.ParseReference(cleanText(u, false)),
			Inlines: nil,
			Attrs:   nil, // TODO
		},
	}
}

func (p *mdP) acceptRawHTML(node *gmAst.RawHTML) ast.InlineSlice {
	segs := make([][]byte, 0, node.Segments.Len())
	for i := range node.Segments.Len() {
		segment := node.Segments.At(i)
		segs = append(segs, segment.Value(p.source))
	}
	return ast.InlineSlice{
		&ast.LiteralNode{
			Kind:    ast.LiteralProg,
			Attrs:   attrs.Attributes{"": "html"},
			Content: bytes.Join(segs, nil),
		},
	}
}
