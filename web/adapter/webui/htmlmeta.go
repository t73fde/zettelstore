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

package webui

import (
	"context"
	"errors"
	"iter"

	"t73f.de/r/sx"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"
	"zettelstore.de/z/box"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/usecase"
)

func (wui *WebUI) writeHTMLMetaValue(
	key string, value meta.Value,
	getTextTitle getTextTitleFunc,
) sx.Object {
	switch kt := meta.Type(key); kt {
	case meta.TypeCredential:
		return sx.MakeString(string(value))
	case meta.TypeEmpty:
		return sx.MakeString(string(value))
	case meta.TypeID:
		return wui.transformIdentifier(value, getTextTitle)
	case meta.TypeIDSet:
		return wui.transformIdentifierSet(value.Fields(), getTextTitle)
	case meta.TypeNumber:
		return wui.transformKeyValueText(key, value, string(value))
	case meta.TypeString:
		return sx.MakeString(string(value))
	case meta.TypeTagSet:
		return wui.transformTagSet(key, value.AsSlice())
	case meta.TypeTimestamp:
		if ts, ok := value.AsTime(); ok {
			return sx.MakeList(
				sx.MakeSymbol("time"),
				sx.MakeList(
					sxhtml.SymAttr,
					sx.Cons(sx.MakeSymbol("datetime"), sx.MakeString(ts.Format("2006-01-02T15:04:05"))),
				),
				sx.MakeList(sxhtml.SymNoEscape, sx.MakeString(ts.Format("2006-01-02&nbsp;15:04:05"))),
			)
		}
		return sx.Nil()
	case meta.TypeURL:
		return wui.url2html(sx.MakeString(string(value)))
	case meta.TypeWord:
		return wui.transformKeyValueText(key, value, string(value))
	default:
		return sx.MakeList(shtml.SymSTRONG, sx.MakeString("Unhandled type: "), sx.MakeString(kt.Name))
	}
}

func (wui *WebUI) transformIdentifier(val meta.Value, getTextTitle getTextTitleFunc) sx.Object {
	text := sx.MakeString(string(val))
	zid, err := id.Parse(string(val))
	if err != nil {
		return text
	}
	title, found := getTextTitle(zid)
	switch {
	case found > 0:
		ub := wui.NewURLBuilder('h').SetZid(zid)
		attrs := sx.Nil()
		if title != "" {
			attrs = attrs.Cons(sx.Cons(shtml.SymAttrTitle, sx.MakeString(title)))
		}
		attrs = attrs.Cons(sx.Cons(shtml.SymAttrHref, sx.MakeString(ub.String()))).Cons(sxhtml.SymAttr)
		return sx.Nil().Cons(sx.MakeString(zid.String())).Cons(attrs).Cons(shtml.SymA)
	case found == 0:
		return sx.MakeList(sx.MakeSymbol("s"), text)
	default: // case found < 0:
		return text
	}
}

var space = sx.MakeString(" ")

func (wui *WebUI) transformIdentifierSet(vals iter.Seq[string], getTextTitle getTextTitleFunc) *sx.Pair {
	var lb sx.ListBuilder
	lb.Add(shtml.SymSPAN)
	hadValue := false
	for val := range vals {
		if hadValue {
			lb.Add(space)
		}
		hadValue = true
		lb.Add(wui.transformIdentifier(meta.Value(val), getTextTitle))
	}
	if hadValue {
		return lb.List()
	}
	return nil
}

func (wui *WebUI) transformTagSet(key string, tags []string) *sx.Pair {
	var lb sx.ListBuilder
	lb.Add(shtml.SymSPAN)
	for i, tag := range tags {
		if i > 0 {
			lb.Add(space)
		}
		lb.Add(wui.transformKeyValueText(key, meta.Value(tag), tag))
	}
	if len(tags) > 1 {
		lb.Add(space)
		lb.Add(wui.transformKeyValuesText(key, tags, "(all)"))
	}
	return lb.List()
}

func (wui *WebUI) transformKeyValueText(key string, value meta.Value, text string) *sx.Pair {
	ub := wui.NewURLBuilder('h').AppendQuery(key + api.SearchOperatorHas + string(value))
	return buildHref(ub, text)
}

func (wui *WebUI) transformKeyValuesText(key string, values []string, text string) *sx.Pair {
	ub := wui.NewURLBuilder('h')
	for _, val := range values {
		ub = ub.AppendQuery(key + api.SearchOperatorHas + val)
	}
	return buildHref(ub, text)
}

func buildHref(ub *api.URLBuilder, text string) *sx.Pair {
	return sx.MakeList(
		shtml.SymA,
		sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(shtml.SymAttrHref, sx.MakeString(ub.String())),
		),
		sx.MakeString(text),
	)
}

type getTextTitleFunc func(id.Zid) (string, int)

func (wui *WebUI) makeGetTextTitle(ctx context.Context, getZettel usecase.GetZettel) getTextTitleFunc {
	return func(zid id.Zid) (string, int) {
		z, err := getZettel.Run(box.NoEnrichContext(ctx), zid)
		if err != nil {
			if errors.Is(err, &box.ErrNotAllowed{}) {
				return "", -1
			}
			return "", 0
		}
		return parser.NormalizedSpacedText(z.Meta.GetTitle()), 1
	}
}
