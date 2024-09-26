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

// Package impl provides the Zettelstore web service.
package impl

import (
	"context"
	"net/http"
	"time"

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/auth"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/zettel/meta"
)

type myServer struct {
	log              *logger.Logger
	baseURL          string
	server           httpServer
	router           httpRouter
	persistentCookie bool
	secureCookie     bool
}

type ServerData struct {
	Log              *logger.Logger
	ListenAddr       string
	BaseURL          string
	URLPrefix        string
	MaxRequestSize   int64
	Auth             auth.TokenManager
	PersistentCookie bool
	SecureCookie     bool
	Profiling        bool
}

// New creates a new web server.
func New(sd ServerData) server.Server {
	srv := myServer{
		log:              sd.Log,
		baseURL:          sd.BaseURL,
		persistentCookie: sd.PersistentCookie,
		secureCookie:     sd.SecureCookie,
	}

	rd := routerData{
		log:            sd.Log,
		urlPrefix:      sd.URLPrefix,
		maxRequestSize: sd.MaxRequestSize,
		auth:           sd.Auth,
		profiling:      sd.Profiling,
	}
	srv.router.initializeRouter(rd)
	srv.server.initializeHTTPServer(sd.ListenAddr, &srv.router)
	return &srv
}

func (srv *myServer) Handle(pattern string, handler http.Handler) {
	srv.router.Handle(pattern, handler)
}
func (srv *myServer) AddListRoute(key byte, method server.Method, handler http.Handler) {
	srv.router.addListRoute(key, method, handler)
}
func (srv *myServer) AddZettelRoute(key byte, method server.Method, handler http.Handler) {
	srv.router.addZettelRoute(key, method, handler)
}
func (srv *myServer) SetUserRetriever(ur server.UserRetriever) {
	srv.router.ur = ur
}

func (srv *myServer) GetURLPrefix() string {
	return srv.router.urlPrefix
}
func (srv *myServer) NewURLBuilder(key byte) *api.URLBuilder {
	return api.NewURLBuilder(srv.GetURLPrefix(), key)
}
func (srv *myServer) NewURLBuilderAbs(key byte) *api.URLBuilder {
	return api.NewURLBuilder(srv.baseURL, key)
}

const sessionName = "zsession"

func (srv *myServer) SetToken(w http.ResponseWriter, token []byte, d time.Duration) {
	cookie := http.Cookie{
		Name:     sessionName,
		Value:    string(token),
		Path:     srv.GetURLPrefix(),
		Secure:   srv.secureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if srv.persistentCookie && d > 0 {
		cookie.Expires = time.Now().Add(d).Add(30 * time.Second).UTC()
	}
	srv.log.Debug().Bytes("token", token).Msg("SetToken")
	if v := cookie.String(); v != "" {
		w.Header().Add("Set-Cookie", v)
		w.Header().Add("Cache-Control", `no-cache="Set-Cookie"`)
		w.Header().Add("Vary", "Cookie")
	}
}

// ClearToken invalidates the session cookie by sending an empty one.
func (srv *myServer) ClearToken(ctx context.Context, w http.ResponseWriter) context.Context {
	if authData := server.GetAuthData(ctx); authData == nil {
		// No authentication data stored in session, nothing to do.
		return ctx
	}
	if w != nil {
		srv.SetToken(w, nil, 0)
	}
	return updateContext(ctx, nil, nil)
}

func updateContext(ctx context.Context, user *meta.Meta, data *auth.TokenData) context.Context {
	if data == nil {
		return context.WithValue(ctx, server.CtxKeySession, &server.AuthData{User: user})
	}
	return context.WithValue(
		ctx,
		server.CtxKeySession,
		&server.AuthData{
			User:    user,
			Token:   data.Token,
			Now:     data.Now,
			Issued:  data.Issued,
			Expires: data.Expires,
		})
}

func (srv *myServer) SetDebug()  { srv.server.SetDebug() }
func (srv *myServer) Run() error { return srv.server.Run() }
func (srv *myServer) Stop()      { srv.server.Stop() }
