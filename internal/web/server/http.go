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
	"log/slog"
	"net"
	"net/http"
	"time"

	"t73f.de/r/webs/middleware"
	"t73f.de/r/webs/middleware/logging"
	"t73f.de/r/webs/middleware/reqid"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/auth"
)

type webServer struct {
	log              *slog.Logger
	baseURL          string
	httpServer       httpServer
	router           httpRouter
	cop              *http.CrossOriginProtection
	persistentCookie bool
	secureCookie     bool
}

// ConfigData contains the data needed to configure a server.
type ConfigData struct {
	Log              *slog.Logger
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
		cop:              http.NewCrossOriginProtection(),
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

	mwReqID := reqid.Config{WithContext: true}
	mwLogReq := logging.ReqConfig{
		Logger: sd.Log, Level: slog.LevelDebug,
		Message: "ServeHTTP", WithRequestID: true, WithRemote: true, WithHeaders: true}
	mwLogResp := logging.RespConfig{Logger: sd.Log, Level: slog.LevelDebug,
		Message: "/ServeHTTP", WithRequestID: true}
	mw := middleware.NewChain(mwReqID.Build(), mwLogReq.Build(), mwLogResp.Build())

	srv.httpServer.initializeHTTPServer(sd.ListenAddr, middleware.Apply(mw, &srv.router))
	return &srv
}

func (srv *webServer) Handle(pattern string, handler http.Handler) {
	srv.router.Handle(pattern, handler)
}
func (srv *webServer) AddListRoute(isAPI bool, key byte, method Method, handler http.Handler) {
	if !isAPI {
		handler = srv.cop.Handler(handler)
	}
	srv.router.addListRoute(key, method, handler)
}
func (srv *webServer) AddZettelRoute(isAPI bool, key byte, method Method, handler http.Handler) {
	if !isAPI {
		handler = srv.cop.Handler(handler)
	}
	srv.router.addZettelRoute(key, method, handler)
}
func (srv *webServer) SetUserRetriever(ur UserRetriever) {
	srv.router.ur = ur
}

func (srv *webServer) NewURLBuilder(key byte) *webapi.URLBuilder {
	return webapi.NewURLBuilder(srv.router.urlPrefix, key)
}
func (srv *webServer) NewURLBuilderAbs(key byte) *webapi.URLBuilder {
	return webapi.NewURLBuilder(srv.baseURL, key)
}

const sessionName = "zsession"

func (srv *webServer) SetToken(w http.ResponseWriter, token []byte, d time.Duration) {
	cookie := http.Cookie{
		Name:     sessionName,
		Value:    string(token),
		Path:     srv.router.urlPrefix,
		Secure:   srv.secureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if srv.persistentCookie && d > 0 {
		cookie.Expires = time.Now().Add(d).Add(30 * time.Second).UTC()
	}
	srv.log.Debug("SetToken", "token", cookie.Value)
	if v := cookie.String(); v != "" {
		w.Header().Add("Set-Cookie", v)
		w.Header().Add("Cache-Control", `no-cache="Set-Cookie"`)
		w.Header().Add("Vary", "Cookie")
	}
}

// ClearToken invalidates the session cookie by sending an empty one.
func (srv *webServer) ClearToken(ctx context.Context, w http.ResponseWriter) context.Context {
	if authData := auth.GetAuthData(ctx); authData == nil {
		// No authentication data stored in session, nothing to do.
		return ctx
	}
	if w != nil {
		srv.SetToken(w, nil, 0)
	}
	return auth.UpdateContext(ctx, nil, nil)
}

func (srv *webServer) Run() error { return srv.httpServer.start() }
func (srv *webServer) Stop()      { srv.httpServer.stop() }

// Server timeout values
const shutdownTimeout = 5 * time.Second

// httpServer is an HTTP server.
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
