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
	"t73f.de/r/zsc/input"

	"zettelstore.de/z/internal/ast"
)

// textEncoder contains all data needed for encoding.
type textEncoder struct{}

// WriteZettel writes metadata and content.
func (te *textEncoder) WriteZettel(w io.Writer, zn *ast.ZettelNode) (int, error) {
	v := newTextVisitor(w)
	te.WriteMeta(&v.b, zn.InhMeta)
	v.visitBlockSlice(&zn.BlocksAST)
	length, err := v.b.Flush()
	return length, err
}

// WriteMeta encodes metadata as text.
func (te *textEncoder) WriteMeta(w io.Writer, m *meta.Meta) (int, error) {
	buf := newEncWriter(w)
	for key, val := range m.Computed() {
		if meta.Type(key) == meta.TypeTagSet {
			writeTagSet(&buf, val.Elems())
		} else {
			buf.WriteString(string(val))
		}
		buf.WriteByte('\n')
	}
	length, err := buf.Flush()
	return length, err
}

func writeTagSet(buf *encWriter, tags iter.Seq[meta.Value]) {
	first := true
	for tag := range tags {
		if !first {
			buf.WriteByte(' ')
		}
		first = false
		buf.WriteString(string(tag.CleanTag()))
	}

}

// WriteContent encodes the zettel content.
func (te *textEncoder) WriteContent(w io.Writer, zn *ast.ZettelNode) (int, error) {
	return te.WriteBlocks(w, &zn.BlocksAST)
}

// WriteBlocks writes the content of a block slice to the writer.
func (*textEncoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) (int, error) {
	v := newTextVisitor(w)
	v.visitBlockSlice(bs)
	length, err := v.b.Flush()
	return length, err
}

// WriteInlines writes an inline slice to the writer
func (*textEncoder) WriteInlines(w io.Writer, is *ast.InlineSlice) (int, error) {
	v := newTextVisitor(w)
	ast.Walk(&v, is)
	length, err := v.b.Flush()
	return length, err
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
			v.b.WriteByte('\n')
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
			v.b.WriteByte('\n')
		} else {
			v.b.WriteByte(' ')
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
			v.b.WriteByte(' ')
		}
		// No 'return nil' to write text
	case *ast.LiteralNode:
		if n.Kind != ast.LiteralComment {
			v.b.Write(n.Content)
		}
	}
	return v
}

func (v *textVisitor) visitVerbatim(vn *ast.VerbatimNode) {
	if vn.Kind == ast.VerbatimComment {
		return
	}
	v.b.Write(vn.Content)
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
			v.b.WriteByte('\n')
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
		v.b.WriteByte('\n')
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
				v.b.WriteByte(' ')
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
		v.b.WriteByte(ch)
	}
}
