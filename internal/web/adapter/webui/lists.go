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
	"net/url"
	"slices"
	"strconv"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/auth/user"
	"zettelstore.de/z/internal/evaluator"
	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter"
)

// MakeListHTMLMetaHandler creates a HTTP handler for rendering the list of zettel as HTML.
func (wui *WebUI) MakeListHTMLMetaHandler(
	queryMeta *usecase.Query,
	tagZettel *usecase.TagZettel,
	roleZettel *usecase.RoleZettel,
	reIndex *usecase.ReIndex) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlQuery := r.URL.Query()
		if wui.handleTagZettel(w, r, tagZettel, urlQuery) ||
			wui.handleRoleZettel(w, r, roleZettel, urlQuery) {
			return
		}
		q := adapter.GetQuery(urlQuery)
		q = q.SetDeterministic()
		ctx := r.Context()
		metaSeq, err := queryMeta.Run(ctx, q)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		actions, err := adapter.TryReIndex(ctx, q.Actions(), metaSeq, reIndex)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		if len(metaSeq) > 0 {
			if slices.Contains(actions, api.RedirectAction) {
				ub := wui.NewURLBuilder('h').SetZid(metaSeq[0].Zid)
				wui.redirectFound(w, r, ub)
				return
			}
		}

		userLang := wui.getUserLang(ctx)

		var content, endnotes *sx.Pair
		numEntries := 0
		if bn, cnt := evaluator.QueryAction(ctx, q, metaSeq); bn != nil {
			enc := wui.getSimpleHTMLEncoder(userLang)
			content, endnotes, err = enc.BlocksSxn(&ast.BlockSlice{bn})
			if err != nil {
				wui.reportError(ctx, w, err)
				return
			}
			numEntries = cnt
		}

		siteName := wui.rtConfig.GetSiteName()
		user := user.GetCurrentUser(ctx)
		env, rb := wui.createRenderEnv(ctx, "list", userLang, siteName, user)
		if q == nil {
			rb.bindString("heading", sx.MakeString(siteName))
		} else {
			var sb strings.Builder
			q.PrintHuman(&sb)
			rb.bindString("heading", sx.MakeString(sb.String()))
		}
		rb.bindString("query-value", sx.MakeString(q.String()))
		if tzl := q.GetMetaValues(meta.KeyTags, false); len(tzl) > 0 {
			sxTzl, sxNoTzl := wui.transformTagZettelList(ctx, tagZettel, tzl)
			if !sx.IsNil(sxTzl) {
				rb.bindString("tag-zettel", sxTzl)
			}
			if !sx.IsNil(sxNoTzl) && wui.canCreate(ctx, user) {
				rb.bindString("create-tag-zettel", sxNoTzl)
			}
		}
		if rzl := q.GetMetaValues(meta.KeyRole, false); len(rzl) > 0 {
			sxRzl, sxNoRzl := wui.transformRoleZettelList(ctx, roleZettel, rzl)
			if !sx.IsNil(sxRzl) {
				rb.bindString("role-zettel", sxRzl)
			}
			if !sx.IsNil(sxNoRzl) && wui.canCreate(ctx, user) {
				rb.bindString("create-role-zettel", sxNoRzl)
			}
		}
		rb.bindString("content", content)
		rb.bindString("endnotes", endnotes)
		rb.bindString("num-entries", sx.Int64(numEntries))
		rb.bindString("num-meta", sx.Int64(len(metaSeq)))
		apiURL := wui.NewURLBuilder('z').AppendQuery(q.String())
		seed, found := q.GetSeed()
		if found {
			apiURL = apiURL.AppendKVQuery(api.QueryKeySeed, strconv.Itoa(seed))
		} else {
			seed = 0
		}
		if len(metaSeq) > 0 {
			rb.bindString("plain-url", sx.MakeString(apiURL.String()))
			rb.bindString("data-url", sx.MakeString(apiURL.AppendKVQuery(api.QueryKeyEncoding, api.EncodingData).String()))
			if wui.canCreate(ctx, user) {
				rb.bindString("create-url", sx.MakeString(wui.createNewURL))
				rb.bindString("seed", sx.Int64(seed))
			}
		}
		if rb.err == nil {
			err = wui.renderSxnTemplate(ctx, w, id.ZidListTemplate, env)
		} else {
			err = rb.err
		}
		if err != nil {
			wui.reportError(ctx, w, err)
		}
	})
}

func (wui *WebUI) transformTagZettelList(ctx context.Context, tagZettel *usecase.TagZettel, tags []meta.Value) (withZettel, withoutZettel *sx.Pair) {
	slices.Reverse(tags)
	for _, tag := range tags {
		tag = tag.NormalizeTag()
		if _, err := tagZettel.Run(ctx, tag); err == nil {
			u := wui.NewURLBuilder('h').AppendKVQuery(api.QueryKeyTag, string(tag))
			withZettel = wui.prependZettelLink(withZettel, string(tag), u)
		} else {
			u := wui.NewURLBuilder('c').SetZid(id.ZidTemplateNewTag).AppendKVQuery(
				queryKeyAction, valueActionNew).AppendKVQuery(meta.KeyTitle, string(tag))
			withoutZettel = wui.prependZettelLink(withoutZettel, string(tag), u)
		}
	}
	return withZettel, withoutZettel
}

func (wui *WebUI) transformRoleZettelList(ctx context.Context, roleZettel *usecase.RoleZettel, roles []meta.Value) (withZettel, withoutZettel *sx.Pair) {
	slices.Reverse(roles)
	for _, role := range roles {
		if _, err := roleZettel.Run(ctx, role); err == nil {
			u := wui.NewURLBuilder('h').AppendKVQuery(api.QueryKeyRole, string(role))
			withZettel = wui.prependZettelLink(withZettel, string(role), u)
		} else {
			u := wui.NewURLBuilder('c').SetZid(id.ZidTemplateNewRole).AppendKVQuery(
				queryKeyAction, valueActionNew).AppendKVQuery(meta.KeyTitle, string(role))
			withoutZettel = wui.prependZettelLink(withoutZettel, string(role), u)
		}
	}
	return withZettel, withoutZettel
}

func (wui *WebUI) prependZettelLink(sxZtl *sx.Pair, name string, u *api.URLBuilder) *sx.Pair {
	link := sx.MakeList(
		shtml.SymA,
		sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(shtml.SymAttrHref, sx.MakeString(u.String())),
		),
		sx.MakeString(name),
	)
	if sxZtl != nil {
		sxZtl = sxZtl.Cons(sx.MakeString(", "))
	}
	return sxZtl.Cons(link)
}

func (wui *WebUI) handleTagZettel(w http.ResponseWriter, r *http.Request, tagZettel *usecase.TagZettel, vals url.Values) bool {
	tag := vals.Get(api.QueryKeyTag)
	if tag == "" {
		return false
	}
	ctx := r.Context()
	z, err := tagZettel.Run(ctx, meta.Value(tag))
	if err != nil {
		wui.reportError(ctx, w, err)
		return true
	}
	wui.redirectFound(w, r, wui.NewURLBuilder('h').SetZid(z.Meta.Zid))
	return true
}

func (wui *WebUI) handleRoleZettel(w http.ResponseWriter, r *http.Request, roleZettel *usecase.RoleZettel, vals url.Values) bool {
	role := vals.Get(api.QueryKeyRole)
	if role == "" {
		return false
	}
	ctx := r.Context()
	z, err := roleZettel.Run(ctx, meta.Value(role))
	if err != nil {
		wui.reportError(ctx, w, err)
		return true
	}
	wui.redirectFound(w, r, wui.NewURLBuilder('h').SetZid(z.Meta.Zid))
	return true
}
