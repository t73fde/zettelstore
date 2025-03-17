//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package encoder

// Encodes the abstract syntax tree back into Markdown.

import (
	"io"

	"t73f.de/r/zsc/attrs"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"

	"zettelstore.de/z/internal/ast"
)

// mdEncoder contains all data needed for encoding.
type mdEncoder struct {
	lang string
}

// WriteZettel writes the encoded zettel to the writer.
func (me *mdEncoder) WriteZettel(w io.Writer, zn *ast.ZettelNode) (int, error) {
	v := newMDVisitor(w, me.lang)
	v.acceptMeta(zn.InhMeta)
	if zn.InhMeta.YamlSep {
		v.b.WriteString("---\n")
	} else {
		v.b.WriteByte('\n')
	}
	ast.Walk(&v, &zn.BlocksAST)
	length, err := v.b.Flush()
	return length, err
}

// WriteMeta encodes meta data as markdown.
func (me *mdEncoder) WriteMeta(w io.Writer, m *meta.Meta) (int, error) {
	v := newMDVisitor(w, me.lang)
	v.acceptMeta(m)
	length, err := v.b.Flush()
	return length, err
}

func (v *mdVisitor) acceptMeta(m *meta.Meta) {
	for key, val := range m.Computed() {
		v.b.WriteStrings(key, ": ", string(val), "\n")
	}
}

// WriteBlocks writes the content of a block slice to the writer.
func (me *mdEncoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) (int, error) {
	v := newMDVisitor(w, me.lang)
	ast.Walk(&v, bs)
	length, err := v.b.Flush()
	return length, err
}

// mdVisitor writes the abstract syntax tree to an EncWriter.
type mdVisitor struct {
	b            encWriter
	listInfo     []int
	listPrefix   string
	langStack    shtml.LangStack
	quoteNesting uint
}

func newMDVisitor(w io.Writer, lang string) mdVisitor {
	return mdVisitor{b: newEncWriter(w), langStack: shtml.NewLangStack(lang)}
}

// pushAttribute adds the current attributes to the visitor.
func (v *mdVisitor) pushAttributes(a attrs.Attributes) {
	if value, ok := a.Get("lang"); ok {
		v.langStack.Push(value)
	} else {
		v.langStack.Dup()
	}
}

// popAttributes removes the current attributes from the visitor.
func (v *mdVisitor) popAttributes() { v.langStack.Pop() }

// getLanguage returns the current language,
func (v *mdVisitor) getLanguage() string { return v.langStack.Top() }

func (v *mdVisitor) getQuotes() (string, string, bool) {
	qi := shtml.GetQuoteInfo(v.getLanguage())
	leftQ, rightQ := qi.GetQuotes(v.quoteNesting)
	return leftQ, rightQ, qi.GetNBSp()
}

func (v *mdVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.BlockSlice:
		v.visitBlockSlice(n)
	case *ast.VerbatimNode:
		v.visitVerbatim(n)
	case *ast.RegionNode:
		v.visitRegion(n)
	case *ast.HeadingNode:
		v.visitHeading(n)
	case *ast.HRuleNode:
		v.b.WriteString("---")
	case *ast.NestedListNode:
		v.visitNestedList(n)
	case *ast.DescriptionListNode:
		return nil // Should write no content
	case *ast.TableNode:
		return nil // Should write no content
	case *ast.TextNode:
		v.b.WriteString(n.Text)
	case *ast.BreakNode:
		v.visitBreak(n)
	case *ast.LinkNode:
		v.visitLink(n)
	case *ast.EmbedRefNode:
		v.visitEmbedRef(n)
	case *ast.FootnoteNode:
		return nil // Should write no content
	case *ast.FormatNode:
		v.visitFormat(n)
	case *ast.LiteralNode:
		v.visitLiteral(n)
	default:
		return v
	}
	return nil
}

func (v *mdVisitor) visitBlockSlice(bs *ast.BlockSlice) {
	for i, bn := range *bs {
		if i > 0 {
			v.b.WriteString("\n\n")
		}
		ast.Walk(v, bn)
	}
}

func (v *mdVisitor) visitVerbatim(vn *ast.VerbatimNode) {
	lc := len(vn.Content)
	if vn.Kind != ast.VerbatimCode || lc == 0 {
		return
	}
	v.writeSpaces(4)
	lcm1 := lc - 1
	for i := 0; i < lc; i++ {
		b := vn.Content[i]
		if b != '\n' && b != '\r' {
			v.b.WriteByte(b)
			continue
		}
		j := i + 1
		for ; j < lc; j++ {
			c := vn.Content[j]
			if c != '\n' && c != '\r' {
				break
			}
		}
		if j >= lcm1 {
			break
		}
		v.b.WriteByte('\n')
		v.writeSpaces(4)
		i = j - 1
	}
}

func (v *mdVisitor) visitRegion(rn *ast.RegionNode) {
	if rn.Kind != ast.RegionQuote {
		return
	}
	v.pushAttributes(rn.Attrs)
	defer v.popAttributes()

	first := true
	for _, bn := range rn.Blocks {
		pn, ok := bn.(*ast.ParaNode)
		if !ok {
			continue
		}
		if !first {
			v.b.WriteString("\n>\n")
		}
		first = false
		v.b.WriteString("> ")
		ast.Walk(v, &pn.Inlines)
	}
}

func (v *mdVisitor) visitHeading(hn *ast.HeadingNode) {
	v.pushAttributes(hn.Attrs)
	defer v.popAttributes()

	const headingSigns = "###### "
	v.b.WriteString(headingSigns[len(headingSigns)-hn.Level-1:])
	ast.Walk(v, &hn.Inlines)
}

func (v *mdVisitor) visitNestedList(ln *ast.NestedListNode) {
	switch ln.Kind {
	case ast.NestedListOrdered:
		v.writeNestedList(ln, "1. ")
	case ast.NestedListUnordered:
		v.writeNestedList(ln, "* ")
	case ast.NestedListQuote:
		v.writeListQuote(ln)
	}
	v.listInfo = v.listInfo[:len(v.listInfo)-1]
}

func (v *mdVisitor) writeNestedList(ln *ast.NestedListNode, enum string) {
	v.listInfo = append(v.listInfo, len(enum))
	regIndent := 4*len(v.listInfo) - 4
	paraIndent := regIndent + len(enum)
	for i, item := range ln.Items {
		if i > 0 {
			v.b.WriteByte('\n')
		}
		v.writeSpaces(regIndent)
		v.b.WriteString(enum)
		for j, in := range item {
			if j > 0 {
				v.b.WriteByte('\n')
				if _, ok := in.(*ast.ParaNode); ok {
					v.writeSpaces(paraIndent)
				}
			}
			ast.Walk(v, in)
		}
	}
}

func (v *mdVisitor) writeListQuote(ln *ast.NestedListNode) {
	v.listInfo = append(v.listInfo, 0)
	if len(v.listInfo) > 1 {
		return
	}

	prefix := v.listPrefix
	v.listPrefix = "> "

	for i, item := range ln.Items {
		if i > 0 {
			v.b.WriteByte('\n')
		}
		v.b.WriteString(v.listPrefix)
		for j, in := range item {
			if j > 0 {
				v.b.WriteByte('\n')
				if _, ok := in.(*ast.ParaNode); ok {
					v.b.WriteString(v.listPrefix)
				}
			}
			ast.Walk(v, in)
		}
	}

	v.listPrefix = prefix
}

func (v *mdVisitor) visitBreak(bn *ast.BreakNode) {
	if bn.Hard {
		v.b.WriteString("\\\n")
	} else {
		v.b.WriteByte('\n')
	}
	if l := len(v.listInfo); l > 0 {
		if v.listPrefix == "" {
			v.writeSpaces(4*l - 4 + v.listInfo[l-1])
		} else {
			v.writeSpaces(4*l - 4)
			v.b.WriteString(v.listPrefix)
		}
	}
}

func (v *mdVisitor) visitLink(ln *ast.LinkNode) {
	v.pushAttributes(ln.Attrs)
	defer v.popAttributes()

	v.writeReference(ln.Ref, ln.Inlines)
}

func (v *mdVisitor) visitEmbedRef(en *ast.EmbedRefNode) {
	v.pushAttributes(en.Attrs)
	defer v.popAttributes()

	v.b.WriteByte('!')
	v.writeReference(en.Ref, en.Inlines)
}

func (v *mdVisitor) writeReference(ref *ast.Reference, is ast.InlineSlice) {
	if ref.State == ast.RefStateQuery {
		ast.Walk(v, &is)
	} else if len(is) > 0 {
		v.b.WriteByte('[')
		ast.Walk(v, &is)
		v.b.WriteStrings("](", ref.String())
		v.b.WriteByte(')')
	} else if isAutoLinkable(ref) {
		v.b.WriteByte('<')
		v.b.WriteString(ref.String())
		v.b.WriteByte('>')
	} else {
		s := ref.String()
		v.b.WriteStrings("[", s, "](", s, ")")
	}
}

func isAutoLinkable(ref *ast.Reference) bool {
	if ref.State != ast.RefStateExternal || ref.URL == nil {
		return false
	}
	return ref.URL.Scheme != ""
}

func (v *mdVisitor) visitFormat(fn *ast.FormatNode) {
	v.pushAttributes(fn.Attrs)
	defer v.popAttributes()

	switch fn.Kind {
	case ast.FormatEmph:
		v.b.WriteByte('*')
		ast.Walk(v, &fn.Inlines)
		v.b.WriteByte('*')
	case ast.FormatStrong:
		v.b.WriteString("__")
		ast.Walk(v, &fn.Inlines)
		v.b.WriteString("__")
	case ast.FormatQuote:
		v.writeQuote(fn)
	case ast.FormatMark:
		v.b.WriteString("<mark>")
		ast.Walk(v, &fn.Inlines)
		v.b.WriteString("</mark>")
	default:
		ast.Walk(v, &fn.Inlines)
	}
}

func (v *mdVisitor) writeQuote(fn *ast.FormatNode) {
	leftQ, rightQ, withNbsp := v.getQuotes()
	v.b.WriteString(leftQ)
	if withNbsp {
		v.b.WriteString("&nbsp;")
	}
	v.quoteNesting++
	ast.Walk(v, &fn.Inlines)
	v.quoteNesting--
	if withNbsp {
		v.b.WriteString("&nbsp;")
	}
	v.b.WriteString(rightQ)
}

func (v *mdVisitor) visitLiteral(ln *ast.LiteralNode) {
	switch ln.Kind {
	case ast.LiteralCode, ast.LiteralInput, ast.LiteralOutput:
		v.b.WriteByte('`')
		v.b.Write(ln.Content)
		v.b.WriteByte('`')
	case ast.LiteralComment: // ignore everything
	default:
		v.b.Write(ln.Content)
	}
}

func (v *mdVisitor) writeSpaces(n int) {
	for range n {
		v.b.WriteByte(' ')
	}
}
