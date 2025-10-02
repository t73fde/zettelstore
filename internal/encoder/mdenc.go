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
	"net/url"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
)

// mdEncoder contains all data needed for encoding.
type mdEncoder struct {
	lang string
}

// WriteZettel writes the encoded zettel to the writer.
func (me *mdEncoder) WriteZettel(w io.Writer, zn *ast.ZettelNode) error {
	v := newMDVisitorAST(w, me.lang)
	v.b.WriteMeta(zn.InhMeta)
	if zn.InhMeta.YamlSep {
		v.b.WriteString("---\n")
	} else {
		v.b.WriteLn()
	}
	ast.Walk(&v, &zn.BlocksAST)
	return v.b.Flush()
}

// WriteMeta encodes meta data as markdown.
func (*mdEncoder) WriteMeta(w io.Writer, m *meta.Meta) error {
	ew := newEncWriter(w)
	ew.WriteMeta(m)
	return ew.Flush()
}

// WriteSz encodes SZ represented zettel content.
func (me *mdEncoder) WriteSz(w io.Writer, node *sx.Pair) error {
	v := newMDVisitor(w, me.lang)
	zsx.WalkIt(&v, node, nil)
	return v.b.Flush()
}

type mdVisitor struct {
	b            encWriter
	listInfo     []int
	listPrefix   string
	defLang      string
	quoteNesting uint
}

func newMDVisitor(w io.Writer, lang string) mdVisitor {
	return mdVisitor{b: newEncWriter(w), defLang: lang}
}

var symLang = sx.MakeSymbol("lang")

func (v *mdVisitor) getLanguage(alst *sx.Pair) string {
	if a := alst.Assoc(symLang); a != nil {
		if s, isString := sx.GetString(a.Cdr()); isString {
			return s.GetValue()
		}
	}
	return v.defLang
}
func (*mdVisitor) setLanguage(alst, attrs *sx.Pair) *sx.Pair {
	if a := attrs.Assoc(sx.MakeString("lang")); a != nil {
		val := a.Cdr()
		if p, isPair := sx.GetPair(val); isPair {
			val = p.Car()
		}
		return alst.Cons(sx.Cons(symLang, val))
	}
	return alst
}

func (v *mdVisitor) getQuotes(alst *sx.Pair) (string, string, bool) {
	qi := shtml.GetQuoteInfo(v.getLanguage(alst))
	leftQ, rightQ := qi.GetQuotes(v.quoteNesting)
	return leftQ, rightQ, qi.GetNBSp()
}
func (v *mdVisitor) walk(node, alst *sx.Pair)    { zsx.WalkIt(v, node, alst) }
func (v *mdVisitor) walkList(lst, alst *sx.Pair) { zsx.WalkItList(v, lst, 0, alst) }
func (v *mdVisitor) VisitBefore(node *sx.Pair, alst *sx.Pair) bool {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymText:
			if s, isString := sx.GetString(node.Tail().Car()); isString {
				v.b.WriteString(s.GetValue())
			}
		case zsx.SymSoft:
			v.visitBreak(false)
		case zsx.SymHard:
			v.visitBreak(true)

		case zsx.SymLink:
			attrs, ref, inlines := zsx.GetLink(node)
			alst = v.setLanguage(alst, attrs)
			v.writeReference(ref, inlines, alst)
		case zsx.SymEmbed:
			attrs, ref, _, inlines := zsx.GetEmbed(node)
			alst = v.setLanguage(alst, attrs)
			_ = v.b.WriteByte('!')
			v.writeReference(ref, inlines, alst)

		case zsx.SymFormatEmph:
			v.writeFormat(node, alst, "*", "*")
		case zsx.SymFormatStrong:
			v.writeFormat(node, alst, "__", "__")
		case zsx.SymFormatQuote:
			v.writeQuote(node, alst)
		case zsx.SymFormatMark:
			v.writeFormat(node, alst, "<mark>", "</mark>")

		case zsx.SymDescription, zsx.SymTable, zsx.SymEndnote:
			// Do nothing, ignore it.
		default:
			return false
		}
		return true
	}
	return false
}

func (v *mdVisitor) VisitAfter(*sx.Pair, *sx.Pair) {}
func (v *mdVisitor) visitBreak(isHard bool) {
	if isHard {
		v.b.WriteString("\\\n")
	} else {
		v.b.WriteLn()
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

func (v *mdVisitor) writeReference(ref, inlines, alst *sx.Pair) {
	refState, val := zsx.GetReference(ref)
	if sz.SymRefStateQuery.IsEqualSymbol(refState) {
		v.walkList(inlines, alst)
	} else if inlines != nil {
		_ = v.b.WriteByte('[')
		v.walkList(inlines, alst)
		v.b.WriteStrings("](", val)
		_ = v.b.WriteByte(')')
	} else if isAutoLinkable(refState, val) {
		_ = v.b.WriteByte('<')
		v.b.WriteString(val)
		_ = v.b.WriteByte('>')
	} else {
		v.b.WriteStrings("[", val, "](", val, ")")
	}
}
func isAutoLinkable(refState *sx.Symbol, val string) bool {
	if zsx.SymRefStateExternal.IsEqualSymbol(refState) {
		if u, err := url.Parse(val); err == nil && u.Scheme != "" {
			return true
		}
	}
	return false
}

func (v *mdVisitor) writeFormat(node, alst *sx.Pair, delim1, delim2 string) {
	_, attrs, inlines := zsx.GetFormat(node)
	alst = v.setLanguage(alst, attrs)
	v.b.WriteString(delim1)
	v.walkList(inlines, alst)
	v.b.WriteString(delim2)
}
func (v *mdVisitor) writeQuote(node, alst *sx.Pair) {
	_, attrs, inlines := zsx.GetFormat(node)
	alst = v.setLanguage(alst, attrs)
	leftQ, rightQ, withNbsp := v.getQuotes(alst)
	v.b.WriteString(leftQ)
	if withNbsp {
		v.b.WriteString("&nbsp;")
	}
	v.quoteNesting++
	v.walkList(inlines, alst)
	v.quoteNesting--
	if withNbsp {
		v.b.WriteString("&nbsp;")
	}
	v.b.WriteString(rightQ)
}

func (v *mdVisitor) writeSpaces(n int) {
	for range n {
		v.b.WriteSpace()
	}
}

// WriteBlocks writes the content of a block slice to the writer.
func (me *mdEncoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) error {
	v := newMDVisitorAST(w, me.lang)
	ast.Walk(&v, bs)
	return v.b.Flush()
}

// mdVisitorAST writes the abstract syntax tree to an EncWriter.
type mdVisitorAST struct {
	b            encWriter
	listInfo     []int
	listPrefix   string
	langStack    shtml.LangStack
	quoteNesting uint
}

func newMDVisitorAST(w io.Writer, lang string) mdVisitorAST {
	return mdVisitorAST{b: newEncWriter(w), langStack: shtml.NewLangStack(lang)}
}

// pushAttribute adds the current attributes to the visitor.
func (v *mdVisitorAST) pushAttributes(a zsx.Attributes) {
	if value, ok := a.Get("lang"); ok {
		v.langStack.Push(value)
	} else {
		v.langStack.Dup()
	}
}

// popAttributes removes the current attributes from the visitor.
func (v *mdVisitorAST) popAttributes() { v.langStack.Pop() }

// getLanguage returns the current language,
func (v *mdVisitorAST) getLanguage() string { return v.langStack.Top() }

func (v *mdVisitorAST) getQuotes() (string, string, bool) {
	qi := shtml.GetQuoteInfo(v.getLanguage())
	leftQ, rightQ := qi.GetQuotes(v.quoteNesting)
	return leftQ, rightQ, qi.GetNBSp()
}

func (v *mdVisitorAST) Visit(node ast.Node) ast.Visitor {
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

func (v *mdVisitorAST) visitBlockSlice(bs *ast.BlockSlice) {
	for i, bn := range *bs {
		if i > 0 {
			v.b.WriteString("\n\n")
		}
		ast.Walk(v, bn)
	}
}

func (v *mdVisitorAST) visitVerbatim(vn *ast.VerbatimNode) {
	lc := len(vn.Content)
	if vn.Kind != ast.VerbatimCode || lc == 0 {
		return
	}
	v.writeSpaces(4)
	lcm1 := lc - 1
	for i := 0; i < lc; i++ {
		b := vn.Content[i]
		if b != '\n' && b != '\r' {
			_ = v.b.WriteByte(b)
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
		v.b.WriteLn()
		v.writeSpaces(4)
		i = j - 1
	}
}

func (v *mdVisitorAST) visitRegion(rn *ast.RegionNode) {
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

func (v *mdVisitorAST) visitHeading(hn *ast.HeadingNode) {
	v.pushAttributes(hn.Attrs)
	defer v.popAttributes()

	const headingSigns = "###### "
	v.b.WriteString(headingSigns[len(headingSigns)-hn.Level-1:])
	ast.Walk(v, &hn.Inlines)
}

func (v *mdVisitorAST) visitNestedList(ln *ast.NestedListNode) {
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

func (v *mdVisitorAST) writeNestedList(ln *ast.NestedListNode, enum string) {
	v.listInfo = append(v.listInfo, len(enum))
	regIndent := 4*len(v.listInfo) - 4
	paraIndent := regIndent + len(enum)
	for i, item := range ln.Items {
		if i > 0 {
			v.b.WriteLn()
		}
		v.writeSpaces(regIndent)
		v.b.WriteString(enum)
		for j, in := range item {
			if j > 0 {
				v.b.WriteLn()
				if _, ok := in.(*ast.ParaNode); ok {
					v.writeSpaces(paraIndent)
				}
			}
			ast.Walk(v, in)
		}
	}
}

func (v *mdVisitorAST) writeListQuote(ln *ast.NestedListNode) {
	v.listInfo = append(v.listInfo, 0)
	if len(v.listInfo) > 1 {
		return
	}

	prefix := v.listPrefix
	v.listPrefix = "> "

	for i, item := range ln.Items {
		if i > 0 {
			v.b.WriteLn()
		}
		v.b.WriteString(v.listPrefix)
		for j, in := range item {
			if j > 0 {
				v.b.WriteLn()
				if _, ok := in.(*ast.ParaNode); ok {
					v.b.WriteString(v.listPrefix)
				}
			}
			ast.Walk(v, in)
		}
	}

	v.listPrefix = prefix
}

func (v *mdVisitorAST) visitBreak(bn *ast.BreakNode) {
	if bn.Hard {
		v.b.WriteString("\\\n")
	} else {
		v.b.WriteLn()
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

func (v *mdVisitorAST) visitLink(ln *ast.LinkNode) {
	v.pushAttributes(ln.Attrs)
	defer v.popAttributes()

	v.writeReference(ln.Ref, ln.Inlines)
}

func (v *mdVisitorAST) visitEmbedRef(en *ast.EmbedRefNode) {
	v.pushAttributes(en.Attrs)
	defer v.popAttributes()

	_ = v.b.WriteByte('!')
	v.writeReference(en.Ref, en.Inlines)
}

func (v *mdVisitorAST) writeReference(ref *ast.Reference, is ast.InlineSlice) {
	if ref.State == ast.RefStateQuery {
		ast.Walk(v, &is)
	} else if len(is) > 0 {
		_ = v.b.WriteByte('[')
		ast.Walk(v, &is)
		v.b.WriteStrings("](", ref.String())
		_ = v.b.WriteByte(')')
	} else if isAutoLinkableAST(ref) {
		_ = v.b.WriteByte('<')
		v.b.WriteString(ref.String())
		_ = v.b.WriteByte('>')
	} else {
		s := ref.String()
		v.b.WriteStrings("[", s, "](", s, ")")
	}
}

func isAutoLinkableAST(ref *ast.Reference) bool {
	if ref.State != ast.RefStateExternal || ref.URL == nil {
		return false
	}
	return ref.URL.Scheme != ""
}

func (v *mdVisitorAST) visitFormat(fn *ast.FormatNode) {
	v.pushAttributes(fn.Attrs)
	defer v.popAttributes()

	switch fn.Kind {
	case ast.FormatEmph:
		_ = v.b.WriteByte('*')
		ast.Walk(v, &fn.Inlines)
		_ = v.b.WriteByte('*')
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

func (v *mdVisitorAST) writeQuote(fn *ast.FormatNode) {
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

func (v *mdVisitorAST) visitLiteral(ln *ast.LiteralNode) {
	switch ln.Kind {
	case ast.LiteralCode, ast.LiteralInput, ast.LiteralOutput:
		_ = v.b.WriteByte('`')
		_, _ = v.b.Write(ln.Content)
		_ = v.b.WriteByte('`')
	case ast.LiteralComment: // ignore everything
	default:
		_, _ = v.b.Write(ln.Content)
	}
}

func (v *mdVisitorAST) writeSpaces(n int) {
	for range n {
		v.b.WriteSpace()
	}
}
