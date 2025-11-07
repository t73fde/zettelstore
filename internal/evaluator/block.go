//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

// Package evaluator interprets and evaluates the AST.
package evaluator

import (
	"errors"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/zettel"
)

func (e *evaluator) evalVerbatimEval(node *sx.Pair) *sx.Pair {
	_, attrs, content := zsx.GetVerbatim(node)
	if p := attrs.Assoc(sx.MakeString("")); p != nil {
		if s, isString := sx.GetString(p.Cdr()); isString && s.GetValue() == meta.ValueSyntaxDraw {
			return parser.ParseDrawBlock(attrs, []byte(content))
		}
	}
	return node
}

func (e *evaluator) evalVerbatimZettel(vn *sx.Pair) *sx.Pair {
	_, attrs, content := zsx.GetVerbatim(vn)
	m := meta.New(id.Invalid)
	m.Set(meta.KeySyntax, getSyntax(attrs, meta.ValueSyntaxText))
	zettel := zettel.Zettel{
		Meta:    m,
		Content: zettel.NewContent([]byte(content)),
	}
	e.transcludeCount++
	zn := e.evaluateEmbeddedZettel(zettel)
	return regionedBlocks(zn.Blocks, nil)
}

func (e *evaluator) evalTransclusion(tn *sx.Pair) *sx.Pair {
	attrs, ref, text := zsx.GetTransclusion(tn)
	refSym, refVal := zsx.GetReference(ref)

	// To prevent e.embedCount from counting
	if errText := e.checkMaxTransclusions(ref); errText != nil {
		return makeBlock(errText)
	}
	if !sz.SymRefStateZettel.IsEqualSymbol(refSym) {
		switch refSym {
		case zsx.SymRefStateInvalid, sz.SymRefStateBroken:
			e.transcludeCount++
			return makeBlock(createInlineErrorText(ref, "Invalid or broken transclusion reference"))
		case zsx.SymRefStateSelf:
			e.transcludeCount++
			return makeBlock(createInlineErrorText(ref, "Self transclusion reference"))
		case sz.SymRefStateFound, zsx.SymRefStateExternal:
			return tn
		case zsx.SymRefStateHosted, sz.SymRefStateBased:
			return makeBlock(e.evalEmbed(zsx.MakeEmbed(attrs, ref, "", text)))
		case sz.SymRefStateQuery:
			e.transcludeCount++
			return e.evalQueryTransclusion(refVal)
		default:
			return makeBlock(createInlineErrorText(ref, "Illegal reference symvol "+refSym.GetValue()))
		}
	}

	zid := mustParseZid(ref, refVal)

	cost, ok := e.costMap[zid]
	zn := cost.zn
	if zn == e.marker {
		e.transcludeCount++
		return makeBlock(createInlineErrorText(ref, "Recursive transclusion"))
	}
	if !ok {
		zettel, err1 := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
		if err1 != nil {
			if errors.Is(err1, &box.ErrNotAllowed{}) {
				return nil
			}
			e.transcludeCount++
			return makeBlock(createInlineErrorText(ref, "Unable to get zettel"))
		}
		setMetadataFromAttributes(zettel.Meta, attrs)
		ec := e.transcludeCount
		e.costMap[zid] = transcludeCost{zn: e.marker, ec: ec}
		zn = e.evaluateEmbeddedZettel(zettel)
		e.costMap[zid] = transcludeCost{zn: zn, ec: e.transcludeCount - ec}
		e.transcludeCount = 0 // No stack needed, because embedding is done left-recursive, depth-first.
	}
	e.transcludeCount++
	if ec := cost.ec; ec > 0 {
		e.transcludeCount += cost.ec
	}
	return regionedBlocks(zn.Blocks, attrs)
}

func (e *evaluator) evalQueryTransclusion(expr string) *sx.Pair {
	q := query.Parse(expr)
	ml, err := e.port.QueryMeta(e.ctx, q)
	if err != nil {
		if errors.Is(err, &box.ErrNotAllowed{}) {
			return nil
		}
		return makeBlock(createInlineErrorText(nil, "Unable to search zettel"))
	}
	result, _ := QueryAction(e.ctx, q, ml)
	if result != nil {
		result = mustPair(zsx.Walk(e, result, nil))
	}
	return result
}

func makeBlock(inl *sx.Pair) *sx.Pair { return zsx.MakePara(inl) }

func regionedBlocks(block *sx.Pair, attrs *sx.Pair) *sx.Pair {
	newAttrs := styleAttr(attrs, "width")
	blocks := zsx.GetBlock(block)
	if blocks.Tail() == nil {
		result := blocks.Head()
		return zsx.MakeRegion(zsx.SymRegionBlock, newAttrs, zsx.MakeBlock(result), nil)
	}
	return zsx.MakeRegion(zsx.SymRegionBlock, newAttrs, blocks, nil)
}

func styleAttr(attrs *sx.Pair, keys ...string) *sx.Pair {
	if attrs == nil {
		return attrs
	}
	a := zsx.GetAttributes(attrs)
	style := strings.TrimSpace(a["style"])
	var sb strings.Builder
	sb.WriteString(style)
	if style != "" && style[len(style)-1] != ';' {
		sb.WriteByte(';')
	}
	found := false
	for _, key := range keys {
		if val, ok := a[key]; ok {
			if found || style != "" {
				sb.WriteByte(' ')
			}
			sb.WriteString(key)
			sb.WriteString(": ")
			sb.WriteString(val)
			sb.WriteByte(';')
			delete(a, key)
			found = true
		}
	}

	if found {
		a["style"] = sb.String()
		return a.AsAssoc()
	}
	return attrs
}
