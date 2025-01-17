//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

// Package sztrans allows to transform a sz representation of text into an
// abstract syntax tree.
package sztrans

import (
	"fmt"
	"log"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/sz"
	"zettelstore.de/z/ast"
)

type transformer struct{}

// GetNode returns an AST node for a given sz representation.
func GetNode(pair *sx.Pair) (ast.Node, error) {
	if pair == nil {
		return nil, nil
	}
	var t transformer
	obj := sz.Walk(&t, pair, sx.Nil())
	if sxn, isNode := obj.(sxNode); isNode {
		return sxn.node, nil
	}
	return nil, fmt.Errorf("no AST for %v: %v", pair, obj)
}

func (t *transformer) VisitBefore(pair *sx.Pair, _ *sx.Pair) (sx.Object, bool) {
	if sym, isSymbol := sx.GetSymbol(pair.Car()); isSymbol {
		switch sym {
		case sz.SymText:
			if p := pair.Tail(); p != nil {
				if s, isString := sx.GetString(p.Car()); isString {
					return sxNode{&ast.TextNode{Text: s.GetValue()}}, true
				}
			}
		case sz.SymSoft:
			return sxNode{&ast.BreakNode{Hard: false}}, true
		case sz.SymHard:
			return sxNode{&ast.BreakNode{Hard: true}}, true
		case sz.SymLiteralProg:
			return handleLiteral(ast.LiteralProg, pair.Tail())
		case sz.SymLiteralComment:
			return handleLiteral(ast.LiteralComment, pair.Tail())
		case sz.SymLiteralInput:
			return handleLiteral(ast.LiteralInput, pair.Tail())
		case sz.SymLiteralMath:
			return handleLiteral(ast.LiteralMath, pair.Tail())
		case sz.SymLiteralOutput:
			return handleLiteral(ast.LiteralOutput, pair.Tail())
		case sz.SymThematic:
			return sxNode{&ast.HRuleNode{Attrs: sz.GetAttributes(pair.Tail())}}, true
		case sz.SymVerbatimComment:
			return handleVerbatim(ast.VerbatimComment, pair.Tail())
		case sz.SymVerbatimEval:
			return handleVerbatim(ast.VerbatimEval, pair.Tail())
		case sz.SymVerbatimHTML:
			return handleVerbatim(ast.VerbatimHTML, pair.Tail())
		case sz.SymVerbatimMath:
			return handleVerbatim(ast.VerbatimMath, pair.Tail())
		case sz.SymVerbatimProg:
			return handleVerbatim(ast.VerbatimProg, pair.Tail())
		case sz.SymVerbatimZettel:
			return handleVerbatim(ast.VerbatimZettel, pair.Tail())
		case sz.SymTransclude:
			return handleTransclude(pair.Tail())
		}
	}
	return sx.Nil(), false
}

func handleLiteral(kind ast.LiteralKind, rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		if curr := rest.Tail(); curr != nil {
			if s, isString := sx.GetString(curr.Car()); isString {
				return sxNode{&ast.LiteralNode{
					Kind:    kind,
					Attrs:   attrs,
					Content: []byte(s.GetValue())}}, true
			}
		}
	}
	return nil, false
}

func handleVerbatim(kind ast.VerbatimKind, rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		if curr := rest.Tail(); curr != nil {
			if s, isString := sx.GetString(curr.Car()); isString {
				return sxNode{&ast.VerbatimNode{
					Kind:    kind,
					Attrs:   attrs,
					Content: []byte(s.GetValue()),
				}}, true
			}
		}
	}
	return nil, false
}

func handleTransclude(rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		if curr := rest.Tail(); curr != nil {
			ref := collectReference(curr.Head())
			return sxNode{&ast.TranscludeNode{
				Attrs: attrs,
				Ref:   ref,
			}}, true
		}
	}
	return nil, false
}

func (t *transformer) VisitAfter(pair *sx.Pair, _ *sx.Pair) (sx.Object, bool) {
	if sym, isSymbol := sx.GetSymbol(pair.Car()); isSymbol {
		switch sym {
		case sz.SymBlock:
			bns := collectBlocks(pair.Tail())
			return sxNode{&bns}, true
		case sz.SymPara:
			return sxNode{&ast.ParaNode{Inlines: collectInlines(pair.Tail())}}, true
		case sz.SymHeading:
			return handleHeading(pair.Tail())
		case sz.SymListOrdered:
			return sxNode{&ast.NestedListNode{
				Kind:  ast.NestedListOrdered,
				Items: collectItemSlices(pair.Tail()),
				Attrs: nil}}, true
		case sz.SymListUnordered:
			return sxNode{&ast.NestedListNode{
				Kind:  ast.NestedListUnordered,
				Items: collectItemSlices(pair.Tail()),
				Attrs: nil}}, true
		case sz.SymListQuote:
			return sxNode{&ast.NestedListNode{
				Kind:  ast.NestedListQuote,
				Items: collectItemSlices(pair.Tail()),
				Attrs: nil}}, true
		case sz.SymDescription:
			return handleDescription(pair.Tail())
		case sz.SymTable:
			return handleTable(pair.Tail())
		case sz.SymCell:
			return handleCell(ast.AlignDefault, pair.Tail())
		case sz.SymCellCenter:
			return handleCell(ast.AlignCenter, pair.Tail())
		case sz.SymCellLeft:
			return handleCell(ast.AlignLeft, pair.Tail())
		case sz.SymCellRight:
			return handleCell(ast.AlignRight, pair.Tail())
		case sz.SymRegionBlock:
			return handleRegion(ast.RegionSpan, pair.Tail())
		case sz.SymRegionQuote:
			return handleRegion(ast.RegionQuote, pair.Tail())
		case sz.SymRegionVerse:
			return handleRegion(ast.RegionVerse, pair.Tail())

		case sz.SymLinkHosted:
			return handleLink(ast.RefStateHosted, pair.Tail())
		case sz.SymLinkInvalid:
			return handleLink(ast.RefStateInvalid, pair.Tail())
		case sz.SymLinkZettel:
			return handleLink(ast.RefStateZettel, pair.Tail())
		case sz.SymLinkSelf:
			return handleLink(ast.RefStateSelf, pair.Tail())
		case sz.SymLinkFound:
			return handleLink(ast.RefStateFound, pair.Tail())
		case sz.SymLinkBroken:
			return handleLink(ast.RefStateBroken, pair.Tail())
		case sz.SymLinkHosted:
			return handleLink(ast.RefStateHosted, pair.Tail())
		case sz.SymLinkBased:
			return handleLink(ast.RefStateBased, pair.Tail())
		case sz.SymLinkQuery:
			return handleLink(ast.RefStateQuery, pair.Tail())
		case sz.SymLinkExternal:
			return handleLink(ast.RefStateExternal, pair.Tail())
		case sz.SymEmbed:
			return handleEmbed(pair.Tail())
		case sz.SymCite:
			return handleCite(pair.Tail())
		case sz.SymMark:
			return handleMark(pair.Tail())
		case sz.SymEndnote:
			return handleEndnote(pair.Tail())
		case sz.SymFormatDelete:
			return handleFormat(ast.FormatDelete, pair.Tail())
		case sz.SymFormatEmph:
			return handleFormat(ast.FormatEmph, pair.Tail())
		case sz.SymFormatInsert:
			return handleFormat(ast.FormatInsert, pair.Tail())
		case sz.SymFormatMark:
			return handleFormat(ast.FormatMark, pair.Tail())
		case sz.SymFormatQuote:
			return handleFormat(ast.FormatQuote, pair.Tail())
		case sz.SymFormatSpan:
			return handleFormat(ast.FormatSpan, pair.Tail())
		case sz.SymFormatSub:
			return handleFormat(ast.FormatSub, pair.Tail())
		case sz.SymFormatSuper:
			return handleFormat(ast.FormatSuper, pair.Tail())
		case sz.SymFormatStrong:
			return handleFormat(ast.FormatStrong, pair.Tail())
		}
		log.Println("MISS", pair)
	}
	return sxNode{}, false
}

func collectBlocks(lst *sx.Pair) (result ast.BlockSlice) {
	for curr := lst; curr != nil; curr = curr.Tail() {
		if sxn, isNode := curr.Car().(sxNode); isNode {
			if bn, isInline := sxn.node.(ast.BlockNode); isInline {
				result = append(result, bn)
			}
		}
	}
	return result
}

func collectInlines(lst *sx.Pair) (result ast.InlineSlice) {
	for curr := lst; curr != nil; curr = curr.Tail() {
		if sxn, isNode := curr.Car().(sxNode); isNode {
			if in, isInline := sxn.node.(ast.InlineNode); isInline {
				result = append(result, in)
			}
		}
	}
	return result
}

func handleHeading(rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		if num, isNumber := rest.Car().(sx.Int64); isNumber && num > 0 && num < 6 {
			if curr := rest.Tail(); curr != nil {
				attrs := sz.GetAttributes(curr.Head())
				if curr = curr.Tail(); curr != nil {
					if sSlug, isSlug := sx.GetString(curr.Car()); isSlug {
						if curr = curr.Tail(); curr != nil {
							if sUniq, isUniq := sx.GetString(curr.Car()); isUniq {
								return sxNode{&ast.HeadingNode{
									Level:    int(num),
									Attrs:    attrs,
									Slug:     sSlug.GetValue(),
									Fragment: sUniq.GetValue(),
									Inlines:  collectInlines(curr.Tail()),
								}}, true
							}
						}
					}
				}
			}
		}
	}
	log.Println("HEAD", rest)
	return nil, false
}

func collectItemSlices(lst *sx.Pair) (result []ast.ItemSlice) {
	for curr := lst; curr != nil; curr = curr.Tail() {
		if sxn, isNode := curr.Car().(sxNode); isNode {
			if bns, isBlockSlice := sxn.node.(*ast.BlockSlice); isBlockSlice {
				items := make(ast.ItemSlice, len(*bns))
				for i, bn := range *bns {
					if it, ok := bn.(ast.ItemNode); ok {
						items[i] = it
					}
				}
				result = append(result, items)
			}
			if ins, isInline := sxn.node.(*ast.InlineSlice); isInline {
				items := make(ast.ItemSlice, len(*ins))
				for i, bn := range *ins {
					if it, ok := bn.(ast.ItemNode); ok {
						items[i] = it
					}
				}
				result = append(result, items)
			}
		}
	}
	return result
}

func handleDescription(rest *sx.Pair) (sx.Object, bool) {
	var descs []ast.Description
	for curr := rest; curr != nil; {
		term := collectInlines(curr.Head())
		curr = curr.Tail()
		if curr == nil {
			descr := ast.Description{Term: term, Descriptions: nil}
			descs = append(descs, descr)
			break
		}

		sxn, isNode := curr.Car().(sxNode)
		if !isNode {
			descs = nil
			break
		}
		blocks, isBlocks := sxn.node.(*ast.BlockSlice)
		if !isBlocks {
			descs = nil
			break
		}

		descSlice := make([]ast.DescriptionSlice, 0, len(*blocks))
		for _, bn := range *blocks {
			bns, isBns := bn.(*ast.BlockSlice)
			if !isBns {
				continue
			}
			ds := make(ast.DescriptionSlice, 0, len(*bns))
			for _, b := range *bns {
				if defNode, isDef := b.(ast.DescriptionNode); isDef {
					ds = append(ds, defNode)
				}
			}
			descSlice = append(descSlice, ds)
		}

		descr := ast.Description{Term: term, Descriptions: descSlice}
		descs = append(descs, descr)

		curr = curr.Tail()
	}
	if len(descs) > 0 {
		return sxNode{&ast.DescriptionListNode{Descriptions: descs}}, true
	}
	log.Println("DESC", rest)
	return nil, false
}

func handleTable(rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		header := collectRow(rest.Head())
		cols := len(header)

		var rows []ast.TableRow
		for curr := rest.Tail(); curr != nil; curr = curr.Tail() {
			row := collectRow(curr.Head())
			rows = append(rows, row)
			cols = max(cols, len(row))
		}
		align := make([]ast.Alignment, cols)
		for i := range cols {
			align[i] = ast.AlignDefault
		}

		return sxNode{&ast.TableNode{
			Header: header,
			Align:  align,
			Rows:   rows,
		}}, true
	}
	log.Println("TABL", rest)
	return nil, false
}

func collectRow(lst *sx.Pair) (row ast.TableRow) {
	for curr := lst; curr != nil; curr = curr.Tail() {
		if sxn, isNode := curr.Car().(sxNode); isNode {
			if cell, isCell := sxn.node.(*ast.TableCell); isCell {
				row = append(row, cell)
			}
		}
	}
	return row
}

func handleCell(align ast.Alignment, rest *sx.Pair) (sx.Object, bool) {
	return sxNode{&ast.TableCell{
		Align:   align,
		Inlines: collectInlines(rest),
	}}, true
}

func handleRegion(kind ast.RegionKind, rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		if curr := rest.Tail(); curr != nil {
			if blockList := curr.Head(); blockList != nil {
				return sxNode{&ast.RegionNode{
					Kind:    kind,
					Attrs:   attrs,
					Blocks:  collectBlocks(blockList),
					Inlines: collectInlines(curr.Tail()),
				}}, true
			}
		}
	}
	log.Println("REGI", rest)
	return nil, false
}

func handleLink(state ast.RefState, rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		if curr := rest.Tail(); curr != nil {
			if sref, isString := sx.GetString(curr.Car()); isString {
				ref := ast.ParseReference(sref.GetValue())
				ref.State = state
				ins := collectInlines(curr.Tail())
				return sxNode{&ast.LinkNode{
					Attrs:   attrs,
					Ref:     ref,
					Inlines: ins,
				}}, true
			}
		}
	}
	log.Println("LINK", state, rest)
	return nil, false
}

func handleEmbed(rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		if curr := rest.Tail(); curr != nil {
			if ref := collectReference(curr.Head()); ref != nil {
				return sxNode{&ast.EmbedRefNode{
					Attrs:   attrs,
					Ref:     ref,
					Syntax:  "",
					Inlines: collectInlines(curr.Tail()),
				}}, true
			}
		}
	}
	log.Println("EMBE", rest)
	return nil, false
}

func collectReference(pair *sx.Pair) *ast.Reference {
	if pair != nil {
		if sym, isSymbol := sx.GetSymbol(pair.Car()); isSymbol {
			if next := pair.Tail(); next != nil {
				if sRef, isString := sx.GetString(next.Car()); isString {
					ref := ast.ParseReference(sRef.GetValue())
					switch sym {
					case sz.SymRefStateInvalid:
						ref.State = ast.RefStateInvalid
					case sz.SymRefStateZettel:
						ref.State = ast.RefStateZettel
					case sz.SymRefStateSelf:
						ref.State = ast.RefStateSelf
					case sz.SymRefStateFound:
						ref.State = ast.RefStateFound
					case sz.SymRefStateBroken:
						ref.State = ast.RefStateBroken
					case sz.SymRefStateHosted:
						ref.State = ast.RefStateHosted
					case sz.SymRefStateBased:
						ref.State = ast.RefStateBased
					case sz.SymRefStateQuery:
						ref.State = ast.RefStateQuery
					case sz.SymRefStateExternal:
						ref.State = ast.RefStateExternal
					}
					return ref
				}
			}
		}
	}
	return nil
}

func handleCite(rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		if curr := rest.Tail(); curr != nil {
			if sKey, isString := sx.GetString(curr.Car()); isString {
				return sxNode{&ast.CiteNode{
					Attrs:   attrs,
					Key:     sKey.GetValue(),
					Inlines: collectInlines(curr.Tail()),
				}}, true
			}
		}
	}
	log.Println("CITE", rest)
	return nil, false
}

func handleMark(rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		if sMark, isMarkS := sx.GetString(rest.Car()); isMarkS {
			if curr := rest.Tail(); curr != nil {
				if sSlug, isSlug := sx.GetString(curr.Car()); isSlug {
					if curr = curr.Tail(); curr != nil {
						if sUniq, isUniq := sx.GetString(curr.Car()); isUniq {
							return sxNode{&ast.MarkNode{
								Mark:     sMark.GetValue(),
								Slug:     sSlug.GetValue(),
								Fragment: sUniq.GetValue(),
								Inlines:  collectInlines(curr.Tail()),
							}}, true
						}
					}
				}
			}
		}
	}
	log.Println("MARK", rest)
	return nil, false
}

func handleEndnote(rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		return sxNode{&ast.FootnoteNode{
			Attrs:   attrs,
			Inlines: collectInlines(rest.Tail()),
		}}, true
	}
	log.Println("ENDN", rest)
	return nil, false
}

func handleFormat(kind ast.FormatKind, rest *sx.Pair) (sx.Object, bool) {
	if rest != nil {
		attrs := sz.GetAttributes(rest.Head())
		return sxNode{&ast.FormatNode{
			Kind:    kind,
			Attrs:   attrs,
			Inlines: collectInlines(rest.Tail()),
		}}, true
	}
	log.Println("FORM", kind, rest)
	return nil, false
}

type sxNode struct {
	node ast.Node
}

func (sxNode) IsNil() bool        { return false }
func (sxNode) IsAtom() bool       { return true }
func (n sxNode) String() string   { return fmt.Sprintf("%T/%v", n.node, n.node) }
func (n sxNode) GoString() string { return n.String() }
func (n sxNode) IsEqual(other sx.Object) bool {
	return n.String() == other.String()
}
