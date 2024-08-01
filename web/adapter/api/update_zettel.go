//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package api

import (
	"net/http"

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
)

// MakeUpdateZettelHandler creates a new HTTP handler to update a zettel.
func (a *API) MakeUpdateZettelHandler(updateZettel *usecase.UpdateZettel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zid, err := id.Parse(r.URL.Path[1:])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()
		var zettel zettel.Zettel
		switch enc, _ := getEncoding(r, q); enc {
		case api.EncoderPlain:
			zettel, err = buildZettelFromPlainData(r, zid)
		case api.EncoderData:
			zettel, err = buildZettelFromData(r, zid)
		default:
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if err != nil {
			a.reportUsecaseError(w, adapter.NewErrBadRequest(err.Error()))
			return
		}
		if err = updateZettel.Run(r.Context(), zettel, true); err != nil {
			a.reportUsecaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
