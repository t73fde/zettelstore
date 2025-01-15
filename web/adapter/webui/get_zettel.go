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
	"net/http"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/shtml"
	"zettelstore.de/z/box"
	"zettelstore.de/z/config"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// MakeGetHTMLZettelHandler creates a new HTTP handler for the use case "get zettel".
func (wui *WebUI) MakeGetHTMLZettelHandler(
	evaluate *usecase.Evaluate,
	getZettel usecase.GetZettel,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		path := r.URL.Path[1:]
		zid, err := id.Parse(path)
		if err != nil {
			wui.reportError(ctx, w, box.ErrInvalidZid{Zid: path})
			return
		}

		q := r.URL.Query()
		zn, err := evaluate.Run(ctx, zid, q.Get(api.KeySyntax))
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}

		zettelLang := wui.getConfig(ctx, zn.InhMeta, api.KeyLang)
		enc := wui.getSimpleHTMLEncoder(zettelLang)
		metaObj := enc.MetaSxn(zn.InhMeta)
		content, endnotes, err := enc.BlocksSxn(&zn.Ast)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}

		user := server.GetUser(ctx)
		getTextTitle := wui.makeGetTextTitle(ctx, getZettel)

		title := parser.NormalizedSpacedText(zn.InhMeta.GetTitle())
		env, rb := wui.createRenderEnv(ctx, "zettel", zettelLang, title, user)
		rb.bindSymbol(symMetaHeader, metaObj)
		rb.bindString("heading", sx.MakeString(title))
		if role, found := zn.InhMeta.Get(api.KeyRole); found && role != "" {
			rb.bindString("role-url", sx.MakeString(wui.NewURLBuilder('h').AppendQuery(api.KeyRole+api.SearchOperatorHas+role).String()))
		}
		if folgeRole, found := zn.InhMeta.Get(api.KeyFolgeRole); found && folgeRole != "" {
			rb.bindString("folge-role-url", sx.MakeString(wui.NewURLBuilder('h').AppendQuery(api.KeyRole+api.SearchOperatorHas+folgeRole).String()))
		}
		rb.bindString("tag-refs", wui.transformTagSet(api.KeyTags, meta.ListFromValue(zn.InhMeta.GetDefault(api.KeyTags, ""))))
		rb.bindString("predecessor-refs", wui.identifierSetAsLinks(zn.InhMeta, api.KeyPredecessor, getTextTitle))
		rb.bindString("precursor-refs", wui.identifierSetAsLinks(zn.InhMeta, api.KeyPrecursor, getTextTitle))
		rb.bindString("prequel-refs", wui.identifierSetAsLinks(zn.InhMeta, api.KeyPrequel, getTextTitle))
		rb.bindString("superior-refs", wui.identifierSetAsLinks(zn.InhMeta, api.KeySuperior, getTextTitle))
		rb.bindString("urls", metaURLAssoc(zn.InhMeta))
		rb.bindString("content", content)
		rb.bindString("endnotes", endnotes)
		wui.bindLinks(ctx, &rb, "folge", zn.InhMeta, api.KeyFolge, config.KeyShowFolgeLinks, getTextTitle)
		wui.bindLinks(ctx, &rb, "sequel", zn.InhMeta, api.KeySequel, config.KeyShowSequelLinks, getTextTitle)
		wui.bindLinks(ctx, &rb, "subordinate", zn.InhMeta, api.KeySubordinates, config.KeyShowSubordinateLinks, getTextTitle)
		wui.bindLinks(ctx, &rb, "back", zn.InhMeta, api.KeyBack, config.KeyShowBackLinks, getTextTitle)
		wui.bindLinks(ctx, &rb, "successor", zn.InhMeta, api.KeySuccessors, config.KeyShowSuccessorLinks, getTextTitle)
		if role, found := zn.InhMeta.Get(api.KeyRole); found && role != "" {
			for _, part := range []string{"meta", "actions", "heading"} {
				rb.rebindResolved("ROLE-"+role+"-"+part, "ROLE-DEFAULT-"+part)
			}
		}
		wui.bindCommonZettelData(ctx, &rb, user, zn.InhMeta, &zn.Content)
		if rb.err == nil {
			err = wui.renderSxnTemplate(ctx, w, id.ZettelTemplateZid, env)
		} else {
			err = rb.err
		}
		if err != nil {
			wui.reportError(ctx, w, err)
		}
	})
}

func (wui *WebUI) identifierSetAsLinks(m *meta.Meta, key string, getTextTitle getTextTitleFunc) *sx.Pair {
	if values, ok := m.GetList(key); ok {
		return wui.transformIdentifierSet(values, getTextTitle)
	}
	return nil
}

func metaURLAssoc(m *meta.Meta) *sx.Pair {
	var result sx.ListBuilder
	for _, p := range m.PairsRest() {
		if key := p.Key; strings.HasSuffix(key, meta.SuffixKeyURL) {
			if val := p.Value; val != "" {
				result.Add(sx.Cons(sx.MakeString(capitalizeMetaKey(key)), sx.MakeString(val)))
			}
		}
	}
	return result.List()
}

func (wui *WebUI) bindLinks(ctx context.Context, rb *renderBinder, varPrefix string, m *meta.Meta, key, configKey string, getTextTitle getTextTitleFunc) {
	varLinks := varPrefix + "-links"
	var symOpen *sx.Symbol
	switch wui.getConfig(ctx, m, configKey) {
	case "false":
		rb.bindString(varLinks, sx.Nil())
		return
	case "close":
	default:
		symOpen = shtml.SymAttrOpen
	}
	lstLinks := wui.zettelLinksSxn(m, key, getTextTitle)
	rb.bindString(varLinks, lstLinks)
	if sx.IsNil(lstLinks) {
		return
	}
	rb.bindString(varPrefix+"-open", symOpen)
}

func (wui *WebUI) zettelLinksSxn(m *meta.Meta, key string, getTextTitle getTextTitleFunc) *sx.Pair {
	values, ok := m.GetList(key)
	if !ok || len(values) == 0 {
		return nil
	}
	return wui.zidLinksSxn(values, getTextTitle)
}

func (wui *WebUI) zidLinksSxn(values []string, getTextTitle getTextTitleFunc) *sx.Pair {
	var lb sx.ListBuilder
	for _, val := range values {
		zid, err := id.Parse(val)
		if err != nil {
			continue
		}
		if title, found := getTextTitle(zid); found > 0 {
			url := sx.MakeString(wui.NewURLBuilder('h').SetZid(zid.ZettelID()).String())
			if title == "" {
				lb.Add(sx.Cons(sx.MakeString(val), url))
			} else {
				lb.Add(sx.Cons(sx.MakeString(title), url))
			}
		}
	}
	return lb.List()
}
