//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package webui provides wet-UI handlers for web requests.
package webui

import (
	"fmt"
	"net/http"
	"strings"

	"zettelstore.de/z/config"
	"zettelstore.de/z/domain"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/session"
)

// MakeGetRenameZettelHandler creates a new HTTP handler to display the HTML rename view of a zettel.
func MakeGetRenameZettelHandler(te *TemplateEngine, getMeta usecase.GetMeta) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		zid, err := domain.ParseZettelID(r.URL.Path[1:])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		ctx := r.Context()
		meta, err := getMeta.Run(ctx, zid)
		if err != nil {
			adapter.ReportUsecaseError(w, err)
			return
		}

		if format := adapter.GetFormat(r, r.URL.Query(), "html"); format != "html" {
			http.Error(w, fmt.Sprintf("Rename zettel %q not possible in format %q", zid.Format(), format), http.StatusBadRequest)
			return
		}

		user := session.GetUser(ctx)
		te.renderTemplate(ctx, w, domain.RenameTemplateID, struct {
			baseData
			Zid       string
			MetaPairs []domain.MetaPair
		}{
			baseData:  te.makeBaseData(ctx, config.GetLang(meta), "Rename Zettel "+zid.Format(), user),
			Zid:       zid.Format(),
			MetaPairs: meta.Pairs(),
		})
	}
}

// MakePostRenameZettelHandler creates a new HTTP handler to rename an existing zettel.
func MakePostRenameZettelHandler(renameZettel usecase.RenameZettel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		curZid, err := domain.ParseZettelID(r.URL.Path[1:])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Unable to read rename zettel form", http.StatusBadRequest)
			return
		}
		if formCurZid, err := domain.ParseZettelID(r.PostFormValue("curzid")); err != nil || formCurZid != curZid {
			http.Error(w, "Invalid value for current zettel id in form", http.StatusBadRequest)
			return
		}
		newZid, err := domain.ParseZettelID(strings.TrimSpace(r.PostFormValue("newzid")))
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid new zettel id %q", newZid.Format()), http.StatusBadRequest)
			return
		}

		if err := renameZettel.Run(r.Context(), curZid, newZid); err != nil {
			adapter.ReportUsecaseError(w, err)
			return
		}
		http.Redirect(w, r, adapter.NewURLBuilder('h').SetZid(newZid).String(), http.StatusFound)
	}
}