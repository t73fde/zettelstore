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

import (
	"encoding/base64"
	"fmt"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
)

// NewSzTransformer returns a new transformer to create s-expressions from AST nodes.
func NewSzTransformer() SzTransformer {
	return SzTransformer{}
}

// SzTransformer contains all data needed to transform into a s-expression.
type SzTransformer struct {
	inVerse bool
}

// GetSz transforms the given node into a sx list.
func (t *SzTransformer) GetSz(node ast.Node) *sx.Pair {
	switch n := node.(type) {
	case *ast.BlockSlice:
		return zsx.MakeBlockList(t.getBlockList(n))
	case *ast.InlineSlice:
		return zsx.MakeInlineList(t.getInlineList(*n))
	case *ast.ParaNode:
		return zsx.MakeParaList(t.getInlineList(n.Inlines))
	case *ast.VerbatimNode:
		return zsx.MakeVerbatim(mapGetS(mapVerbatimKindS, n.Kind), getAttributes(n.Attrs), string(n.Content))
	case *ast.RegionNode:
		return t.getRegion(n)
	case *ast.HeadingNode:
		return zsx.MakeHeading(n.Level, getAttributes(n.Attrs), t.getInlineList(n.Inlines), n.Slug, n.Fragment)
	case *ast.HRuleNode:
		return zsx.MakeThematic(getAttributes(n.Attrs))
	case *ast.NestedListNode:
		return t.getNestedList(n)
	case *ast.DescriptionListNode:
		return t.getDescriptionList(n)
	case *ast.TableNode:
		return t.getTable(n)
	case *ast.TranscludeNode:
		return zsx.MakeTransclusion(getAttributes(n.Attrs), getReference(n.Ref), t.getInlineList(n.Inlines))
	case *ast.BLOBNode:
		return t.getBLOB(n)
	case *ast.TextNode:
		return zsx.MakeText(n.Text)
	case *ast.BreakNode:
		if n.Hard {
			return zsx.MakeHard()
		}
		return zsx.MakeSoft()
	case *ast.LinkNode:
		return t.getLink(n)
	case *ast.EmbedRefNode:
		return zsx.MakeEmbed(getAttributes(n.Attrs), getReference(n.Ref), n.Syntax, t.getInlineList(n.Inlines))
	case *ast.EmbedBLOBNode:
		return t.getEmbedBLOB(n)
	case *ast.CiteNode:
		return zsx.MakeCite(getAttributes(n.Attrs), n.Key, t.getInlineList(n.Inlines))
	case *ast.FootnoteNode:
		return zsx.MakeEndnote(getAttributes(n.Attrs), t.getInlineList(n.Inlines))
	case *ast.MarkNode:
		return zsx.MakeMark(n.Mark, n.Slug, n.Fragment, t.getInlineList(n.Inlines))
	case *ast.FormatNode:
		return zsx.MakeFormat(mapGetS(mapFormatKindS, n.Kind), getAttributes(n.Attrs), t.getInlineList(n.Inlines))
	case *ast.LiteralNode:
		return zsx.MakeLiteral(mapGetS(mapLiteralKindS, n.Kind), getAttributes(n.Attrs), string(n.Content))
	}
	return sx.MakeList(zsx.SymUnknown, sx.MakeString(fmt.Sprintf("%T %v", node, node)))
}

var mapVerbatimKindS = map[ast.VerbatimKind]*sx.Symbol{
	ast.VerbatimZettel:  zsx.SymVerbatimZettel,
	ast.VerbatimCode:    zsx.SymVerbatimCode,
	ast.VerbatimEval:    zsx.SymVerbatimEval,
	ast.VerbatimMath:    zsx.SymVerbatimMath,
	ast.VerbatimComment: zsx.SymVerbatimComment,
	ast.VerbatimHTML:    zsx.SymVerbatimHTML,
}

var mapFormatKindS = map[ast.FormatKind]*sx.Symbol{
	ast.FormatEmph:   zsx.SymFormatEmph,
	ast.FormatStrong: zsx.SymFormatStrong,
	ast.FormatDelete: zsx.SymFormatDelete,
	ast.FormatInsert: zsx.SymFormatInsert,
	ast.FormatSuper:  zsx.SymFormatSuper,
	ast.FormatSub:    zsx.SymFormatSub,
	ast.FormatQuote:  zsx.SymFormatQuote,
	ast.FormatMark:   zsx.SymFormatMark,
	ast.FormatSpan:   zsx.SymFormatSpan,
}

var mapLiteralKindS = map[ast.LiteralKind]*sx.Symbol{
	ast.LiteralCode:    zsx.SymLiteralCode,
	ast.LiteralInput:   zsx.SymLiteralInput,
	ast.LiteralOutput:  zsx.SymLiteralOutput,
	ast.LiteralComment: zsx.SymLiteralComment,
	ast.LiteralMath:    zsx.SymLiteralMath,
}

var mapRegionKindS = map[ast.RegionKind]*sx.Symbol{
	ast.RegionSpan:  zsx.SymRegionBlock,
	ast.RegionQuote: zsx.SymRegionQuote,
	ast.RegionVerse: zsx.SymRegionVerse,
}

func (t *SzTransformer) getRegion(rn *ast.RegionNode) *sx.Pair {
	saveInVerse := t.inVerse
	if rn.Kind == ast.RegionVerse {
		t.inVerse = true
	}
	symBlocks := t.getBlockList(&rn.Blocks)
	t.inVerse = saveInVerse
	return zsx.MakeRegion(
		mapGetS(mapRegionKindS, rn.Kind),
		getAttributes(rn.Attrs), symBlocks,
		t.getInlineList(rn.Inlines),
	)
}

var mapNestedListKindS = map[ast.NestedListKind]*sx.Symbol{
	ast.NestedListOrdered:   zsx.SymListOrdered,
	ast.NestedListUnordered: zsx.SymListUnordered,
	ast.NestedListQuote:     zsx.SymListQuote,
}

func (t *SzTransformer) getNestedList(ln *ast.NestedListNode) *sx.Pair {
	var items sx.ListBuilder
	isCompact := isCompactList(ln.Items)
	for _, item := range ln.Items {
		if isCompact && len(item) > 0 {
			paragraph := t.GetSz(item[0])
			items.Add(zsx.MakeInlineList(paragraph.Tail()))
			continue
		}
		var itemObjs sx.ListBuilder
		for _, in := range item {
			itemObjs.Add(t.GetSz(in))
		}
		if isCompact {
			items.Add(zsx.MakeInlineList(itemObjs.List()))
		} else {
			items.Add(zsx.MakeBlockList(itemObjs.List()))
		}
	}
	return zsx.MakeList(mapGetS(mapNestedListKindS, ln.Kind), getAttributes(ln.Attrs), items.List())
}
func isCompactList(itemSlice []ast.ItemSlice) bool {
	for _, items := range itemSlice {
		if len(items) > 1 {
			return false
		}
		if len(items) == 1 {
			if _, ok := items[0].(*ast.ParaNode); !ok {
				return false
			}
		}
	}
	return true
}

func (t *SzTransformer) getDescriptionList(dn *ast.DescriptionListNode) *sx.Pair {
	var dlObjs sx.ListBuilder
	for _, def := range dn.Descriptions {
		dlObjs.Add(t.getInlineList(def.Term))
		var descObjs sx.ListBuilder
		for _, b := range def.Descriptions {
			var dVal sx.ListBuilder
			for _, dn := range b {
				dVal.Add(t.GetSz(dn))
			}
			descObjs.Add(zsx.MakeBlockList(dVal.List()))
		}
		dlObjs.Add(zsx.MakeBlockList(descObjs.List()))
	}
	return dlObjs.List().Cons(getAttributes(dn.Attrs)).Cons(zsx.SymDescription)
}

func (t *SzTransformer) getTable(tn *ast.TableNode) *sx.Pair {
	var lb sx.ListBuilder
	lb.AddN(zsx.SymTable, sx.Nil(), t.getHeader(tn.Header))
	for _, row := range tn.Rows {
		lb.Add(t.getRow(row))
	}
	return lb.List()
}
func (t *SzTransformer) getHeader(header ast.TableRow) *sx.Pair {
	if len(header) == 0 {
		return nil
	}
	return t.getRow(header)
}
func (t *SzTransformer) getRow(row ast.TableRow) *sx.Pair {
	var lb sx.ListBuilder
	for _, cell := range row {
		lb.Add(t.getCell(cell))
	}
	return lb.List()
}

func (t *SzTransformer) getCell(cell *ast.TableCell) *sx.Pair {
	var attrs *sx.Pair
	switch cell.Align {
	case ast.AlignCenter:
		attrs = sx.Cons(sx.Cons(zsx.SymAttrAlign, zsx.AttrAlignCenter), nil)
	case ast.AlignLeft:
		attrs = sx.Cons(sx.Cons(zsx.SymAttrAlign, zsx.AttrAlignLeft), nil)
	case ast.AlignRight:
		attrs = sx.Cons(sx.Cons(zsx.SymAttrAlign, zsx.AttrAlignRight), nil)
	}
	return zsx.MakeCell(attrs, t.getInlineList(cell.Inlines))
}

func (t *SzTransformer) getBLOB(bn *ast.BLOBNode) *sx.Pair {
	var content string
	if bn.Syntax == meta.ValueSyntaxSVG {
		content = string(bn.Blob)
	} else {
		content = getBase64String(bn.Blob)
	}
	return zsx.MakeBLOB(getAttributes(bn.Attrs), t.getInlineList(bn.Description), bn.Syntax, content)
}

func (t *SzTransformer) getLink(ln *ast.LinkNode) *sx.Pair {
	return zsx.MakeLink(
		getAttributes(ln.Attrs),
		getReference(ln.Ref),
		t.getInlineList(ln.Inlines),
	)
}

func (t *SzTransformer) getEmbedBLOB(en *ast.EmbedBLOBNode) *sx.Pair {
	var content string
	if en.Syntax == meta.ValueSyntaxSVG {
		content = string(en.Blob)
	} else {
		content = getBase64String(en.Blob)
	}
	return zsx.MakeEmbedBLOB(getAttributes(en.Attrs), en.Syntax, content, t.getInlineList(en.Inlines))
}

func (t *SzTransformer) getBlockList(bs *ast.BlockSlice) *sx.Pair {
	var lb sx.ListBuilder
	for _, n := range *bs {
		lb.Add(t.GetSz(n))
	}
	return lb.List()
}
func (t *SzTransformer) getInlineList(is ast.InlineSlice) *sx.Pair {
	var lb sx.ListBuilder
	for _, n := range is {
		lb.Add(t.GetSz(n))
	}
	return lb.List()
}

func getAttributes(a zsx.Attributes) *sx.Pair {
	if a.IsEmpty() {
		return sx.Nil()
	}
	keys := a.Keys()
	var lb sx.ListBuilder
	for _, k := range keys {
		lb.Add(sx.Cons(sx.MakeString(k), sx.MakeString(a[k])))
	}
	return lb.List()
}

var mapRefStateS = map[ast.RefState]*sx.Symbol{
	ast.RefStateInvalid:  zsx.SymRefStateInvalid,
	ast.RefStateZettel:   sz.SymRefStateZettel,
	ast.RefStateSelf:     zsx.SymRefStateSelf,
	ast.RefStateFound:    sz.SymRefStateFound,
	ast.RefStateBroken:   sz.SymRefStateBroken,
	ast.RefStateHosted:   zsx.SymRefStateHosted,
	ast.RefStateBased:    sz.SymRefStateBased,
	ast.RefStateQuery:    sz.SymRefStateQuery,
	ast.RefStateExternal: zsx.SymRefStateExternal,
}

func getReference(ref *ast.Reference) *sx.Pair {
	return sx.MakeList(mapGetS(mapRefStateS, ref.State), sx.MakeString(ref.Value))
}

var mapMetaTypeS = map[*meta.DescriptionType]*sx.Symbol{
	meta.TypeCredential: sz.SymTypeCredential,
	meta.TypeEmpty:      sz.SymTypeEmpty,
	meta.TypeID:         sz.SymTypeID,
	meta.TypeIDSet:      sz.SymTypeIDSet,
	meta.TypeNumber:     sz.SymTypeNumber,
	meta.TypeString:     sz.SymTypeString,
	meta.TypeTagSet:     sz.SymTypeTagSet,
	meta.TypeTimestamp:  sz.SymTypeTimestamp,
	meta.TypeURL:        sz.SymTypeURL,
	meta.TypeWord:       sz.SymTypeWord,
}

// GetMeta transforms the given metadata into a sx list.
func (t *SzTransformer) GetMeta(m *meta.Meta) *sx.Pair {
	var lb sx.ListBuilder
	lb.Add(sz.SymMeta)
	for key, val := range m.Computed() {
		ty := m.Type(key)
		symType := mapGetS(mapMetaTypeS, ty)
		var obj sx.Object
		if ty.IsSet {
			var setObjs sx.ListBuilder
			for _, val := range val.AsSlice() {
				setObjs.Add(sx.MakeString(val))
			}
			obj = setObjs.List()
		} else {
			obj = sx.MakeString(string(val))
		}
		lb.Add(sx.Nil().Cons(obj).Cons(sx.MakeSymbol(key)).Cons(symType))
	}
	return lb.List()
}

func mapGetS[T comparable](m map[T]*sx.Symbol, k T) *sx.Symbol {
	if result, found := m[k]; found {
		return result
	}
	return sx.MakeSymbol(fmt.Sprintf("**%v:NOT-FOUND**", k))
}

func getBase64String(data []byte) string {
	var sb strings.Builder
	encoder := base64.NewEncoder(base64.StdEncoding, &sb)
	_, err := encoder.Write(data)
	if err == nil {
		err = encoder.Close()
	}
	if err == nil {
		return sb.String()
	}
	return ""
}
