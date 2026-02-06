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
	"bytes"
	"context"
	"net/http"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/evaluator"
	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter"
	"zettelstore.de/z/internal/zettel"
)

// MakeGetCreateZettelHandler creates a new HTTP handler to display the
// HTML edit view for the various zettel creation methods.
func (wui *WebUI) MakeGetCreateZettelHandler(
	getZettel usecase.GetZettel,
	createZettel *usecase.CreateZettel,
	ucListRoles usecase.ListRoles,
	ucListSyntax usecase.ListSyntax,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()
		op := getCreateAction(q.Get(queryKeyAction))
		path := r.URL.Path[1:]
		zid, err := id.Parse(path)
		if err != nil {
			wui.reportError(ctx, w, box.ErrInvalidZid{Zid: path})
			return
		}
		origZettel, err := getZettel.Run(box.NoEnrichContext(ctx), zid)
		if err != nil {
			wui.reportError(ctx, w, box.ErrZettelNotFound{Zid: zid})
			return
		}

		roleData, syntaxData := retrieveDataLists(ctx, ucListRoles, ucListSyntax)
		switch op {
		case actionCopy:
			wui.renderZettelForm(ctx, w, createZettel.PrepareCopy(origZettel), "Copy Zettel", "", roleData, syntaxData)
		case actionFolge:
			wui.renderZettelForm(ctx, w, createZettel.PrepareFolge(origZettel), "Folge Zettel", "", roleData, syntaxData)
		case actionNew:
			title := sz.NormalizedSpacedText(origZettel.Meta.GetTitle())
			newTitle := sz.NormalizedSpacedText(q.Get(meta.KeyTitle))
			wui.renderZettelForm(ctx, w, createZettel.PrepareNew(origZettel, newTitle), title, "", roleData, syntaxData)
		case actionSequel:
			wui.renderZettelForm(ctx, w, createZettel.PrepareSequel(origZettel), "Sequel Zettel", "", roleData, syntaxData)
		}
	})
}

func retrieveDataLists(ctx context.Context, ucListRoles usecase.ListRoles, ucListSyntax usecase.ListSyntax) ([]string, []string) {
	roleData := dataListFromArrangement(ucListRoles.Run(ctx))
	syntaxData := dataListFromArrangement(ucListSyntax.Run(ctx))
	return roleData, syntaxData
}

func dataListFromArrangement(ar meta.Arrangement, err error) []string {
	if err == nil {
		l := ar.Counted()
		l.SortByCount()
		return l.Categories()
	}
	return nil
}

func (wui *WebUI) renderZettelForm(
	ctx context.Context,
	w http.ResponseWriter,
	ztl zettel.Zettel,
	title string,
	formActionURL string,
	roleData []string,
	syntaxData []string,
) {
	user := auth.GetCurrentUser(ctx)
	m := ztl.Meta

	var sb strings.Builder
	for key, val := range m.Rest() {
		sb.WriteString(key)
		sb.WriteString(": ")
		sb.WriteString(string(val))
		sb.WriteByte('\n')
	}
	env, rb := wui.createRenderEnvironment(ctx, "form", wui.getUserLang(ctx), title, user)
	rb.bindString("heading", sx.MakeString(title))
	rb.bindString("form-action-url", sx.MakeString(formActionURL))
	rb.bindString("role-data", makeStringList(roleData))
	rb.bindString("syntax-data", makeStringList(syntaxData))
	rb.bindString("meta", sx.MakeString(sb.String()))
	if !ztl.Content.IsBinary() {
		rb.bindString("content", sx.MakeString(ztl.Content.AsString()))
	}
	wui.bindCommonZettelData(ctx, &rb, user, m, "", &ztl.Content)
	if rb.err == nil {
		rb.err = wui.renderSxnTemplate(ctx, w, id.ZidFormTemplate, env)
	}
	if err := rb.err; err != nil {
		wui.reportError(ctx, w, err)
	}
}

// MakePostCreateZettelHandler creates a new HTTP handler to store content of
// an existing zettel.
func (wui *WebUI) MakePostCreateZettelHandler(createZettel *usecase.CreateZettel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		reEdit, zettel, err := parseZettelForm(r, id.Invalid)
		if err == errMissingContent {
			wui.reportError(ctx, w, adapter.NewErrBadRequest("Content is missing"))
			return
		}
		if err != nil {
			const msg = "Unable to read form data"
			wui.logger.Info(msg, "err", err)
			wui.reportError(ctx, w, adapter.NewErrBadRequest(msg))
			return
		}

		newZid, err := createZettel.Run(ctx, zettel)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		if reEdit {
			wui.redirectFound(w, r, wui.NewURLBuilder('e').SetZid(newZid))
		} else {
			wui.redirectFound(w, r, wui.NewURLBuilder('h').SetZid(newZid))
		}
	})
}

// MakeGetZettelFromListHandler creates a new HTTP handler to store content of
// an existing zettel.
func (wui *WebUI) MakeGetZettelFromListHandler(
	queryMeta *usecase.Query,
	evaluate *usecase.Evaluate,
	ucListRoles usecase.ListRoles,
	ucListSyntax usecase.ListSyntax,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := adapter.GetQuery(r.URL.Query())
		ctx := r.Context()
		metaSeq, err := queryMeta.Run(box.NoEnrichQuery(ctx, q), q)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		entries, _ := evaluator.QueryAction(ctx, q, metaSeq)
		blocks := evaluate.RunBlockNode(ctx, entries)
		enc := encoder.Create(webapi.EncoderZmk, nil)
		var zmkContent bytes.Buffer
		if err = enc.WriteSz(&zmkContent, blocks); err != nil {
			wui.reportError(ctx, w, err)
			return
		}

		m := meta.New(id.Invalid)
		m.Set(meta.KeyTitle, meta.Value(q.Human()))
		m.Set(meta.KeySyntax, meta.ValueSyntaxZmk)
		if qval := q.String(); qval != "" {
			m.Set(meta.KeyQuery, meta.Value(qval))
		}
		zettel := zettel.Zettel{Meta: m, Content: zettel.NewContent(zmkContent.Bytes())}
		roleData, syntaxData := retrieveDataLists(ctx, ucListRoles, ucListSyntax)
		wui.renderZettelForm(ctx, w, zettel, "Zettel from list", wui.createNewURL, roleData, syntaxData)
	})
}
