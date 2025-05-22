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

// Package server provides the Zettelstore web service.
package server

import (
	"context"
	"net/http"
	"time"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

// UserRetriever allows to retrieve user data based on a given zettel identifier.
type UserRetriever interface {
	GetUser(ctx context.Context, zid id.Zid, ident string) (*meta.Meta, error)
}

// Method enumerates the allowed HTTP methods.
type Method uint8

// Values for method type
const (
	MethodGet Method = iota
	MethodHead
	MethodPost
	MethodPut
	MethodDelete
	methodLAST // must always be the last one
)

// Router allows to state routes for various URL paths.
type Router interface {
	Handle(pattern string, handler http.Handler)
	AddListRoute(key byte, method Method, handler http.Handler)
	AddZettelRoute(key byte, method Method, handler http.Handler)
	SetUserRetriever(ur UserRetriever)
}

// Builder allows to build new URLs for the web service.
type Builder interface {
	GetURLPrefix() string
	NewURLBuilder(key byte) *api.URLBuilder
	NewURLBuilderAbs(key byte) *api.URLBuilder
}

// Auth is the authencation interface.
type Auth interface {
	// SetToken sends the token to the client.
	SetToken(w http.ResponseWriter, token []byte, d time.Duration)

	// ClearToken invalidates the session cookie by sending an empty one.
	ClearToken(ctx context.Context, w http.ResponseWriter) context.Context
}

// AuthData stores all relevant authentication data for a context.
type AuthData struct {
	User    *meta.Meta
	Token   []byte
	Now     time.Time
	Issued  time.Time
	Expires time.Time
}

// GetAuthData returns the full authentication data from the context.
func GetAuthData(ctx context.Context) *AuthData {
	if ctx != nil {
		if data, ok := ctx.Value(ctxKeyTypeSession{}).(*AuthData); ok {
			return data
		}
	}
	return nil
}

// GetUser returns the metadata of the current user, or nil if there is no one.
func GetUser(ctx context.Context) *meta.Meta {
	if data := GetAuthData(ctx); data != nil {
		return data.User
	}
	return nil
}

// ctxKeyTypeSession is just an additional type to make context value retrieval unambiguous.
type ctxKeyTypeSession struct{}

// AuthBuilder is a Builder that also allows to execute authentication functions.
type AuthBuilder interface {
	Auth
	Builder
}

// Server is the main web server for accessing Zettelstore via HTTP.
type Server interface {
	Router
	Auth
	Builder

	Run() error
	Stop()
}
