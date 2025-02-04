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
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/maps"
	"t73f.de/r/zsc/strfun"
	"zettelstore.de/z/box"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/server"
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

		user := server.GetUser(ctx)
		env, rb := wui.createRenderEnv(
			ctx, "delete", wui.getUserLang(ctx), "Delete Zettel "+m.Zid.String(), user)
		if len(zs) > 1 {
			rb.bindString("shadowed-box", sx.MakeString(zs[1].Meta.GetDefault(api.KeyBoxNumber, "???")))
			rb.bindString("incoming", nil)
		} else {
			rb.bindString("shadowed-box", nil)
			rb.bindString("incoming", wui.encodeIncoming(m, wui.makeGetTextTitle(ctx, getZettel)))
		}
		wui.bindCommonZettelData(ctx, &rb, user, m, nil)

		if rb.err == nil {
			err = wui.renderSxnTemplate(ctx, w, api.ZidDeleteTemplate, env)
		} else {
			err = rb.err
		}
		if err != nil {
			wui.reportError(ctx, w, err)
		}
	})
}

func (wui *WebUI) encodeIncoming(m *meta.Meta, getTextTitle getTextTitleFunc) *sx.Pair {
	zidMap := make(strfun.Set)
	addListValues(zidMap, m, api.KeyBackward)
	for _, kd := range meta.GetSortedKeyDescriptions() {
		inverseKey := kd.Inverse
		if inverseKey == "" {
			continue
		}
		ikd := meta.GetDescription(inverseKey)
		switch ikd.Type {
		case meta.TypeID:
			if val, ok := m.Get(inverseKey); ok {
				zidMap.Set(val)
			}
		case meta.TypeIDSet:
			addListValues(zidMap, m, inverseKey)
		}
	}
	return wui.zidLinksSxn(maps.Keys(zidMap), getTextTitle)
}

func addListValues(zidMap strfun.Set, m *meta.Meta, key string) {
	if values, ok := m.GetList(key); ok {
		for _, val := range values {
			zidMap.Set(val)
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
