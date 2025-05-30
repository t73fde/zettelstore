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

package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/logger"
)

type webServer struct {
	log              *logger.DLogger
	baseURL          string
	httpServer       httpServer
	router           httpRouter
	persistentCookie bool
	secureCookie     bool
}

// ConfigData contains the data needed to configure a server.
type ConfigData struct {
	Log              *logger.DLogger
	ListenAddr       string
	BaseURL          string
	URLPrefix        string
	MaxRequestSize   int64
	Auth             auth.TokenManager
	LoopbackIdent    string
	LoopbackZid      id.Zid
	PersistentCookie bool
	SecureCookie     bool
	Profiling        bool
}

// New creates a new web server.
func New(sd ConfigData) Server {
	srv := webServer{
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
		loopbackIdent:  sd.LoopbackIdent,
		loopbackZid:    sd.LoopbackZid,
		profiling:      sd.Profiling,
	}
	srv.router.initializeRouter(rd)
	srv.httpServer.initializeHTTPServer(sd.ListenAddr, &srv.router)
	return &srv
}

func (srv *webServer) Handle(pattern string, handler http.Handler) {
	srv.router.Handle(pattern, handler)
}
func (srv *webServer) AddListRoute(key byte, method Method, handler http.Handler) {
	srv.router.addListRoute(key, method, handler)
}
func (srv *webServer) AddZettelRoute(key byte, method Method, handler http.Handler) {
	srv.router.addZettelRoute(key, method, handler)
}
func (srv *webServer) SetUserRetriever(ur UserRetriever) {
	srv.router.ur = ur
}

func (srv *webServer) GetURLPrefix() string {
	return srv.router.urlPrefix
}
func (srv *webServer) NewURLBuilder(key byte) *api.URLBuilder {
	return api.NewURLBuilder(srv.GetURLPrefix(), key)
}
func (srv *webServer) NewURLBuilderAbs(key byte) *api.URLBuilder {
	return api.NewURLBuilder(srv.baseURL, key)
}

const sessionName = "zsession"

func (srv *webServer) SetToken(w http.ResponseWriter, token []byte, d time.Duration) {
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
func (srv *webServer) ClearToken(ctx context.Context, w http.ResponseWriter) context.Context {
	if authData := GetAuthData(ctx); authData == nil {
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
		return context.WithValue(ctx, ctxKeyTypeSession{}, &AuthData{User: user})
	}
	return context.WithValue(
		ctx,
		ctxKeyTypeSession{},
		&AuthData{
			User:    user,
			Token:   data.Token,
			Now:     data.Now,
			Issued:  data.Issued,
			Expires: data.Expires,
		})
}

func (srv *webServer) Run() error { return srv.httpServer.start() }
func (srv *webServer) Stop()      { srv.httpServer.stop() }

// Server timeout values
const shutdownTimeout = 5 * time.Second

// httpServer is a HTTP server.
type httpServer struct {
	http.Server
}

// initializeHTTPServer creates a new HTTP server object.
func (srv *httpServer) initializeHTTPServer(addr string, handler http.Handler) {
	if addr == "" {
		addr = ":http"
	}
	srv.Server = http.Server{
		Addr:    addr,
		Handler: handler,
	}
}

// start the web server, but does not wait for its completion.
func (srv *httpServer) start() error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}

	go func() { _ = srv.Serve(ln) }()
	return nil
}

// stop the web server.
func (srv *httpServer) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	_ = srv.Shutdown(ctx)
}
