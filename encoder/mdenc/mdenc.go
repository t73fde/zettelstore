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

// Package mdenc encodes the abstract syntax tree back into Markdown.
package mdenc

import (
	"io"

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/encoder"
	"zettelstore.de/z/zettel/meta"
)

func init() {
	encoder.Register(api.EncoderMD, func(*encoder.CreateParameter) encoder.Encoder { return Create() })
}

// Create an encoder.
func Create() *Encoder { return &myME }

type Encoder struct{}

var myME Encoder

// WriteZettel writes the encoded zettel to the writer.
func (*Encoder) WriteZettel(w io.Writer, zn *ast.ZettelNode, evalMeta encoder.EvalMetaFunc) (int, error) {
	v := newVisitor(w)
	v.acceptMeta(zn.InhMeta, evalMeta)
	if zn.InhMeta.YamlSep {
		v.b.WriteString("---\n")
	} else {
		v.b.WriteByte('\n')
	}
	ast.Walk(v, &zn.Ast)
	length, err := v.b.Flush()
	return length, err
}

// WriteMeta encodes meta data as markdown.
func (*Encoder) WriteMeta(w io.Writer, m *meta.Meta, evalMeta encoder.EvalMetaFunc) (int, error) {
	v := newVisitor(w)
	v.acceptMeta(m, evalMeta)
	length, err := v.b.Flush()
	return length, err
}

func (v *visitor) acceptMeta(m *meta.Meta, evalMeta encoder.EvalMetaFunc) {
	for _, p := range m.ComputedPairs() {
		key := p.Key
		v.b.WriteStrings(key, ": ")
		if meta.Type(key) == meta.TypeZettelmarkup {
			is := evalMeta(p.Value)
			ast.Walk(v, &is)
		} else {
			v.b.WriteString(p.Value)
		}
		v.b.WriteByte('\n')
	}
}

func (ze *Encoder) WriteContent(w io.Writer, zn *ast.ZettelNode) (int, error) {
	return ze.WriteBlocks(w, &zn.Ast)
}

// WriteBlocks writes the content of a block slice to the writer.
func (*Encoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) (int, error) {
	v := newVisitor(w)
	ast.Walk(v, bs)
	length, err := v.b.Flush()
	return length, err
}

// WriteInlines writes an inline slice to the writer
func (*Encoder) WriteInlines(w io.Writer, is *ast.InlineSlice) (int, error) {
	v := newVisitor(w)
	ast.Walk(v, is)
	length, err := v.b.Flush()
	return length, err
}

// visitor writes the abstract syntax tree to an EncWriter.
type visitor struct {
	b          encoder.EncWriter
	listInfo   []int
	listPrefix string
}

func newVisitor(w io.Writer) *visitor {
	return &visitor{b: encoder.NewEncWriter(w)}
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
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

func (v *visitor) visitBlockSlice(bs *ast.BlockSlice) {
	for i, bn := range *bs {
		if i > 0 {
			v.b.WriteString("\n\n")
		}
		ast.Walk(v, bn)
	}
}

func (v *visitor) visitVerbatim(vn *ast.VerbatimNode) {
	lc := len(vn.Content)
	if vn.Kind != ast.VerbatimProg || lc == 0 {
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

func (v *visitor) visitRegion(rn *ast.RegionNode) {
	if rn.Kind != ast.RegionQuote {
		return
	}
	first := true
	for _, bn := range rn.Blocks {
		pn, ok := bn.(*ast.ParaNode)
		if !ok {
			continue
		}
		if !first {
			v.b.WriteString("\n\n")
		}
		first = false
		v.b.WriteString("> ")
		ast.Walk(v, &pn.Inlines)
	}
}

func (v *visitor) visitHeading(hn *ast.HeadingNode) {
	const headingSigns = "###### "
	v.b.WriteString(headingSigns[len(headingSigns)-hn.Level-1:])
	ast.Walk(v, &hn.Inlines)
}

func (v *visitor) visitNestedList(ln *ast.NestedListNode) {
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

func (v *visitor) writeNestedList(ln *ast.NestedListNode, enum string) {
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

func (v *visitor) writeListQuote(ln *ast.NestedListNode) {
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

func (v *visitor) visitBreak(bn *ast.BreakNode) {
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

func (v *visitor) visitLink(ln *ast.LinkNode) {
	v.writeReference(ln.Ref, ln.Inlines)
}

func (v *visitor) visitEmbedRef(en *ast.EmbedRefNode) {
	v.b.WriteByte('!')
	v.writeReference(en.Ref, en.Inlines)
}

func (v *visitor) writeReference(ref *ast.Reference, is ast.InlineSlice) {
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

func (v *visitor) visitFormat(fn *ast.FormatNode) {
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
		v.b.WriteString("<q>")
		ast.Walk(v, &fn.Inlines)
		v.b.WriteString("</q>")
	case ast.FormatMark:
		v.b.WriteString("<mark>")
		ast.Walk(v, &fn.Inlines)
		v.b.WriteString("</mark>")
	default:
		ast.Walk(v, &fn.Inlines)
	}
}

func (v *visitor) visitLiteral(ln *ast.LiteralNode) {
	switch ln.Kind {
	case ast.LiteralProg:
		v.b.WriteByte('`')
		v.b.Write(ln.Content)
		v.b.WriteByte('`')
	case ast.LiteralComment, ast.LiteralHTML: // ignore everything
	default:
		v.b.Write(ln.Content)
	}
}

func (v *visitor) writeSpaces(n int) {
	for range n {
		v.b.WriteByte(' ')
	}
}
