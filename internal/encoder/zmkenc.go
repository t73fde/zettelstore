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
	"fmt"
	"io"
	"strings"

	"t73f.de/r/zero/set"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
)

// zmkEncoder contains all data needed for encoding.
type zmkEncoder struct{}

// WriteZettel writes the encoded zettel to the writer.
func (*zmkEncoder) WriteZettel(w io.Writer, zn *ast.ZettelNode) error {
	v := newZmkVisitor(w)
	v.acceptMeta(zn.InhMeta)
	if zn.InhMeta.YamlSep {
		v.b.WriteString("---\n")
	} else {
		v.b.WriteLn()
	}
	ast.Walk(&v, &zn.BlocksAST)
	return v.b.Flush()
}

// WriteMeta encodes meta data as zmk.
func (*zmkEncoder) WriteMeta(w io.Writer, m *meta.Meta) error {
	v := newZmkVisitor(w)
	v.acceptMeta(m)
	return v.b.Flush()
}

func (v *zmkVisitor) acceptMeta(m *meta.Meta) {
	for key, val := range m.Computed() {
		v.b.WriteStrings(key, ": ", string(val), "\n")
	}
}

// WriteBlocks writes the content of a block slice to the writer.
func (*zmkEncoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) error {
	v := newZmkVisitor(w)
	ast.Walk(&v, bs)
	return v.b.Flush()
}

// zmkVisitor writes the abstract syntax tree to an io.Writer.
type zmkVisitor struct {
	b         encWriter
	textEnc   TextEncoder
	prefix    []byte
	inVerse   bool
	inlinePos int
}

func newZmkVisitor(w io.Writer) zmkVisitor { return zmkVisitor{b: newEncWriter(w)} }

func (v *zmkVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.BlockSlice:
		v.visitBlockSlice(n)
	case *ast.InlineSlice:
		for i, in := range *n {
			v.inlinePos = i
			ast.Walk(v, in)
		}
		v.inlinePos = 0
	case *ast.VerbatimNode:
		v.visitVerbatim(n)
	case *ast.RegionNode:
		v.visitRegion(n)
	case *ast.HeadingNode:
		v.visitHeading(n)
	case *ast.HRuleNode:
		v.b.WriteString("---")
		v.visitAttributes(n.Attrs)
	case *ast.NestedListNode:
		v.visitNestedList(n)
	case *ast.DescriptionListNode:
		v.visitDescriptionList(n)
	case *ast.TableNode:
		v.visitTable(n)
	case *ast.TranscludeNode:
		v.b.WriteStrings("{{{", n.Ref.String(), "}}}") // FIXME n.Inlines
		v.visitAttributes(n.Attrs)
	case *ast.BLOBNode:
		v.visitBLOB(n)
	case *ast.TextNode:
		v.visitText(n)
	case *ast.BreakNode:
		v.visitBreak(n)
	case *ast.LinkNode:
		v.visitLink(n)
	case *ast.EmbedRefNode:
		v.visitEmbedRef(n)
	case *ast.EmbedBLOBNode:
		v.visitEmbedBLOB(n)
	case *ast.CiteNode:
		v.visitCite(n)
	case *ast.FootnoteNode:
		v.b.WriteString("[^")
		ast.Walk(v, &n.Inlines)
		_ = v.b.WriteByte(']')
		v.visitAttributes(n.Attrs)
	case *ast.MarkNode:
		v.visitMark(n)
	case *ast.FormatNode:
		v.visitFormat(n)
	case *ast.LiteralNode:
		v.visitLiteral(n)
	default:
		return v
	}
	return nil
}

func (v *zmkVisitor) visitBlockSlice(bs *ast.BlockSlice) {
	var lastWasParagraph bool
	for i, bn := range *bs {
		if i > 0 {
			v.b.WriteLn()
			if lastWasParagraph && !v.inVerse {
				if _, ok := bn.(*ast.ParaNode); ok {
					v.b.WriteLn()
				}
			}
		}
		ast.Walk(v, bn)
		_, lastWasParagraph = bn.(*ast.ParaNode)
	}
}

var mapVerbatimKind = map[ast.VerbatimKind]string{
	ast.VerbatimZettel:  "@@@",
	ast.VerbatimComment: "%%%",
	ast.VerbatimHTML:    "@@@", // Attribute is set to {="html"}
	ast.VerbatimCode:    "```",
	ast.VerbatimEval:    "~~~",
	ast.VerbatimMath:    "$$$",
}

func (v *zmkVisitor) visitVerbatim(vn *ast.VerbatimNode) {
	kind, ok := mapVerbatimKind[vn.Kind]
	if !ok {
		panic(fmt.Sprintf("Unknown verbatim kind %d", vn.Kind))
	}
	attrs := vn.Attrs
	if vn.Kind == ast.VerbatimHTML {
		attrs = syntaxToHTML(attrs)
	}

	// TODO: scan cn.Lines to find embedded kind[0]s at beginning
	v.b.WriteString(kind)
	v.visitAttributes(attrs)
	v.b.WriteLn()
	_, _ = v.b.Write(vn.Content)
	v.b.WriteLn()
	v.b.WriteString(kind)
}

var mapRegionKind = map[ast.RegionKind]string{
	ast.RegionSpan:  ":::",
	ast.RegionQuote: "<<<",
	ast.RegionVerse: "\"\"\"",
}

func (v *zmkVisitor) visitRegion(rn *ast.RegionNode) {
	// Scan rn.Blocks for embedded regions to adjust length of regionCode
	kind, ok := mapRegionKind[rn.Kind]
	if !ok {
		panic(fmt.Sprintf("Unknown region kind %d", rn.Kind))
	}
	v.b.WriteString(kind)
	v.visitAttributes(rn.Attrs)
	v.b.WriteLn()
	saveInVerse := v.inVerse
	v.inVerse = rn.Kind == ast.RegionVerse
	ast.Walk(v, &rn.Blocks)
	v.inVerse = saveInVerse
	v.b.WriteLn()
	v.b.WriteString(kind)
	if len(rn.Inlines) > 0 {
		v.b.WriteSpace()
		ast.Walk(v, &rn.Inlines)
	}
}

func (v *zmkVisitor) visitHeading(hn *ast.HeadingNode) {
	const headingSigns = "========= "
	v.b.WriteString(headingSigns[len(headingSigns)-hn.Level-3:])
	ast.Walk(v, &hn.Inlines)
	v.visitAttributes(hn.Attrs)
}

var mapNestedListKind = map[ast.NestedListKind]byte{
	ast.NestedListOrdered:   '#',
	ast.NestedListUnordered: '*',
	ast.NestedListQuote:     '>',
}

func (v *zmkVisitor) visitNestedList(ln *ast.NestedListNode) {
	v.prefix = append(v.prefix, mapNestedListKind[ln.Kind])
	for i, item := range ln.Items {
		if i > 0 {
			v.b.WriteLn()
		}
		_, _ = v.b.Write(v.prefix)
		v.b.WriteSpace()
		for j, in := range item {
			if j > 0 {
				v.b.WriteLn()
				if _, ok := in.(*ast.ParaNode); ok {
					v.writePrefixSpaces()
				}
			}
			ast.Walk(v, in)
		}
	}
	v.prefix = v.prefix[:len(v.prefix)-1]
}

func (v *zmkVisitor) writePrefixSpaces() {
	if prefixLen := len(v.prefix); prefixLen > 0 {
		for i := 0; i <= prefixLen; i++ {
			v.b.WriteSpace()
		}
	}
}

func (v *zmkVisitor) visitDescriptionList(dn *ast.DescriptionListNode) {
	for i, descr := range dn.Descriptions {
		if i > 0 {
			v.b.WriteLn()
		}
		v.b.WriteString("; ")
		ast.Walk(v, &descr.Term)

		for _, b := range descr.Descriptions {
			v.b.WriteString("\n: ")
			for jj, dn := range b {
				if jj > 0 {
					v.b.WriteString("\n\n  ")
				}
				ast.Walk(v, dn)
			}
		}
	}
}

var alignCode = map[ast.Alignment]string{
	ast.AlignDefault: "",
	ast.AlignLeft:    "<",
	ast.AlignCenter:  ":",
	ast.AlignRight:   ">",
}

func (v *zmkVisitor) visitTable(tn *ast.TableNode) {
	if header := tn.Header; len(header) > 0 {
		v.writeTableHeader(header, tn.Align)
		v.b.WriteLn()
	}
	for i, row := range tn.Rows {
		if i > 0 {
			v.b.WriteLn()
		}
		v.writeTableRow(row, tn.Align)
	}
}

func (v *zmkVisitor) writeTableHeader(header ast.TableRow, align []ast.Alignment) {
	for pos, cell := range header {
		v.b.WriteString("|=")
		colAlign := align[pos]
		if cell.Align != colAlign {
			v.b.WriteString(alignCode[cell.Align])
		}
		ast.Walk(v, &cell.Inlines)
		if colAlign != ast.AlignDefault {
			v.b.WriteString(alignCode[colAlign])
		}
	}
}

func (v *zmkVisitor) writeTableRow(row ast.TableRow, align []ast.Alignment) {
	for pos, cell := range row {
		_ = v.b.WriteByte('|')
		if cell.Align != align[pos] {
			v.b.WriteString(alignCode[cell.Align])
		}
		ast.Walk(v, &cell.Inlines)
	}
}

func (v *zmkVisitor) visitBLOB(bn *ast.BLOBNode) {
	if bn.Syntax == meta.ValueSyntaxSVG {
		v.b.WriteStrings("@@@", bn.Syntax, "\n")
		_, _ = v.b.Write(bn.Blob)
		v.b.WriteString("\n@@@\n")
		return
	}
	var sb strings.Builder
	_ = v.textEnc.WriteInlines(&sb, &bn.Description)
	v.b.WriteStrings("%% Unable to display BLOB with description '", sb.String(), "' and syntax '", bn.Syntax, "'.")
}

var escapeSeqs = set.New(
	"\\", "__", "**", "~~", "^^", ",,", ">>", `""`, "::", "''", "``", "++", "==", "##",
)

func (v *zmkVisitor) visitText(tn *ast.TextNode) {
	last := 0
	for i := 0; i < len(tn.Text); i++ {
		if b := tn.Text[i]; b == '\\' {
			v.b.WriteString(tn.Text[last:i])
			v.b.WriteBytes('\\', b)
			last = i + 1
			continue
		}
		if i < len(tn.Text)-1 {
			s := tn.Text[i : i+2]
			if escapeSeqs.Contains(s) {
				v.b.WriteString(tn.Text[last:i])
				for j := range len(s) {
					v.b.WriteBytes('\\', s[j])
				}
				i++
				last = i + 1
				continue
			}
		}
	}
	v.b.WriteString(tn.Text[last:])
}

func (v *zmkVisitor) visitBreak(bn *ast.BreakNode) {
	if bn.Hard {
		v.b.WriteString("\\\n")
	} else {
		v.b.WriteLn()
	}
	v.writePrefixSpaces()
}

func (v *zmkVisitor) visitLink(ln *ast.LinkNode) {
	v.b.WriteString("[[")
	if len(ln.Inlines) > 0 {
		ast.Walk(v, &ln.Inlines)
		_ = v.b.WriteByte('|')
	}
	if ln.Ref.State == ast.RefStateBased {
		_ = v.b.WriteByte('/')
	}
	v.b.WriteStrings(ln.Ref.String(), "]]")
}

func (v *zmkVisitor) visitEmbedRef(en *ast.EmbedRefNode) {
	v.b.WriteString("{{")
	if len(en.Inlines) > 0 {
		ast.Walk(v, &en.Inlines)
		_ = v.b.WriteByte('|')
	}
	v.b.WriteStrings(en.Ref.String(), "}}")
}

func (v *zmkVisitor) visitEmbedBLOB(en *ast.EmbedBLOBNode) {
	if en.Syntax == meta.ValueSyntaxSVG {
		v.b.WriteString("@@")
		_, _ = v.b.Write(en.Blob)
		v.b.WriteStrings("@@{=", en.Syntax, "}")
		return
	}
	v.b.WriteString("{{TODO: display inline BLOB}}")
}

func (v *zmkVisitor) visitCite(cn *ast.CiteNode) {
	v.b.WriteStrings("[@", cn.Key)
	if len(cn.Inlines) > 0 {
		v.b.WriteSpace()
		ast.Walk(v, &cn.Inlines)
	}
	_ = v.b.WriteByte(']')
	v.visitAttributes(cn.Attrs)
}

func (v *zmkVisitor) visitMark(mn *ast.MarkNode) {
	v.b.WriteStrings("[!", mn.Mark)
	if len(mn.Inlines) > 0 {
		_ = v.b.WriteByte('|')
		ast.Walk(v, &mn.Inlines)
	}
	_ = v.b.WriteByte(']')

}

var mapFormatKind = map[ast.FormatKind][]byte{
	ast.FormatEmph:   []byte("__"),
	ast.FormatStrong: []byte("**"),
	ast.FormatInsert: []byte(">>"),
	ast.FormatDelete: []byte("~~"),
	ast.FormatSuper:  []byte("^^"),
	ast.FormatSub:    []byte(",,"),
	ast.FormatQuote:  []byte(`""`),
	ast.FormatMark:   []byte("##"),
	ast.FormatSpan:   []byte("::"),
}

func (v *zmkVisitor) visitFormat(fn *ast.FormatNode) {
	kind, ok := mapFormatKind[fn.Kind]
	if !ok {
		panic(fmt.Sprintf("Unknown format kind %d", fn.Kind))
	}
	_, _ = v.b.Write(kind)
	ast.Walk(v, &fn.Inlines)
	_, _ = v.b.Write(kind)
	v.visitAttributes(fn.Attrs)
}

func (v *zmkVisitor) visitLiteral(ln *ast.LiteralNode) {
	switch ln.Kind {
	case ast.LiteralCode:
		v.writeLiteral('`', ln.Attrs, ln.Content)
	case ast.LiteralMath:
		v.b.WriteStrings("$$", string(ln.Content), "$$")
		v.visitAttributes(ln.Attrs)
	case ast.LiteralInput:
		v.writeLiteral('\'', ln.Attrs, ln.Content)
	case ast.LiteralOutput:
		v.writeLiteral('=', ln.Attrs, ln.Content)
	case ast.LiteralComment:
		v.b.WriteString("%%")
		v.visitAttributes(ln.Attrs)
		v.b.WriteSpace()
		_, _ = v.b.Write(ln.Content)
	default:
		panic(fmt.Sprintf("Unknown literal kind %v", ln.Kind))
	}
}

func (v *zmkVisitor) writeLiteral(code byte, a zsx.Attributes, content []byte) {
	v.b.WriteBytes(code, code)
	v.writeEscaped(string(content), code)
	v.b.WriteBytes(code, code)
	v.visitAttributes(a)
}

// visitAttributes write HTML attributes
func (v *zmkVisitor) visitAttributes(a zsx.Attributes) {
	if a.IsEmpty() {
		return
	}
	_ = v.b.WriteByte('{')
	for i, k := range a.Keys() {
		if i > 0 {
			v.b.WriteSpace()
		}
		if k == "-" {
			_ = v.b.WriteByte('-')
			continue
		}
		v.b.WriteString(k)
		if vl := a[k]; len(vl) > 0 {
			v.b.WriteStrings("=\"", vl)
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

func syntaxToHTML(a zsx.Attributes) zsx.Attributes {
	return a.Clone().Set("", meta.ValueSyntaxHTML).Remove(meta.KeySyntax)
}
