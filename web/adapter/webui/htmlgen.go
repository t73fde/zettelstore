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

package webui

import (
	"maps"
	"net/url"
	"slices"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/attrs"
	"t73f.de/r/zsc/shtml"
	"t73f.de/r/zsc/sz"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/encoder/szenc"
	"zettelstore.de/z/strfun"
	"zettelstore.de/z/zettel/meta"
)

// Builder allows to build new URLs for the web service.
type urlBuilder interface {
	GetURLPrefix() string
	NewURLBuilder(key byte) *api.URLBuilder
}

type htmlGenerator struct {
	tx    *szenc.Transformer
	th    *shtml.Evaluator
	lang  string
	symAt *sx.Symbol
}

func (wui *WebUI) createGenerator(builder urlBuilder, lang string) *htmlGenerator {
	th := shtml.NewEvaluator(1)

	findA := func(obj sx.Object) (attr, assoc, rest *sx.Pair) {
		pair, isPair := sx.GetPair(obj)
		if !isPair || !shtml.SymA.IsEqual(pair.Car()) {
			return nil, nil, nil
		}
		rest = pair.Tail()
		if rest == nil {
			return nil, nil, nil
		}
		objA := rest.Car()
		attr, isPair = sx.GetPair(objA)
		if !isPair || !sxhtml.SymAttr.IsEqual(attr.Car()) {
			return nil, nil, nil
		}
		return attr, attr.Tail(), rest.Tail()
	}
	linkZettel := func(obj sx.Object) sx.Object {
		attr, assoc, rest := findA(obj)
		if attr == nil {
			return obj
		}

		hrefP := assoc.Assoc(shtml.SymAttrHref)
		if hrefP == nil {
			return obj
		}
		href, ok := sx.GetString(hrefP.Cdr())
		if !ok {
			return obj
		}
		zid, fragment, hasFragment := strings.Cut(href.GetValue(), "#")
		u := builder.NewURLBuilder('h').SetZid(api.ZettelID(zid))
		if hasFragment {
			u = u.SetFragment(fragment)
		}
		assoc = assoc.Cons(sx.Cons(shtml.SymAttrHref, sx.MakeString(u.String())))
		return rest.Cons(assoc.Cons(sxhtml.SymAttr)).Cons(shtml.SymA)
	}

	rebind(th, sz.SymLinkZettel, linkZettel)
	rebind(th, sz.SymLinkFound, linkZettel)
	rebind(th, sz.SymLinkBased, func(obj sx.Object) sx.Object {
		attr, assoc, rest := findA(obj)
		if attr == nil {
			return obj
		}
		hrefP := assoc.Assoc(shtml.SymAttrHref)
		if hrefP == nil {
			return obj
		}
		href, ok := sx.GetString(hrefP.Cdr())
		if !ok {
			return obj
		}
		u := builder.NewURLBuilder('/')
		assoc = assoc.Cons(sx.Cons(shtml.SymAttrHref, sx.MakeString(u.String()+href.GetValue()[1:])))
		return rest.Cons(assoc.Cons(sxhtml.SymAttr)).Cons(shtml.SymA)
	})
	rebind(th, sz.SymLinkQuery, func(obj sx.Object) sx.Object {
		attr, assoc, rest := findA(obj)
		if attr == nil {
			return obj
		}
		hrefP := assoc.Assoc(shtml.SymAttrHref)
		if hrefP == nil {
			return obj
		}
		href, ok := sx.GetString(hrefP.Cdr())
		if !ok {
			return obj
		}
		ur, err := url.Parse(href.GetValue())
		if err != nil {
			return obj
		}
		urlQuery := ur.Query()
		if !urlQuery.Has(api.QueryKeyQuery) {
			return obj
		}
		u := builder.NewURLBuilder('h')
		if q := urlQuery.Get(api.QueryKeyQuery); q != "" {
			u = u.AppendQuery(q)
		}
		assoc = assoc.Cons(sx.Cons(shtml.SymAttrHref, sx.MakeString(u.String())))
		return rest.Cons(assoc.Cons(sxhtml.SymAttr)).Cons(shtml.SymA)
	})
	rebind(th, sz.SymLinkExternal, func(obj sx.Object) sx.Object {
		attr, _, rest := findA(obj)
		if attr == nil {
			return obj
		}
		a := sz.GetAttributes(attr)
		a = a.Set("target", "_blank")
		a = a.Add("rel", "external").Add("rel", "noreferrer")
		return rest.Cons(shtml.EvaluateAttrbute(a)).Cons(shtml.SymA)
	})
	rebind(th, sz.SymEmbed, func(obj sx.Object) sx.Object {
		pair, isPair := sx.GetPair(obj)
		if !isPair || !shtml.SymIMG.IsEqual(pair.Car()) {
			return obj
		}
		attr, isPair := sx.GetPair(pair.Tail().Car())
		if !isPair || !sxhtml.SymAttr.IsEqual(attr.Car()) {
			return obj
		}
		srcP := attr.Tail().Assoc(shtml.SymAttrSrc)
		if srcP == nil {
			return obj
		}
		src, isString := sx.GetString(srcP.Cdr())
		if !isString {
			return obj
		}
		zid := api.ZettelID(src.GetValue())
		if !zid.IsValid() {
			return obj
		}
		u := builder.NewURLBuilder('z').SetZid(zid)
		imgAttr := attr.Tail().Cons(sx.Cons(shtml.SymAttrSrc, sx.MakeString(u.String()))).Cons(sxhtml.SymAttr)
		return pair.Tail().Tail().Cons(imgAttr).Cons(shtml.SymIMG)
	})

	return &htmlGenerator{
		tx:   szenc.NewTransformer(),
		th:   th,
		lang: lang,
	}
}

func rebind(ev *shtml.Evaluator, sym *sx.Symbol, fn func(sx.Object) sx.Object) {
	prevFn := ev.ResolveBinding(sym)
	ev.Rebind(sym, func(args sx.Vector, env *shtml.Environment) sx.Object {
		obj := prevFn(args, env)
		if env.GetError() == nil {
			return fn(obj)
		}
		return sx.Nil()
	})
}

// SetUnique sets a prefix to make several HTML ids unique.
func (g *htmlGenerator) SetUnique(s string) *htmlGenerator { g.th.SetUnique(s); return g }

var mapMetaKey = map[string]string{
	api.KeyCopyright: "copyright",
	api.KeyLicense:   "license",
}

func (g *htmlGenerator) MetaSxn(m *meta.Meta) *sx.Pair {
	tm := g.tx.GetMeta(m)
	env := shtml.MakeEnvironment(g.lang)
	hm, err := g.th.Evaluate(tm, &env)
	if err != nil {
		return nil
	}

	ignore := strfun.NewSet(api.KeyTitle, api.KeyLang)
	metaMap := make(map[string]*sx.Pair, m.Length())
	if tags, ok := m.Get(api.KeyTags); ok {
		metaMap[api.KeyTags] = g.transformMetaTags(tags)
		ignore.Set(api.KeyTags)
	}

	for elem := hm; elem != nil; elem = elem.Tail() {
		mlst, isPair := sx.GetPair(elem.Car())
		if !isPair {
			continue
		}
		att, isPair := sx.GetPair(mlst.Tail().Car())
		if !isPair {
			continue
		}
		if !att.Car().IsEqual(g.symAt) {
			continue
		}
		a := make(attrs.Attributes, 32)
		for aelem := att.Tail(); aelem != nil; aelem = aelem.Tail() {
			if p, ok := sx.GetPair(aelem.Car()); ok {
				key := p.Car()
				val := p.Cdr()
				if tail, isTail := sx.GetPair(val); isTail {
					val = tail.Car()
				}
				a = a.Set(sz.GoValue(key), sz.GoValue(val))
			}
		}
		name, found := a.Get("name")
		if !found || ignore.Has(name) {
			continue
		}

		newName, found := mapMetaKey[name]
		if !found {
			continue
		}
		a = a.Set("name", newName)
		metaMap[newName] = g.th.EvaluateMeta(a)
	}
	var lb sx.ListBuilder
	for _, key := range slices.Sorted(maps.Keys(metaMap)) {
		lb.Add(metaMap[key])
	}
	return lb.List()
}

func (g *htmlGenerator) transformMetaTags(tags string) *sx.Pair {
	var sb strings.Builder
	for i, val := range meta.ListFromValue(tags) {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strings.TrimPrefix(val, "#"))
	}
	metaTags := sb.String()
	if len(metaTags) == 0 {
		return nil
	}
	return g.th.EvaluateMeta(attrs.Attributes{"name": "keywords", "content": metaTags})
}

func (g *htmlGenerator) BlocksSxn(bs *ast.BlockSlice) (content, endnotes *sx.Pair, _ error) {
	if bs == nil || len(*bs) == 0 {
		return nil, nil, nil
	}
	sx := g.tx.GetSz(bs)
	env := shtml.MakeEnvironment(g.lang)
	sh, err := g.th.Evaluate(sx, &env)
	if err != nil {
		return nil, nil, err
	}
	return sh, shtml.Endnotes(&env), nil
}

// InlinesSxHTML returns an inline slice, encoded as a SxHTML object.
func (g *htmlGenerator) InlinesSxHTML(is *ast.InlineSlice) *sx.Pair {
	if is == nil || len(*is) == 0 {
		return nil
	}
	return g.nodeSxHTML(is)
}

func (g *htmlGenerator) nodeSxHTML(node ast.Node) *sx.Pair {
	sz := g.tx.GetSz(node)
	env := shtml.MakeEnvironment(g.lang)
	sh, err := g.th.Evaluate(sz, &env)
	if err != nil {
		return nil
	}
	return sh
}
