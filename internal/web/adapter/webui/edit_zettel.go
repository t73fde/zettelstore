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
	"net/http"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter"
)

// MakeEditGetZettelHandler creates a new HTTP handler to display the
// HTML edit view of a zettel.
func (wui *WebUI) MakeEditGetZettelHandler(
	getZettel usecase.GetZettel,
	ucListRoles usecase.ListRoles,
	ucListSyntax usecase.ListSyntax,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		path := r.URL.Path[1:]
		zid, err := id.Parse(path)
		if err != nil {
			wui.reportError(ctx, w, box.ErrInvalidZid{Zid: path})
			return
		}

		zettel, err := getZettel.Run(box.NoEnrichContext(ctx), zid)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}

		roleData, syntaxData := retrieveDataLists(ctx, ucListRoles, ucListSyntax)
		wui.renderZettelForm(ctx, w, zettel, "Edit Zettel", "", roleData, syntaxData)
	})
}

// MakeEditSetZettelHandler creates a new HTTP handler to store content of
// an existing zettel.
func (wui *WebUI) MakeEditSetZettelHandler(updateZettel *usecase.UpdateZettel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		path := r.URL.Path[1:]
		zid, err := id.Parse(path)
		if err != nil {
			wui.reportError(ctx, w, box.ErrInvalidZid{Zid: path})
			return
		}

		reEdit, zettel, err := parseZettelForm(r, zid)
		hasContent := true
		if err != nil {
			if err != errMissingContent {
				const msg = "Unable to read zettel form"
				wui.log.Info().Err(err).Msg(msg)
				wui.reportError(ctx, w, adapter.NewErrBadRequest(msg))
				return
			}
			hasContent = false
		}
		if err = updateZettel.Run(r.Context(), zettel, hasContent); err != nil {
			wui.reportError(ctx, w, err)
			return
		}

		if reEdit {
			wui.redirectFound(w, r, wui.NewURLBuilder('e').SetZid(zid))
		} else {
			wui.redirectFound(w, r, wui.NewURLBuilder('h').SetZid(zid))
		}
	})
}
