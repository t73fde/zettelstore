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

// Package textenc encodes the abstract syntax tree into its text.
package textenc

import (
	"io"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/encoder"
)

func init() {
	encoder.Register(api.EncoderText, func(*encoder.CreateParameter) encoder.Encoder { return Create() })
}

// Create an encoder.
func Create() *Encoder { return &myTE }

// Encoder contains all data needed for encoding.
type Encoder struct{}

var myTE Encoder // Only a singleton is required.

// WriteZettel writes metadata and content.
func (te *Encoder) WriteZettel(w io.Writer, zn *ast.ZettelNode) (int, error) {
	v := newVisitor(w)
	te.WriteMeta(&v.b, zn.InhMeta)
	v.visitBlockSlice(&zn.Ast)
	length, err := v.b.Flush()
	return length, err
}

// WriteMeta encodes metadata as text.
func (te *Encoder) WriteMeta(w io.Writer, m *meta.Meta) (int, error) {
	buf := encoder.NewEncWriter(w)
	for _, pair := range m.ComputedPairs() {
		if meta.Type(pair.Key) == meta.TypeTagSet {
			writeTagSet(&buf, meta.ListFromValue(pair.Value))
		} else {
			buf.WriteString(pair.Value)
		}
		buf.WriteByte('\n')
	}
	length, err := buf.Flush()
	return length, err
}

func writeTagSet(buf *encoder.EncWriter, tags []string) {
	for i, tag := range tags {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(meta.CleanTag(tag))
	}

}

// WriteContent encodes the zettel content.
func (te *Encoder) WriteContent(w io.Writer, zn *ast.ZettelNode) (int, error) {
	return te.WriteBlocks(w, &zn.Ast)
}

// WriteBlocks writes the content of a block slice to the writer.
func (*Encoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) (int, error) {
	v := newVisitor(w)
	v.visitBlockSlice(bs)
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

// visitor writes the abstract syntax tree to an io.Writer.
type visitor struct {
	b         encoder.EncWriter
	inlinePos int
}

func newVisitor(w io.Writer) *visitor {
	return &visitor{b: encoder.NewEncWriter(w)}
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
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
	case *ast.TranscludeNode, *ast.BLOBNode:
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

func (v *visitor) visitVerbatim(vn *ast.VerbatimNode) {
	if vn.Kind == ast.VerbatimComment {
		return
	}
	v.b.Write(vn.Content)
}

func (v *visitor) visitNestedList(ln *ast.NestedListNode) {
	for i, item := range ln.Items {
		v.writePosChar(i, '\n')
		for j, it := range item {
			v.writePosChar(j, '\n')
			ast.Walk(v, it)
		}
	}
}

func (v *visitor) visitDescriptionList(dl *ast.DescriptionListNode) {
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

func (v *visitor) visitTable(tn *ast.TableNode) {
	if len(tn.Header) > 0 {
		v.writeRow(tn.Header)
		v.b.WriteByte('\n')
	}
	for i, row := range tn.Rows {
		v.writePosChar(i, '\n')
		v.writeRow(row)
	}
}

func (v *visitor) writeRow(row ast.TableRow) {
	for i, cell := range row {
		v.writePosChar(i, ' ')
		ast.Walk(v, &cell.Inlines)
	}
}

func (v *visitor) visitBlockSlice(bs *ast.BlockSlice) {
	for i, bn := range *bs {
		v.writePosChar(i, '\n')
		ast.Walk(v, bn)
	}
}

func (v *visitor) visitInlineSlice(is *ast.InlineSlice) {
	for i, in := range *is {
		v.inlinePos = i
		ast.Walk(v, in)
	}
	v.inlinePos = 0
}

func (v *visitor) visitText(s string) {
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

func (v *visitor) writePosChar(pos int, ch byte) {
	if pos > 0 {
		v.b.WriteByte(ch)
	}
}
