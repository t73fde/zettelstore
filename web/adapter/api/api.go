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

// Package api provides api handlers for web requests.
package api

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/auth"
	"zettelstore.de/z/config"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/server"
)

// API holds all data and methods for delivering API call results.
type API struct {
	log      *logger.Logger
	b        server.Builder
	authz    auth.AuthzManager
	token    auth.TokenManager
	rtConfig config.Config
	policy   auth.Policy

	tokenLifetime time.Duration
}

// New creates a new API object.
func New(log *logger.Logger, b server.Builder, authz auth.AuthzManager, token auth.TokenManager,
	rtConfig config.Config, pol auth.Policy) *API {
	a := &API{
		log:      log,
		b:        b,
		authz:    authz,
		token:    token,
		rtConfig: rtConfig,
		policy:   pol,

		tokenLifetime: kernel.Main.GetConfig(kernel.WebService, kernel.WebTokenLifetimeAPI).(time.Duration),
	}
	return a
}

// NewURLBuilder creates a new URL builder object with the given key.
func (a *API) NewURLBuilder(key byte) *api.URLBuilder { return a.b.NewURLBuilder(key) }

func (a *API) getAuthData(ctx context.Context) *server.AuthData {
	return server.GetAuthData(ctx)
}
func (a *API) withAuth() bool { return a.authz.WithAuth() }
func (a *API) getToken(ident *meta.Meta) ([]byte, error) {
	return a.token.GetToken(ident, a.tokenLifetime, auth.KindAPI)
}

func (a *API) reportUsecaseError(w http.ResponseWriter, err error) {
	code, text := adapter.CodeMessageFromError(err)
	if code == http.StatusInternalServerError {
		a.log.Error().Err(err).Msg(text)
		http.Error(w, http.StatusText(code), code)
		return
	}
	// TODO: must call PrepareHeader somehow
	http.Error(w, text, code)
}

func writeBuffer(w http.ResponseWriter, buf *bytes.Buffer, contentType string) error {
	return adapter.WriteData(w, buf.Bytes(), contentType)
}

func (a *API) getRights(ctx context.Context, m *meta.Meta) (result api.ZettelRights) {
	pol := a.policy
	user := server.GetUser(ctx)
	if pol.CanCreate(user, m) {
		result |= api.ZettelCanCreate
	}
	if pol.CanRead(user, m) {
		result |= api.ZettelCanRead
	}
	if pol.CanWrite(user, m, m) {
		result |= api.ZettelCanWrite
	}
	if pol.CanDelete(user, m) {
		result |= api.ZettelCanDelete
	}
	if result == 0 {
		return api.ZettelCanNone
	}
	return result
}
