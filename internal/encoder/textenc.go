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

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/ast"
)

// TextEncoder encodes just the text and ignores any formatting.
type TextEncoder struct{}

// WriteZettel writes metadata and content.
func (te *TextEncoder) WriteZettel(w io.Writer, zn *ast.Zettel) error {
	v := newTextVisitor(w)
	_ = te.WriteMeta(&v.b, zn.InhMeta)
	v.walk(zn.Blocks, nil)
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
	v := newTextVisitorAST(w)
	v.visitBlockSlice(bs)
	return v.b.Flush()
}

// WriteInlines writes an inline slice to the writer
func (*TextEncoder) WriteInlines(w io.Writer, is *ast.InlineSlice) error {
	v := newTextVisitorAST(w)
	ast.Walk(&v, is)
	return v.b.Flush()
}

// WriteSz writes SZ encoded content to the writer.
func (*TextEncoder) WriteSz(w io.Writer, node *sx.Pair) error {
	v := newTextVisitor(w)
	v.walk(node, nil)
	return v.b.Flush()
}

// textVisitor writes the sx.Object-based AST to an io.Writer.
type textVisitor struct{ b encWriter }

func newTextVisitor(w io.Writer) textVisitor {
	return textVisitor{b: newEncWriter(w)}
}
func (v *textVisitor) walk(node, alst *sx.Pair)    { zsx.WalkIt(v, node, alst) }
func (v *textVisitor) walkList(lst, alst *sx.Pair) { zsx.WalkItList(v, lst, 0, alst) }
func (v *textVisitor) VisitItBefore(node *sx.Pair, alst *sx.Pair) bool {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymText:
			s := zsx.GetText(node)
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

		case zsx.SymHard:
			v.b.WriteLn()
		case zsx.SymSoft:
			_ = v.b.WriteByte(' ')

		case zsx.SymEndnote:
			if zsx.GetWalkPos(alst) > 0 {
				_ = v.b.WriteByte(' ')
			}
			return false

		case zsx.SymLiteralCode, zsx.SymLiteralInput, zsx.SymLiteralMath, zsx.SymLiteralOutput:
			if s, found := sx.GetString(node.Tail().Tail().Car()); found {
				v.b.WriteString(s.GetValue())
			}
		case zsx.SymLiteralComment:
			// Do nothing

		case zsx.SymBlock:
			blocks := zsx.GetBlock(node)
			first := true
			for n := range blocks.Pairs() {
				if first {
					first = false
				} else {
					v.b.WriteLn()
				}
				v.walk(n.Head(), alst)
			}
		case zsx.SymInline:
			inlines := zsx.GetInline(node)
			first := true
			for n := range inlines.Pairs() {
				if first {
					first = false
				} else {
					v.b.WriteLn()
				}
				v.walk(n.Head(), alst)
			}

		case zsx.SymListOrdered, zsx.SymListUnordered, zsx.SymListQuote:
			_, _, items := zsx.GetList(node)
			first := true
			for n := range items.Pairs() {
				if first {
					first = false
				} else {
					v.b.WriteLn()
				}
				v.walk(n.Head(), alst)
			}

		case zsx.SymTable:
			_, header, rowList := zsx.GetTable(node)
			firstRow := true
			for n := range rowList.Cons(header).Pairs() {
				row := n.Head()
				if row == nil {
					continue
				}
				if firstRow {
					firstRow = false
				} else {
					v.b.WriteLn()
				}
				firstCell := true
				for elem := range row.Pairs() {
					if firstCell {
						firstCell = false
					} else {
						_ = v.b.WriteByte(' ')
					}
					v.walk(elem.Head(), alst)
				}
			}

		case zsx.SymDescription:
			_, termsVals := zsx.GetDescription(node)
			first := true
			for n := termsVals; n != nil; n = n.Tail() {
				if first {
					first = false
				} else {
					v.b.WriteLn()
				}
				v.walkList(n.Head(), alst)
				n = n.Tail()
				if n == nil {
					break
				}
				dvals := n.Head()
				if zsx.SymBlock.IsEqual(dvals.Car()) {
					for val := range dvals.Tail().Pairs() {
						v.b.WriteLn()
						v.walk(val.Head(), alst)
					}
				}
			}

		case zsx.SymRegionBlock, zsx.SymRegionQuote, zsx.SymRegionVerse:
			_, _, content, inlines := zsx.GetRegion(node)
			first := true
			for n := range content.Pairs() {
				if first {
					first = false
				} else {
					v.b.WriteLn()
				}
				v.walk(n.Head(), alst)
			}
			if inlines != nil {
				v.b.WriteLn()
				v.walkList(inlines, alst)
			}

		case zsx.SymVerbatimCode, zsx.SymVerbatimEval, zsx.SymVerbatimHTML, zsx.SymVerbatimMath, zsx.SymVerbatimZettel:
			_, _, s := zsx.GetVerbatim(node)
			v.b.WriteString(s)

		case zsx.SymVerbatimComment, zsx.SymBLOB:
			// Do nothing

		default:
			return false
		}
		return true
	}
	return false
}
func (v *textVisitor) VisitItAfter(*sx.Pair, *sx.Pair) {}

// textVisitorAST writes the abstract syntax tree to an io.Writer.
type textVisitorAST struct {
	b         encWriter
	inlinePos int
}

func newTextVisitorAST(w io.Writer) textVisitorAST {
	return textVisitorAST{b: newEncWriter(w)}
}

func (v *textVisitorAST) Visit(node ast.Node) ast.Visitor {
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

func (v *textVisitorAST) visitVerbatim(vn *ast.VerbatimNode) {
	if vn.Kind != ast.VerbatimComment {
		_, _ = v.b.Write(vn.Content)
	}
}

func (v *textVisitorAST) visitNestedList(ln *ast.NestedListNode) {
	for i, item := range ln.Items {
		v.writePosChar(i, '\n')
		for j, it := range item {
			v.writePosChar(j, '\n')
			ast.Walk(v, it)
		}
	}
}

func (v *textVisitorAST) visitDescriptionList(dl *ast.DescriptionListNode) {
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

func (v *textVisitorAST) visitTable(tn *ast.TableNode) {
	if len(tn.Header) > 0 {
		v.writeRow(tn.Header)
		v.b.WriteLn()
	}
	for i, row := range tn.Rows {
		v.writePosChar(i, '\n')
		v.writeRow(row)
	}
}

func (v *textVisitorAST) writeRow(row ast.TableRow) {
	for i, cell := range row {
		v.writePosChar(i, ' ')
		ast.Walk(v, &cell.Inlines)
	}
}

func (v *textVisitorAST) visitBlockSlice(bs *ast.BlockSlice) {
	for i, bn := range *bs {
		v.writePosChar(i, '\n')
		ast.Walk(v, bn)
	}
}

func (v *textVisitorAST) visitInlineSlice(is *ast.InlineSlice) {
	for i, in := range *is {
		v.inlinePos = i
		ast.Walk(v, in)
	}
	v.inlinePos = 0
}

func (v *textVisitorAST) visitText(s string) {
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

func (v *textVisitorAST) writePosChar(pos int, ch byte) {
	if pos > 0 {
		_ = v.b.WriteByte(ch)
	}
}
