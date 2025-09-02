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

// textenc encodes the abstract syntax tree into its text.

import (
	"io"
	"iter"

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/ast"
)

// TextEncoder encodes just the text and ignores any formatting.
type TextEncoder struct{}

// WriteZettel writes metadata and content.
func (te *TextEncoder) WriteZettel(w io.Writer, zn *ast.ZettelNode) error {
	v := newTextVisitor(w)
	_ = te.WriteMeta(&v.b, zn.InhMeta)
	v.visitBlockSlice(&zn.BlocksAST)
	return v.b.Flush()
}

// WriteMeta encodes metadata as text.
func (te *TextEncoder) WriteMeta(w io.Writer, m *meta.Meta) error {
	buf := newEncWriter(w)
	for key, val := range m.Computed() {
		if meta.Type(key) == meta.TypeTagSet {
			writeTagSet(&buf, val.Elems())
		} else {
			buf.WriteString(string(val))
		}
		buf.WriteLn()
	}
	return buf.Flush()
}

func writeTagSet(buf *encWriter, tags iter.Seq[meta.Value]) {
	first := true
	for tag := range tags {
		if !first {
			buf.WriteSpace()
		}
		first = false
		buf.WriteString(string(tag.CleanTag()))
	}
}

// WriteBlocks writes the content of a block slice to the writer.
func (*TextEncoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) error {
	v := newTextVisitor(w)
	v.visitBlockSlice(bs)
	return v.b.Flush()
}

// WriteInlines writes an inline slice to the writer
func (*TextEncoder) WriteInlines(w io.Writer, is *ast.InlineSlice) error {
	v := newTextVisitor(w)
	ast.Walk(&v, is)
	return v.b.Flush()
}

// textVisitor writes the abstract syntax tree to an io.Writer.
type textVisitor struct {
	b         encWriter
	inlinePos int
}

func newTextVisitor(w io.Writer) textVisitor {
	return textVisitor{b: newEncWriter(w)}
}

func (v *textVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.BlockSlice:
		v.visitBlockSlice(n)
		return nil
	case *ast.InlineSlice:
		v.visitInlineSlice(n)
		return nil
	case *ast.VerbatimNode:
		v.visitVerbatim(n)
		return nil
	case *ast.RegionNode:
		v.visitBlockSlice(&n.Blocks)
		if len(n.Inlines) > 0 {
			v.b.WriteLn()
			ast.Walk(v, &n.Inlines)
		}
		return nil
	case *ast.NestedListNode:
		v.visitNestedList(n)
		return nil
	case *ast.DescriptionListNode:
		v.visitDescriptionList(n)
		return nil
	case *ast.TableNode:
		v.visitTable(n)
		return nil
	case *ast.TranscludeNode:
		ast.Walk(v, &n.Inlines)
	case *ast.BLOBNode:
		return nil
	case *ast.TextNode:
		v.visitText(n.Text)
		return nil
	case *ast.BreakNode:
		if n.Hard {
			v.b.WriteLn()
		} else {
			v.b.WriteSpace()
		}
		return nil
	case *ast.LinkNode:
		if len(n.Inlines) > 0 {
			ast.Walk(v, &n.Inlines)
		}
		return nil
	case *ast.MarkNode:
		if len(n.Inlines) > 0 {
			ast.Walk(v, &n.Inlines)
		}
		return nil
	case *ast.FootnoteNode:
		if v.inlinePos > 0 {
			v.b.WriteSpace()
		}
		// No 'return nil' to write text
	case *ast.LiteralNode:
		if n.Kind != ast.LiteralComment {
			_, _ = v.b.Write(n.Content)
		}
	}
	return v
}

func (v *textVisitor) visitVerbatim(vn *ast.VerbatimNode) {
	if vn.Kind != ast.VerbatimComment {
		_, _ = v.b.Write(vn.Content)
	}
}

func (v *textVisitor) visitNestedList(ln *ast.NestedListNode) {
	for i, item := range ln.Items {
		v.writePosChar(i, '\n')
		for j, it := range item {
			v.writePosChar(j, '\n')
			ast.Walk(v, it)
		}
	}
}

func (v *textVisitor) visitDescriptionList(dl *ast.DescriptionListNode) {
	for i, descr := range dl.Descriptions {
		v.writePosChar(i, '\n')
		ast.Walk(v, &descr.Term)
		for _, b := range descr.Descriptions {
			v.b.WriteLn()
			for k, d := range b {
				v.writePosChar(k, '\n')
				ast.Walk(v, d)
			}
		}
	}
}

func (v *textVisitor) visitTable(tn *ast.TableNode) {
	if len(tn.Header) > 0 {
		v.writeRow(tn.Header)
		v.b.WriteLn()
	}
	for i, row := range tn.Rows {
		v.writePosChar(i, '\n')
		v.writeRow(row)
	}
}

func (v *textVisitor) writeRow(row ast.TableRow) {
	for i, cell := range row {
		v.writePosChar(i, ' ')
		ast.Walk(v, &cell.Inlines)
	}
}

func (v *textVisitor) visitBlockSlice(bs *ast.BlockSlice) {
	for i, bn := range *bs {
		v.writePosChar(i, '\n')
		ast.Walk(v, bn)
	}
}

func (v *textVisitor) visitInlineSlice(is *ast.InlineSlice) {
	for i, in := range *is {
		v.inlinePos = i
		ast.Walk(v, in)
	}
	v.inlinePos = 0
}

func (v *textVisitor) visitText(s string) {
	spaceFound := false
	for _, ch := range s {
		if input.IsSpace(ch) {
			if !spaceFound {
				v.b.WriteSpace()
				spaceFound = true
			}
			continue
		}
		spaceFound = false
		v.b.WriteString(string(ch))
	}
}

func (v *textVisitor) writePosChar(pos int, ch byte) {
	if pos > 0 {
		_ = v.b.WriteByte(ch)
	}
}
