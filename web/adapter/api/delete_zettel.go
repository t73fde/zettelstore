//-----------------------------------------------------------------------------
// Copyright (c) 2021 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package api provides api handlers for web requests.
package api

import (
	"net/http"

	"zettelstore.de/z/domain/id"
	"zettelstore.de/z/usecase"
)

// MakeDeleteZettelHandler creates a new HTTP handler to delete a zettel.
func (a *API) MakeDeleteZettelHandler(deleteZettel usecase.DeleteZettel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		zid, err := id.Parse(r.URL.Path[1:])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if err = deleteZettel.Run(r.Context(), zid); err != nil {
			a.reportUsecaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
