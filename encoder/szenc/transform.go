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

package szenc

import (
	"encoding/base64"
	"fmt"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/attrs"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"zettelstore.de/z/ast"
)

// NewTransformer returns a new transformer to create s-expressions from AST nodes.
func NewTransformer() *Transformer {
	t := Transformer{}
	return &t
}

// Transformer contains all data needed to transform into a s-expression.
type Transformer struct {
	inVerse bool
}

// GetSz transforms the given node into a sx list.
func (t *Transformer) GetSz(node ast.Node) *sx.Pair {
	switch n := node.(type) {
	case *ast.BlockSlice:
		return t.getBlockList(n).Cons(sz.SymBlock)
	case *ast.InlineSlice:
		return t.getInlineList(*n).Cons(sz.SymInline)
	case *ast.ParaNode:
		return t.getInlineList(n.Inlines).Cons(sz.SymPara)
	case *ast.VerbatimNode:
		return sx.MakeList(
			mapGetS(mapVerbatimKindS, n.Kind),
			getAttributes(n.Attrs),
			sx.MakeString(string(n.Content)),
		)
	case *ast.RegionNode:
		return t.getRegion(n)
	case *ast.HeadingNode:
		return t.getInlineList(n.Inlines).
			Cons(sx.MakeString(n.Fragment)).
			Cons(sx.MakeString(n.Slug)).
			Cons(getAttributes(n.Attrs)).
			Cons(sx.Int64(int64(n.Level))).
			Cons(sz.SymHeading)
	case *ast.HRuleNode:
		return sx.MakeList(sz.SymThematic, getAttributes(n.Attrs))
	case *ast.NestedListNode:
		return t.getNestedList(n)
	case *ast.DescriptionListNode:
		return t.getDescriptionList(n)
	case *ast.TableNode:
		return t.getTable(n)
	case *ast.TranscludeNode:
		return sx.MakeList(sz.SymTransclude, getAttributes(n.Attrs), getReference(n.Ref))
	case *ast.BLOBNode:
		return t.getBLOB(n)
	case *ast.TextNode:
		return sx.MakeList(sz.SymText, sx.MakeString(n.Text))
	case *ast.BreakNode:
		if n.Hard {
			return sx.MakeList(sz.SymHard)
		}
		return sx.MakeList(sz.SymSoft)
	case *ast.LinkNode:
		return t.getLink(n)
	case *ast.EmbedRefNode:
		return t.getInlineList(n.Inlines).
			Cons(sx.MakeString(n.Syntax)).
			Cons(getReference(n.Ref)).
			Cons(getAttributes(n.Attrs)).
			Cons(sz.SymEmbed)
	case *ast.EmbedBLOBNode:
		return t.getEmbedBLOB(n)
	case *ast.CiteNode:
		return t.getInlineList(n.Inlines).
			Cons(sx.MakeString(n.Key)).
			Cons(getAttributes(n.Attrs)).
			Cons(sz.SymCite)
	case *ast.FootnoteNode:
		// (ENDNODE attrs InlineElement ...)
		return t.getInlineList(n.Inlines).Cons(getAttributes(n.Attrs)).Cons(sz.SymEndnote)
	case *ast.MarkNode:
		return t.getInlineList(n.Inlines).
			Cons(sx.MakeString(n.Fragment)).
			Cons(sx.MakeString(n.Slug)).
			Cons(sx.MakeString(n.Mark)).
			Cons(sz.SymMark)
	case *ast.FormatNode:
		return t.getInlineList(n.Inlines).
			Cons(getAttributes(n.Attrs)).
			Cons(mapGetS(mapFormatKindS, n.Kind))
	case *ast.LiteralNode:
		return sx.MakeList(
			mapGetS(mapLiteralKindS, n.Kind),
			getAttributes(n.Attrs),
			sx.MakeString(string(n.Content)),
		)
	}
	return sx.MakeList(sz.SymUnknown, sx.MakeString(fmt.Sprintf("%T %v", node, node)))
}

var mapVerbatimKindS = map[ast.VerbatimKind]*sx.Symbol{
	ast.VerbatimZettel:  sz.SymVerbatimZettel,
	ast.VerbatimCode:    sz.SymVerbatimCode,
	ast.VerbatimEval:    sz.SymVerbatimEval,
	ast.VerbatimMath:    sz.SymVerbatimMath,
	ast.VerbatimComment: sz.SymVerbatimComment,
	ast.VerbatimHTML:    sz.SymVerbatimHTML,
}

var mapFormatKindS = map[ast.FormatKind]*sx.Symbol{
	ast.FormatEmph:   sz.SymFormatEmph,
	ast.FormatStrong: sz.SymFormatStrong,
	ast.FormatDelete: sz.SymFormatDelete,
	ast.FormatInsert: sz.SymFormatInsert,
	ast.FormatSuper:  sz.SymFormatSuper,
	ast.FormatSub:    sz.SymFormatSub,
	ast.FormatQuote:  sz.SymFormatQuote,
	ast.FormatMark:   sz.SymFormatMark,
	ast.FormatSpan:   sz.SymFormatSpan,
}

var mapLiteralKindS = map[ast.LiteralKind]*sx.Symbol{
	ast.LiteralCode:    sz.SymLiteralCode,
	ast.LiteralInput:   sz.SymLiteralInput,
	ast.LiteralOutput:  sz.SymLiteralOutput,
	ast.LiteralComment: sz.SymLiteralComment,
	ast.LiteralMath:    sz.SymLiteralMath,
}

var mapRegionKindS = map[ast.RegionKind]*sx.Symbol{
	ast.RegionSpan:  sz.SymRegionBlock,
	ast.RegionQuote: sz.SymRegionQuote,
	ast.RegionVerse: sz.SymRegionVerse,
}

func (t *Transformer) getRegion(rn *ast.RegionNode) *sx.Pair {
	saveInVerse := t.inVerse
	if rn.Kind == ast.RegionVerse {
		t.inVerse = true
	}
	symBlocks := t.getBlockList(&rn.Blocks)
	t.inVerse = saveInVerse
	return t.getInlineList(rn.Inlines).
		Cons(symBlocks).
		Cons(getAttributes(rn.Attrs)).
		Cons(mapGetS(mapRegionKindS, rn.Kind))
}

var mapNestedListKindS = map[ast.NestedListKind]*sx.Symbol{
	ast.NestedListOrdered:   sz.SymListOrdered,
	ast.NestedListUnordered: sz.SymListUnordered,
	ast.NestedListQuote:     sz.SymListQuote,
}

func (t *Transformer) getNestedList(ln *ast.NestedListNode) *sx.Pair {
	nlistObjs := make(sx.Vector, len(ln.Items)+1)
	nlistObjs[0] = mapGetS(mapNestedListKindS, ln.Kind)
	isCompact := isCompactList(ln.Items)
	for i, item := range ln.Items {
		if isCompact && len(item) > 0 {
			paragraph := t.GetSz(item[0])
			nlistObjs[i+1] = paragraph.Tail().Cons(sz.SymInline)
			continue
		}
		itemObjs := make(sx.Vector, len(item))
		for j, in := range item {
			itemObjs[j] = t.GetSz(in)
		}
		if isCompact {
			nlistObjs[i+1] = sx.MakeList(itemObjs...).Cons(sz.SymInline)
		} else {
			nlistObjs[i+1] = sx.MakeList(itemObjs...).Cons(sz.SymBlock)
		}
	}
	return sx.MakeList(nlistObjs...)
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

func (t *Transformer) getDescriptionList(dn *ast.DescriptionListNode) *sx.Pair {
	dlObjs := make(sx.Vector, 2*len(dn.Descriptions))
	for i, def := range dn.Descriptions {
		dlObjs[2*i] = t.getInlineList(def.Term)
		descObjs := make(sx.Vector, len(def.Descriptions))
		for j, b := range def.Descriptions {
			dVal := make(sx.Vector, len(b))
			for k, dn := range b {
				dVal[k] = t.GetSz(dn)
			}
			descObjs[j] = sx.MakeList(dVal...).Cons(sz.SymBlock)
		}
		dlObjs[2*i+1] = sx.MakeList(descObjs...).Cons(sz.SymBlock)
	}
	return sx.MakeList(dlObjs...).Cons(sz.SymDescription)
}

func (t *Transformer) getTable(tn *ast.TableNode) *sx.Pair {
	tObjs := make(sx.Vector, len(tn.Rows)+2)
	tObjs[0] = sz.SymTable
	tObjs[1] = t.getHeader(tn.Header)
	for i, row := range tn.Rows {
		tObjs[i+2] = t.getRow(row)
	}
	return sx.MakeList(tObjs...)
}
func (t *Transformer) getHeader(header ast.TableRow) *sx.Pair {
	if len(header) == 0 {
		return nil
	}
	return t.getRow(header)
}
func (t *Transformer) getRow(row ast.TableRow) *sx.Pair {
	rObjs := make(sx.Vector, len(row))
	for i, cell := range row {
		rObjs[i] = t.getCell(cell)
	}
	return sx.MakeList(rObjs...)
}

var alignmentSymbolS = map[ast.Alignment]*sx.Symbol{
	ast.AlignDefault: sz.SymCell,
	ast.AlignLeft:    sz.SymCellLeft,
	ast.AlignCenter:  sz.SymCellCenter,
	ast.AlignRight:   sz.SymCellRight,
}

func (t *Transformer) getCell(cell *ast.TableCell) *sx.Pair {
	return t.getInlineList(cell.Inlines).Cons(mapGetS(alignmentSymbolS, cell.Align))
}

func (t *Transformer) getBLOB(bn *ast.BLOBNode) *sx.Pair {
	var lastObj sx.Object
	if bn.Syntax == meta.ValueSyntaxSVG {
		lastObj = sx.MakeString(string(bn.Blob))
	} else {
		lastObj = getBase64String(bn.Blob)
	}
	return sx.MakeList(
		sz.SymBLOB,
		t.getInlineList(bn.Description),
		sx.MakeString(bn.Syntax),
		lastObj,
	)
}

var mapRefStateLink = map[ast.RefState]*sx.Symbol{
	ast.RefStateInvalid:  sz.SymLinkInvalid,
	ast.RefStateZettel:   sz.SymLinkZettel,
	ast.RefStateSelf:     sz.SymLinkSelf,
	ast.RefStateFound:    sz.SymLinkFound,
	ast.RefStateBroken:   sz.SymLinkBroken,
	ast.RefStateHosted:   sz.SymLinkHosted,
	ast.RefStateBased:    sz.SymLinkBased,
	ast.RefStateQuery:    sz.SymLinkQuery,
	ast.RefStateExternal: sz.SymLinkExternal,
}

func (t *Transformer) getLink(ln *ast.LinkNode) *sx.Pair {
	return t.getInlineList(ln.Inlines).
		Cons(sx.MakeString(ln.Ref.Value)).
		Cons(getAttributes(ln.Attrs)).
		Cons(mapGetS(mapRefStateLink, ln.Ref.State))
}

func (t *Transformer) getEmbedBLOB(en *ast.EmbedBLOBNode) *sx.Pair {
	tail := t.getInlineList(en.Inlines)
	if en.Syntax == meta.ValueSyntaxSVG {
		tail = tail.Cons(sx.MakeString(string(en.Blob)))
	} else {
		tail = tail.Cons(getBase64String(en.Blob))
	}
	return tail.Cons(sx.MakeString(en.Syntax)).Cons(getAttributes(en.Attrs)).Cons(sz.SymEmbedBLOB)
}

func (t *Transformer) getBlockList(bs *ast.BlockSlice) *sx.Pair {
	objs := make(sx.Vector, len(*bs))
	for i, n := range *bs {
		objs[i] = t.GetSz(n)
	}
	return sx.MakeList(objs...)
}
func (t *Transformer) getInlineList(is ast.InlineSlice) *sx.Pair {
	objs := make(sx.Vector, len(is))
	for i, n := range is {
		objs[i] = t.GetSz(n)
	}
	return sx.MakeList(objs...)
}

func getAttributes(a attrs.Attributes) sx.Object {
	if a.IsEmpty() {
		return sx.Nil()
	}
	keys := a.Keys()
	objs := make(sx.Vector, 0, len(keys))
	for _, k := range keys {
		objs = append(objs, sx.Cons(sx.MakeString(k), sx.MakeString(a[k])))
	}
	return sx.MakeList(objs...)
}

var mapRefStateS = map[ast.RefState]*sx.Symbol{
	ast.RefStateInvalid:  sz.SymRefStateInvalid,
	ast.RefStateZettel:   sz.SymRefStateZettel,
	ast.RefStateSelf:     sz.SymRefStateSelf,
	ast.RefStateFound:    sz.SymRefStateFound,
	ast.RefStateBroken:   sz.SymRefStateBroken,
	ast.RefStateHosted:   sz.SymRefStateHosted,
	ast.RefStateBased:    sz.SymRefStateBased,
	ast.RefStateQuery:    sz.SymRefStateQuery,
	ast.RefStateExternal: sz.SymRefStateExternal,
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
func (t *Transformer) GetMeta(m *meta.Meta) *sx.Pair {
	objs := sx.Vector{}
	for key, val := range m.Computed() {
		ty := m.Type(key)
		symType := mapGetS(mapMetaTypeS, ty)
		var obj sx.Object
		if ty.IsSet {
			setList := val.AsSlice()
			setObjs := make(sx.Vector, len(setList))
			for i, val := range setList {
				setObjs[i] = sx.MakeString(val)
			}
			obj = sx.MakeList(setObjs...)
		} else {
			obj = sx.MakeString(string(val))
		}
		objs = append(objs, sx.Nil().Cons(obj).Cons(sx.MakeSymbol(key)).Cons(symType))
	}
	return sx.MakeList(objs...).Cons(sz.SymMeta)
}

func mapGetS[T comparable](m map[T]*sx.Symbol, k T) *sx.Symbol {
	if result, found := m[k]; found {
		return result
	}
	return sx.MakeSymbol(fmt.Sprintf("**%v:NOT-FOUND**", k))
}

func getBase64String(data []byte) sx.String {
	var sb strings.Builder
	encoder := base64.NewEncoder(base64.StdEncoding, &sb)
	_, err := encoder.Write(data)
	if err == nil {
		err = encoder.Close()
	}
	if err == nil {
		return sx.MakeString(sb.String())
	}
	return sx.MakeString("")
}
