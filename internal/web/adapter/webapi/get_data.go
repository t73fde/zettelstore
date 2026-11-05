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

package webapi

import (
	"net/http"

	"t73f.de/r/sx"
	"t73f.de/r/zero/semver"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/usecase"
)

// MakeGetDataHandler creates a new HTTP handler to return zettelstore data.
func (a *WebAPI) MakeGetDataHandler(
	ucVersion usecase.Version,
	ucIsReady usecase.IsReady,
	ucSiteInfo usecase.SiteInfo,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch q.Get("data") {
		case "ready":
			a.writeIsReady(w, ucIsReady.Run())
		case "", "version":
			a.writeVersion(w, ucVersion.Run())
		case "site-info":
			siteInfo, err := ucSiteInfo.Run(r.Context())
			if err != nil {
				a.reportUsecaseError(w, err)
				return
			}
			a.writeSiteInfo(w, siteInfo)
		default:
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		}
	})
}

func (a *WebAPI) writeVersion(w http.ResponseWriter, version semver.SemVer) {
	err := a.writeObject(w, id.Invalid, sx.MakeList(
		sx.Int64(version.Major),
		sx.Int64(version.Minor),
		sx.Int64(version.Patch),
		sx.MakeString(version.PreRelease),
		sx.MakeString(version.Build),
	))
	if err != nil {
		a.logger.Error("Write version info", "err", err)
	}
}

func (a *WebAPI) writeIsReady(w http.ResponseWriter, isReady bool) {
	var result []byte

	h := w.Header()
	h.Add("Cache-Control", "no-store")
	if isReady {
		w.WriteHeader(http.StatusOK)
		result = append(result, "ready\n"...)
	} else {
		h.Add("Retry-After", "1") // 1s, can be constant
		w.WriteHeader(http.StatusServiceUnavailable)
		result = append(result, "incomplete\n"...)
	}

	if _, err := w.Write(result); err != nil {
		a.logger.Error("IsReady", "err", err)
	}
}

func (a *WebAPI) writeSiteInfo(w http.ResponseWriter, siteInfo webapi.SiteInfo) {
	err := a.writeObject(w, id.Invalid, sx.MakeList(
		sx.MakeString(siteInfo.SiteName),
		sx.MakeString(siteInfo.DefaultLang),
		sx.MakeString(siteInfo.HomeZettel.String()),
		sx.MakeString(siteInfo.Lang),
	))
	if err != nil {
		a.logger.Error("Write site info", "err", err)
	}
}
