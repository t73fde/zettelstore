//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package adapter provides handlers for web requests.
package adapter

import (
	"context"
	"net/http"

	"zettelstore.de/z/config"
	"zettelstore.de/z/domain"
)

type getRootStore interface {
	// GetMeta retrieves just the meta data of a specific zettel.
	GetMeta(ctx context.Context, zid domain.ZettelID) (*domain.Meta, error)
}

// MakeGetRootHandler creates a new HTTP handler to show the root URL.
func MakeGetRootHandler(s getRootStore, startNotFound, startFound http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		startID := config.GetStart()
		if startID.IsValid() {
			if _, err := s.GetMeta(r.Context(), startID); err == nil {
				r.URL.Path = "/" + startID.Format()
				startFound(w, r)
				return
			}
		}
		startNotFound(w, r)
	}
}
