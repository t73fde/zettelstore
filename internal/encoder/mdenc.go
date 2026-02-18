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

	"zettelstore.de/z/internal/domain"
)

// mdEncoder contains all data needed for encoding.
type mdEncoder struct {
	lang string
}

// WriteZettel writes the encoded zettel to the writer.
func (me *mdEncoder) WriteZettel(w io.Writer, zn *domain.Zettel) error {
	v := newMDVisitor(w, me.lang)
	v.b.WriteMeta(zn.InhMeta)
	if zn.InhMeta.YamlSep {
		v.b.WriteString("---\n")
	} else {
		v.b.WriteLn()
	}
	v.walk(zn.Blocks, nil)
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
func (v *mdVisitor) VisitItBefore(node *sx.Pair, alst *sx.Pair) bool {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymBlock:
			v.visitBlock(node, alst)

		case zsx.SymText:
			v.b.WriteString(zsx.GetText(node))
		case zsx.SymSoft:
			v.visitBreak(false)
		case zsx.SymHard:
			v.visitBreak(true)

		case zsx.SymLink:
			attrs, ref, inlines := zsx.GetLink(node)
			alst = v.setLanguage(alst, attrs)
			v.visitReference(ref, inlines, alst)
		case zsx.SymEmbed:
			attrs, ref, _, inlines := zsx.GetEmbed(node)
			alst = v.setLanguage(alst, attrs)
			_ = v.b.WriteByte('!')
			v.visitReference(ref, inlines, alst)

		case zsx.SymFormatEmph:
			v.visitFormat(node, alst, "*", "*")
		case zsx.SymFormatStrong:
			v.visitFormat(node, alst, "__", "__")
		case zsx.SymFormatQuote:
			v.visitQuote(node, alst)
		case zsx.SymFormatMark:
			v.visitFormat(node, alst, "<mark>", "</mark>")
		case zsx.SymFormatSpan, zsx.SymFormatDelete, zsx.SymFormatInsert, zsx.SymFormatSub, zsx.SymFormatSuper:
			v.visitFormat(node, alst, "", "")

		case zsx.SymLiteralCode, zsx.SymLiteralInput, zsx.SymLiteralOutput:
			_, _, content := zsx.GetLiteral(node)
			v.b.WriteStrings("`", content, "`")
		case zsx.SymLiteralMath:
			_, _, content := zsx.GetLiteral(node)
			v.b.WriteString(content)

		case zsx.SymHeading:
			level, attrs, text, _, _ := zsx.GetHeading(node)
			const headingSigns = "###### "
			v.b.WriteString(headingSigns[len(headingSigns)-level-1:])
			v.walkList(text, v.setLanguage(alst, attrs))

		case zsx.SymThematic:
			v.b.WriteString("---")

		case zsx.SymListOrdered:
			v.visitNestedList(node, alst, enumOrdered)
		case zsx.SymListUnordered:
			v.visitNestedList(node, alst, enumUnordered)
		case zsx.SymListQuote:
			if len(v.listInfo) == 0 {
				v.visitListQuote(node, alst)
			}

		case zsx.SymVerbatimCode:
			v.visitVerbatim(node)

		case zsx.SymRegionQuote:
			v.visitRegion(node, alst)

		case zsx.SymRegionBlock, zsx.SymRegionVerse,
			zsx.SymVerbatimComment, zsx.SymVerbatimEval, zsx.SymVerbatimHTML, zsx.SymVerbatimMath, zsx.SymVerbatimZettel,
			zsx.SymDescription, zsx.SymTable, zsx.SymEndnote,
			zsx.SymLiteralComment:
			// Do nothing, ignore it.

		default:
			return false
		}
		return true
	}
	return false
}

func (v *mdVisitor) VisitItAfter(*sx.Pair, *sx.Pair) {}

func (v *mdVisitor) visitBlock(node *sx.Pair, alst *sx.Pair) {
	first := true
	for bn := range node.Tail().Pairs() {
		if first {
			first = false
		} else {
			v.b.WriteString("\n\n")
		}
		v.walk(bn.Head(), alst)
	}
}

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

func (v *mdVisitor) visitReference(ref, inlines, alst *sx.Pair) {
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

func (v *mdVisitor) visitFormat(node, alst *sx.Pair, delim1, delim2 string) {
	_, attrs, inlines := zsx.GetFormat(node)
	alst = v.setLanguage(alst, attrs)
	v.b.WriteString(delim1)
	v.walkList(inlines, alst)
	v.b.WriteString(delim2)
}
func (v *mdVisitor) visitQuote(node, alst *sx.Pair) {
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

const enumOrdered = "1. "
const enumUnordered = "* "

func (v *mdVisitor) visitNestedList(node *sx.Pair, alst *sx.Pair, enum string) {
	v.listInfo = append(v.listInfo, len(enum))
	regIndent := 4*len(v.listInfo) - 4
	paraIndent := regIndent + len(enum)
	_, attrs, blocks := zsx.GetList(node)
	alst = v.setLanguage(alst, attrs)
	firstBlk := true
	for blk := range blocks.Pairs() {
		if firstBlk {
			firstBlk = false
		} else {
			v.b.WriteLn()
		}
		v.writeSpaces(regIndent)
		v.b.WriteString(enum)
		first := true
		for item := range blk.Head().Tail().Pairs() {
			in := item.Head()
			if first {
				first = false
			} else {
				v.b.WriteLn()
				if zsx.SymPara.IsEqual(in.Car()) {
					v.writeSpaces(paraIndent)
				}
			}
			v.walk(in, alst)
		}
	}
	v.listInfo = v.listInfo[:len(v.listInfo)-1]
}
func (v *mdVisitor) visitListQuote(node *sx.Pair, alst *sx.Pair) {
	v.listInfo = []int{0}
	oldPrefix := v.listPrefix
	v.listPrefix = "> "

	_, attrs, blocks := zsx.GetList(node)
	alst = v.setLanguage(alst, attrs)
	firstBlk := true
	for blk := range blocks.Pairs() {
		if firstBlk {
			firstBlk = false
		} else {
			v.b.WriteLn()
		}
		v.b.WriteString(v.listPrefix)
		first := true
		for item := range blk.Head().Tail().Pairs() {
			in := item.Head()
			if first {
				first = false
			} else {
				v.b.WriteLn()
				if zsx.SymPara.IsEqual(in.Car()) {
					v.b.WriteString(v.listPrefix)
				}
			}
			v.walk(in, alst)
		}
	}
	v.listPrefix = oldPrefix
	v.listInfo = nil
}

func (v *mdVisitor) visitVerbatim(node *sx.Pair) {
	if _, _, content := zsx.GetVerbatim(node); content != "" {
		lc := len(content)
		v.writeSpaces(4)
		lcm1 := lc - 1
		for i := 0; i < lc; i++ {
			b := content[i]
			if b != '\n' && b != '\r' {
				_ = v.b.WriteByte(b)
				continue
			}
			j := i + 1
			for ; j < lc; j++ {
				c := content[j]
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
}

func (v *mdVisitor) visitRegion(node *sx.Pair, alst *sx.Pair) {
	_, attrs, blocks, _ := zsx.GetRegion(node)
	alst = v.setLanguage(alst, attrs)

	first := true
	for n := range blocks.Pairs() {
		blk := n.Head()
		if zsx.SymPara.IsEqual(blk.Car()) {
			if first {
				first = false
			} else {
				v.b.WriteString("\n>\n")
			}
			v.b.WriteString("> ")
			v.walk(blk, alst)
		}
	}
}

func (v *mdVisitor) writeSpaces(n int) {
	for range n {
		v.b.WriteSpace()
	}
}
