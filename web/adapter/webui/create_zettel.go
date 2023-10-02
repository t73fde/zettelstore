//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package webui

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"zettelstore.de/client.fossil/api"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/z/box"
	"zettelstore.de/z/encoder/zmkenc"
	"zettelstore.de/z/evaluator"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// MakeGetCreateZettelHandler creates a new HTTP handler to display the
// HTML edit view for the various zettel creation methods.
func (wui *WebUI) MakeGetCreateZettelHandler(
	getZettel usecase.GetZettel, createZettel *usecase.CreateZettel,
	ucListRoles usecase.ListRoles, ucListSyntax usecase.ListSyntax) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		case actionChild:
			wui.renderZettelForm(ctx, w, createZettel.PrepareChild(origZettel), "Child Zettel", "", roleData, syntaxData)
		case actionCopy:
			wui.renderZettelForm(ctx, w, createZettel.PrepareCopy(origZettel), "Copy Zettel", "", roleData, syntaxData)
		case actionFolge:
			wui.renderZettelForm(ctx, w, createZettel.PrepareFolge(origZettel), "Folge Zettel", "", roleData, syntaxData)
		case actionNew:
			title := parser.NormalizedSpacedText(origZettel.Meta.GetTitle())
			newTitle := parser.NormalizedSpacedText(q.Get(api.KeyTitle))
			wui.renderZettelForm(ctx, w, createZettel.PrepareNew(origZettel, newTitle), title, "", roleData, syntaxData)
		case actionVersion:
			wui.renderZettelForm(ctx, w, createZettel.PrepareVersion(origZettel), "Version Zettel", "", roleData, syntaxData)
		}
	}
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
	user := server.GetUser(ctx)
	m := ztl.Meta

	var sb strings.Builder
	for _, p := range m.PairsRest() {
		sb.WriteString(p.Key)
		sb.WriteString(": ")
		sb.WriteString(p.Value)
		sb.WriteByte('\n')
	}
	env, rb := wui.createRenderEnv(ctx, "form", wui.rtConfig.Get(ctx, nil, api.KeyLang), title, user)
	rb.bindString("heading", sx.String(title))
	rb.bindString("form-action-url", sx.String(formActionURL))
	rb.bindString("role-data", makeStringList(roleData))
	rb.bindString("syntax-data", makeStringList(syntaxData))
	rb.bindString("meta", sx.String(sb.String()))
	if !ztl.Content.IsBinary() {
		rb.bindString("content", sx.String(ztl.Content.AsString()))
	}
	wui.bindCommonZettelData(ctx, &rb, user, m, &ztl.Content)
	if rb.err == nil {
		rb.err = wui.renderSxnTemplate(ctx, w, id.FormTemplateZid, env)
	}
	if err := rb.err; err != nil {
		wui.reportError(ctx, w, err)
	}
}

// MakePostCreateZettelHandler creates a new HTTP handler to store content of
// an existing zettel.
func (wui *WebUI) MakePostCreateZettelHandler(createZettel *usecase.CreateZettel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		reEdit, zettel, err := parseZettelForm(r, id.Invalid)
		if err == errMissingContent {
			wui.reportError(ctx, w, adapter.NewErrBadRequest("Content is missing"))
			return
		}
		if err != nil {
			const msg = "Unable to read form data"
			wui.log.Info().Err(err).Msg(msg)
			wui.reportError(ctx, w, adapter.NewErrBadRequest(msg))
			return
		}

		newZid, err := createZettel.Run(ctx, zettel)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		if reEdit {
			wui.redirectFound(w, r, wui.NewURLBuilder('e').SetZid(api.ZettelID(newZid.String())))
		} else {
			wui.redirectFound(w, r, wui.NewURLBuilder('h').SetZid(api.ZettelID(newZid.String())))
		}
	}
}

// MakeGetZettelFromListHandler creates a new HTTP handler to store content of
// an existing zettel.
func (wui *WebUI) MakeGetZettelFromListHandler(
	queryMeta *usecase.Query, evaluate *usecase.Evaluate,
	ucListRoles usecase.ListRoles, ucListSyntax usecase.ListSyntax) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		q := adapter.GetQuery(r.URL.Query())
		ctx := r.Context()
		metaSeq, err := queryMeta.Run(box.NoEnrichQuery(ctx, q), q)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		bns := evaluate.RunBlockNode(ctx, evaluator.QueryAction(ctx, q, metaSeq, wui.rtConfig))
		enc := zmkenc.Create()
		var zmkContent bytes.Buffer
		_, err = enc.WriteBlocks(&zmkContent, &bns)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}

		m := meta.New(id.Invalid)
		m.Set(api.KeyTitle, q.Human())
		m.Set(api.KeySyntax, api.ValueSyntaxZmk)
		if qval := q.String(); qval != "" {
			m.Set(api.KeyQuery, qval)
		}
		zettel := zettel.Zettel{Meta: m, Content: zettel.NewContent(zmkContent.Bytes())}
		roleData, syntaxData := retrieveDataLists(ctx, ucListRoles, ucListSyntax)
		wui.renderZettelForm(ctx, w, zettel, "Zettel from list", wui.createNewURL, roleData, syntaxData)
	}
}
