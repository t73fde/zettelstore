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

package api

import (
	"net/http"
	"time"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter"
)

// MakePostLoginHandler creates a new HTTP handler to authenticate the given user via API.
func (a *API) MakePostLoginHandler(ucAuth *usecase.Authenticate) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.withAuth() {
			if err := a.writeToken(w, "freeaccess", 24*366*10*time.Hour); err != nil {
				a.dlog.Error().Err(err).Msg("Login/free")
			}
			return
		}
		var token []byte
		if ident, cred := retrieveIdentCred(r); ident != "" {
			var err error
			token, err = ucAuth.Run(r.Context(), r, ident, cred, a.tokenLifetime, auth.KindAPI)
			if err != nil {
				a.reportUsecaseError(w, err)
				return
			}
		}
		if len(token) == 0 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="Default"`)
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		if err := a.writeToken(w, string(token), a.tokenLifetime); err != nil {
			a.dlog.Error().Err(err).Msg("Login")
		}
	})
}

func retrieveIdentCred(r *http.Request) (string, string) {
	if ident, cred, ok := adapter.GetCredentialsViaForm(r); ok {
		return ident, cred
	}
	if ident, cred, ok := r.BasicAuth(); ok {
		return ident, cred
	}
	return "", ""
}

// MakeRenewAuthHandler creates a new HTTP handler to renew the authenticate of a user.
func (a *API) MakeRenewAuthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if !a.withAuth() {
			if err := a.writeToken(w, "freeaccess", 24*366*10*time.Hour); err != nil {
				a.dlog.Error().Err(err).Msg("Refresh/free")
			}
			return
		}
		authData := a.getAuthData(ctx)
		if authData == nil || len(authData.Token) == 0 || authData.User == nil {
			adapter.BadRequest(w, "Not authenticated")
			return
		}
		totalLifetime := authData.Expires.Sub(authData.Issued)
		currentLifetime := authData.Now.Sub(authData.Issued)
		// If we are in the first quarter of the tokens lifetime, return the token
		if currentLifetime*4 < totalLifetime {
			if err := a.writeToken(w, string(authData.Token), totalLifetime-currentLifetime); err != nil {
				a.dlog.Error().Err(err).Msg("Write old token")
			}
			return
		}

		// Token is a little bit aged. Create a new one
		token, err := a.getToken(authData.User)
		if err != nil {
			a.reportUsecaseError(w, err)
			return
		}
		if err = a.writeToken(w, string(token), a.tokenLifetime); err != nil {
			a.dlog.Error().Err(err).Msg("Write renewed token")
		}
	})
}

func (a *API) writeToken(w http.ResponseWriter, token string, lifetime time.Duration) error {
	return a.writeObject(w, id.Invalid, sx.MakeList(
		sx.MakeString("Bearer"),
		sx.MakeString(token),
		sx.Int64(int64(lifetime/time.Second)),
	))
}
