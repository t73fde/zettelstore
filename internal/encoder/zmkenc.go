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

package encoder

// zmkenc encodes the abstract syntax tree back into Zettelmarkup.

import (
	"io"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zero/set"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/domain"
)

// zmkEncoder contains all data needed for encoding.
type zmkEncoder struct{}

// WriteZettel writes the encoded zettel to the writer.
func (ze *zmkEncoder) WriteZettel(w io.Writer, zn *domain.Zettel) error {
	v := newZmkVisitor(w)
	v.b.WriteMeta(zn.InhMeta)
	if zn.InhMeta.YamlSep {
		v.b.WriteString("---\n")
	} else {
		v.b.WriteLn()
	}
	v.walk(zn.Blocks, nil)
	return v.b.Flush()
}

// WriteMeta encodes meta data as zmk.
func (ze *zmkEncoder) WriteMeta(w io.Writer, m *meta.Meta) error {
	ew := newEncWriter(w)
	ew.WriteMeta(m)
	return ew.Flush()
}

// WriteSz encodes SZ represented zettel content.
func (*zmkEncoder) WriteSz(w io.Writer, node *sx.Pair) error {
	v := newZmkVisitor(w)
	zsx.WalkIt(&v, node, nil)
	return v.b.Flush()
}

type zmkVisitor struct {
	b      encWriter
	prefix []byte
}

func newZmkVisitor(w io.Writer) zmkVisitor { return zmkVisitor{b: newEncWriter(w)} }

func (v *zmkVisitor) walk(node, alst *sx.Pair)    { zsx.WalkIt(v, node, alst) }
func (v *zmkVisitor) walkList(lst, alst *sx.Pair) { zsx.WalkItList(v, lst, 0, alst) }

func (v *zmkVisitor) VisitItBefore(node *sx.Pair, alst *sx.Pair) bool {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymText:
			v.writeText(zsx.GetText(node))
		case zsx.SymSoft:
			v.writeBreak(false)
		case zsx.SymHard:
			v.writeBreak(true)

		case zsx.SymFormatEmph:
			v.visitFormat(node, alst, "__")
		case zsx.SymFormatStrong:
			v.visitFormat(node, alst, "**")
		case zsx.SymFormatInsert:
			v.visitFormat(node, alst, ">>")
		case zsx.SymFormatDelete:
			v.visitFormat(node, alst, "~~")
		case zsx.SymFormatSuper:
			v.visitFormat(node, alst, "^^")
		case zsx.SymFormatSub:
			v.visitFormat(node, alst, ",,")
		case zsx.SymFormatQuote:
			v.visitFormat(node, alst, `""`)
		case zsx.SymFormatMark:
			v.visitFormat(node, alst, "##")
		case zsx.SymFormatSpan:
			v.visitFormat(node, alst, "::")

		case zsx.SymLiteralCode:
			_, attrs, content := zsx.GetLiteral(node)
			v.writeLiteral('`', attrs, content)
		case zsx.SymLiteralMath:
			_, attrs, content := zsx.GetLiteral(node)
			v.b.WriteStrings("$$", content, "$$")
			v.writeAttributes(attrs)
		case zsx.SymLiteralInput:
			_, attrs, content := zsx.GetLiteral(node)
			v.writeLiteral('\'', attrs, content)
		case zsx.SymLiteralOutput:
			_, attrs, content := zsx.GetLiteral(node)
			v.writeLiteral('=', attrs, content)
		case zsx.SymLiteralComment:
			_, attrs, content := zsx.GetLiteral(node)
			v.b.WriteString("%%")
			v.writeAttributes(attrs)
			v.b.WriteSpace()
			v.b.WriteString(content)

		case zsx.SymLink:
			v.visitLink(node, alst)
		case zsx.SymEmbed:
			v.visitEmbedRef(node, alst)
		case zsx.SymEndnote:
			v.visitEndnote(node, alst)
		case zsx.SymCite:
			v.visitCite(node, alst)
		case zsx.SymMark:
			v.visitMark(node, alst)

		case zsx.SymBlock:
			v.visitBlock(node, alst)
		case zsx.SymHeading:
			v.visitHeading(node, alst)
		case zsx.SymThematic:
			attrs := zsx.GetThematic(node)
			v.b.WriteString("---")
			v.writeAttributes(attrs)

		case zsx.SymListOrdered:
			v.visitNestedList(node, alst, '#')
		case zsx.SymListQuote:
			v.visitNestedList(node, alst, '>')
		case zsx.SymListUnordered:
			v.visitNestedList(node, alst, '*')

		case zsx.SymRegionBlock:
			v.visitRegion(node, alst, ":::")
		case zsx.SymRegionQuote:
			v.visitRegion(node, alst, "<<<")
		case zsx.SymRegionVerse:
			v.visitRegion(node, alst, "\"\"\"")

		case zsx.SymDescription:
			v.visitDescription(node, alst)
		case zsx.SymTable:
			v.visitTable(node, alst)
		case zsx.SymCell:
			v.visitCell(node, alst)

		case zsx.SymVerbatimCode:
			v.visitVerbatim(node, "```")
		case zsx.SymVerbatimComment:
			v.visitVerbatim(node, "%%%")
		case zsx.SymVerbatimEval:
			v.visitVerbatim(node, "~~~")
		case zsx.SymVerbatimHTML:
			v.visitVerbatim(node, "@@@")
		case zsx.SymVerbatimMath:
			v.visitVerbatim(node, "$$$")
		case zsx.SymVerbatimZettel:
			v.visitVerbatim(node, "@@@")

		case zsx.SymBLOB:
			v.visitBLOB(node)
		case zsx.SymTransclude:
			v.visitTransclude(node, alst)
		default:
			return false
		}
		return true
	}
	return false
}
func (v *zmkVisitor) VisitItAfter(*sx.Pair, *sx.Pair) {}

func (v *zmkVisitor) visitFormat(node *sx.Pair, alst *sx.Pair, delim string) {
	_, attrs, inlines := zsx.GetFormat(node)
	v.b.WriteString(delim)
	v.walkList(inlines, alst)
	v.b.WriteString(delim)
	v.writeAttributes(attrs)
}

func (v *zmkVisitor) writeLiteral(code byte, attrs *sx.Pair, content string) {
	v.b.WriteBytes(code, code)
	v.writeEscaped(content, code)
	v.b.WriteBytes(code, code)
	v.writeAttributes(attrs)
}

func (v *zmkVisitor) visitLink(node *sx.Pair, alst *sx.Pair) {
	attrs, ref, inlines := zsx.GetLink(node)
	v.b.WriteString("[[")
	if inlines != nil {
		v.walkList(inlines, alst)
		_ = v.b.WriteByte('|')
	}
	_ = sz.WriteReference(&v.b, ref)
	v.b.WriteString("]]")
	v.writeAttributes(attrs)
}

func (v *zmkVisitor) visitEmbedRef(node *sx.Pair, alst *sx.Pair) {
	attrs, ref, _, inlines := zsx.GetEmbed(node)
	v.b.WriteString("{{")
	if inlines != nil {
		v.walkList(inlines, alst)
		_ = v.b.WriteByte('|')
	}
	_ = sz.WriteReference(&v.b, ref)
	v.b.WriteString("}}")
	v.writeAttributes(attrs)
}

func (v *zmkVisitor) visitEndnote(node *sx.Pair, alst *sx.Pair) {
	attrs, inlines := zsx.GetEndnote(node)
	v.b.WriteString("[^")
	v.walkList(inlines, alst)
	_ = v.b.WriteByte(']')
	v.writeAttributes(attrs)
}

func (v *zmkVisitor) visitCite(node *sx.Pair, alst *sx.Pair) {
	attrs, key, inlines := zsx.GetCite(node)
	v.b.WriteStrings("[@", key)
	if inlines != nil {
		v.b.WriteSpace()
		v.walkList(inlines, alst)
	}
	_ = v.b.WriteByte(']')
	v.writeAttributes(attrs)
}

func (v *zmkVisitor) visitMark(node *sx.Pair, alst *sx.Pair) {
	mark, _, _, inlines := zsx.GetMark(node)
	v.b.WriteStrings("[!", mark)
	if inlines != nil {
		_ = v.b.WriteByte('|')
		v.walkList(inlines, alst)
	}
	_ = v.b.WriteByte(']')
}

func (v *zmkVisitor) visitBlock(node *sx.Pair, alst *sx.Pair) {
	blocks := zsx.GetBlock(node)
	lastWasParagraph := false
	first := true
	for bn := range blocks.Pairs() {
		blk := bn.Head()
		if first {
			first = false
		} else {
			v.b.WriteLn()
			if lastWasParagraph && alst.Assoc(zsx.SymRegionVerse) == nil {
				if zsx.SymPara.IsEqual(blk.Car()) {
					v.b.WriteLn()
				}
			}
		}
		v.walk(blk, alst)
		lastWasParagraph = zsx.SymPara.IsEqual(blk.Car())
	}
}

func (v *zmkVisitor) visitHeading(node *sx.Pair, alst *sx.Pair) {
	level, attrs, inlines, _, _ := zsx.GetHeading(node)
	const headingSigns = "========= "
	v.b.WriteString(headingSigns[len(headingSigns)-level-3:])
	v.walkList(inlines, alst)
	v.writeAttributes(attrs)
}

func (v *zmkVisitor) visitNestedList(node *sx.Pair, alst *sx.Pair, code byte) {
	_, _, items := zsx.GetList(node)
	v.prefix = append(v.prefix, code)

	first := true
	for itm := range items.Pairs() {
		if first {
			first = false
		} else {
			v.b.WriteLn()
		}
		item := zsx.GetBlock(itm.Head())
		if sym, isSymbol := sx.GetSymbol(item.Head().Car()); isSymbol &&
			!zsx.SymListOrdered.IsEqualSymbol(sym) &&
			!zsx.SymListUnordered.IsEqualSymbol(sym) &&
			!zsx.SymListQuote.IsEqualSymbol(sym) {

			_, _ = v.b.Write(v.prefix)
			v.b.WriteSpace()
		}

		second := false
		for inn := range item.Pairs() {
			inl := inn.Head()
			if second {
				v.b.WriteLn()
				if zsx.SymPara.IsEqual(inl.Car()) {
					v.writePrefixSpaces()
				}
			} else {
				second = true
			}
			v.walk(inl, alst)
		}
	}
	v.prefix = v.prefix[:len(v.prefix)-1]
}

func (v *zmkVisitor) visitRegion(node *sx.Pair, alst *sx.Pair, delim string) {
	sym, attrs, blocks, inlines := zsx.GetRegion(node)
	//TODO: Scan rn.Blocks for embedded regions to adjust length of regionCode
	v.b.WriteString(delim)
	v.writeAttributes(attrs)
	v.b.WriteLn()
	if zsx.SymRegionVerse.IsEqualSymbol(sym) {
		alst = alst.Cons(sx.Cons(zsx.SymRegionVerse, sx.Nil()))
	}
	v.walk(zsx.MakeBlockList(blocks), alst)
	v.b.WriteLn()
	v.b.WriteString(delim)
	if inlines != nil {
		v.b.WriteSpace()
		v.walkList(inlines, alst)
	}
}

func (v *zmkVisitor) visitDescription(node *sx.Pair, alst *sx.Pair) {
	_, termVals := zsx.GetDescription(node)
	first := true
	for n := termVals; n != nil; n = n.Tail() {
		if first {
			first = false
		} else {
			v.b.WriteLn()
		}
		v.b.WriteString("; ")
		if term := n.Head(); term != nil {
			v.walkList(term, alst)
		}
		n = n.Tail()
		if n == nil {
			break
		}
		for bns := range zsx.GetBlock(n.Head()).Pairs() {
			v.b.WriteString("\n: ")
			second := false
			for pn := range zsx.GetBlock(bns.Head()).Pairs() {
				if second {
					v.b.WriteString("\n\n  ")
				} else {
					second = true
				}
				v.walk(pn.Head(), alst)
			}
		}
	}
}

func (v *zmkVisitor) visitTable(node *sx.Pair, alst *sx.Pair) {
	_, headerRow, rows := zsx.GetTable(node)
	if headerRow != nil {
		v.writeRow(headerRow, alst, "|=")
		v.b.WriteLn()
	}
	first := true
	for row := range rows.Pairs() {
		if first {
			first = false
		} else {
			v.b.WriteLn()
		}
		v.writeRow(row.Head(), alst, "|")
	}
}
func (v *zmkVisitor) writeRow(row *sx.Pair, alst *sx.Pair, delim string) {
	for n := range row.Pairs() {
		v.b.WriteString(delim)
		v.walk(n.Head(), alst)
	}
}
func (v *zmkVisitor) visitCell(node *sx.Pair, alst *sx.Pair) {
	attrs, inlines := zsx.GetCell(node)
	align := ""
	if alignPair := attrs.Assoc(zsx.SymAttrAlign); alignPair != nil {
		if alignValue := alignPair.Cdr(); zsx.AttrAlignCenter.IsEqual(alignValue) {
			align = ":"
		} else if zsx.AttrAlignLeft.IsEqual(alignValue) {
			align = "<"
		} else if zsx.AttrAlignRight.IsEqual(alignValue) {
			align = ">"
		}
	}
	v.b.WriteString(align)
	v.walkList(inlines, alst)
}

func (v *zmkVisitor) visitVerbatim(node *sx.Pair, delim string) {
	sym, attrs, content := zsx.GetVerbatim(node)

	if zsx.SymVerbatimHTML.IsEqualSymbol(sym) {
		attrs = attrs.RemoveAssoc(sx.MakeString(meta.KeySyntax))
		attrs = attrs.Cons(sx.Cons(sx.MakeString(""), sx.MakeString(meta.ValueSyntaxHTML)))
	}

	// TODO: scan cn.Lines to find embedded kind[0]s at beginning
	v.b.WriteString(delim)
	v.writeAttributes(attrs)
	v.b.WriteLn()
	v.b.WriteString(content)
	v.b.WriteLn()
	v.b.WriteString(delim)
}

func (v *zmkVisitor) visitBLOB(node *sx.Pair) {
	_, syntax, content, inlines := zsx.GetBLOBuncode(node)
	if syntax == meta.ValueSyntaxSVG {
		v.b.WriteString("@@@")
		v.b.WriteStrings(syntax, "\n", content, "\n@@@\n")
		return
	}
	var sb strings.Builder
	var textEnc TextEncoder
	_ = textEnc.WriteSz(&sb, zsx.MakeInlineList(inlines))
	v.b.WriteStrings("%% Unable to display BLOB with description '", sb.String(), "' and syntax '", syntax, "'.")
}

func (v *zmkVisitor) visitTransclude(node *sx.Pair, alst *sx.Pair) {
	attrs, ref, inlines := zsx.GetTransclusion(node)
	v.b.WriteString("{{{")
	if inlines != nil {
		v.walkList(inlines, alst)
		_ = v.b.WriteByte('|')
	}
	_ = sz.WriteReference(&v.b, ref)
	v.b.WriteString("}}}")
	v.writeAttributes(attrs)
}

var escapeSeqs = set.New(
	"\\", "__", "**", "~~", "^^", ",,", ">>", `""`, "::", "''", "``", "++", "==", "##",
)

func (v *zmkVisitor) writeText(text string) {
	last := 0
	for i := 0; i < len(text); i++ {
		if b := text[i]; b == '\\' {
			v.b.WriteString(text[last:i])
			v.b.WriteBytes('\\', b)
			last = i + 1
			continue
		}
		if i < len(text)-1 {
			s := text[i : i+2]
			if escapeSeqs.Contains(s) {
				v.b.WriteString(text[last:i])
				for j := range len(s) {
					v.b.WriteBytes('\\', s[j])
				}
				i++
				last = i + 1
				continue
			}
		}
	}
	v.b.WriteString(text[last:])
}
func (v *zmkVisitor) writeBreak(isHard bool) {
	if isHard {
		v.b.WriteString("\\\n")
	} else {
		v.b.WriteLn()
	}
	v.writePrefixSpaces()
}

func (v *zmkVisitor) writeAttributes(attrs *sx.Pair) {
	a := zsx.GetAttributes(attrs)
	if a.IsEmpty() {
		return
	}
	_ = v.b.WriteByte('{')
	for i, k := range a.Keys() {
		if i > 0 {
			v.b.WriteSpace()
		}
		if k == zsx.DefaultAttribute {
			_ = v.b.WriteByte('-')
			continue
		}
		v.b.WriteString(k)
		if vl := a[k]; len(vl) > 0 {
			v.b.WriteString("=\"")
			v.writeEscaped(vl, '"')
			_ = v.b.WriteByte('"')
		}
	}
	_ = v.b.WriteByte('}')
}

func (v *zmkVisitor) writeEscaped(s string, toEscape byte) {
	last := 0
	for i := range len(s) {
		if b := s[i]; b == toEscape || b == '\\' {
			v.b.WriteString(s[last:i])
			v.b.WriteBytes('\\', b)
			last = i + 1
		}
	}
	v.b.WriteString(s[last:])
}

func (v *zmkVisitor) writePrefixSpaces() {
	if prefixLen := len(v.prefix); prefixLen > 0 {
		for i := 0; i <= prefixLen; i++ {
			v.b.WriteSpace()
		}
	}
}
