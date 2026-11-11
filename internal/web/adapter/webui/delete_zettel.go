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

	"t73f.de/r/sx"
	"t73f.de/r/zero/roster"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/usecase"
)

// MakeGetDeleteZettelHandler creates a new HTTP handler to display the
// HTML delete view of a zettel.
func (wui *WebUI) MakeGetDeleteZettelHandler(
	getZettel usecase.GetZettel,
	getAllZettel usecase.GetAllZettel,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		path := r.URL.Path[1:]
		zid, err := id.Parse(path)
		if err != nil {
			wui.reportError(ctx, w, box.ErrInvalidZid{Zid: path})
			return
		}

		zs, err := getAllZettel.Run(ctx, zid)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		m := zs[0].Meta
		currZid := zid.String()

		user := auth.GetCurrentUser(ctx)
		env, rb := wui.createRenderEnvironment(
			ctx, "delete", wui.getUserLang(ctx), "Delete Zettel "+currZid, user)
		if len(zs) > 1 {
			rb.bindString("shadowed-box",
				sx.MakeString(string(zs[1].Meta.GetDefault(meta.KeyBoxName, "???"))))
			rb.bindString("incoming", nil)
		} else {
			rb.bindString("shadowed-box", nil)
			rb.bindString("incoming",
				wui.encodeIncoming(m, currZid, wui.makeGetTextTitle(ctx, getZettel)))
		}
		wui.bindCommonZettelData(ctx, &rb, user, m, "", nil)

		if rb.err == nil {
			err = wui.renderSxnTemplate(ctx, w, id.ZidDeleteTemplate, env)
		} else {
			err = rb.err
		}
		if err != nil {
			wui.reportError(ctx, w, err)
		}
	})
}

func (wui *WebUI) encodeIncoming(
	m *meta.Meta,
	currZid string,
	getTextTitle getTextTitleFunc,
) *sx.Pair {
	var zidMap roster.Roster[string]
	addListValues(&zidMap, m, meta.KeyBackward, currZid)
	for kd := range meta.KeyDescriptionSeq() {
		inverseKey := kd.Inverse
		if inverseKey == "" {
			continue
		}
		switch meta.GetDescription(inverseKey).Type {
		case meta.TypeID:
			if val, ok := m.Get(inverseKey); ok && string(val) != currZid {
				zidMap.Insert(string(val))
			}
		case meta.TypeIDSet:
			addListValues(&zidMap, m, inverseKey, currZid)
		}
	}
	// Roster.Values() is already sorted
	return wui.zidLinksSxn(zidMap.Values(), getTextTitle)
}

func addListValues(zidMap *roster.Roster[string], m *meta.Meta, key string, currZid string) {
	for val := range m.GetFields(key) {
		if val != currZid {
			zidMap.Insert(val)
		}
	}
}

// MakePostDeleteZettelHandler creates a new HTTP handler to delete a zettel.
func (wui *WebUI) MakePostDeleteZettelHandler(deleteZettel *usecase.DeleteZettel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		path := r.URL.Path[1:]
		zid, err := id.Parse(path)
		if err != nil {
			wui.reportError(ctx, w, box.ErrInvalidZid{Zid: path})
			return
		}

		if err = deleteZettel.Run(r.Context(), zid); err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		wui.redirectFound(w, r, wui.NewURLBuilder('/'))
	})
}
