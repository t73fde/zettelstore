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
	"io"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"regexp"
	rtprf "runtime/pprof"
	"strings"

	"t73f.de/r/webs/ip"
	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/auth/user"
)

type (
	methodHandler [methodLAST]http.Handler
	routingTable  [256]*methodHandler
)

var mapMethod = map[string]Method{
	http.MethodHead:   MethodHead,
	http.MethodGet:    MethodGet,
	http.MethodPost:   MethodPost,
	http.MethodPut:    MethodPut,
	http.MethodDelete: MethodDelete,
}

// httpRouter handles all routing for zettelstore.
type httpRouter struct {
	log           *slog.Logger
	urlPrefix     string
	auth          auth.TokenManager
	loopbackIdent string
	loopbackZid   id.Zid
	minKey        byte
	maxKey        byte
	reURL         *regexp.Regexp
	listTable     routingTable
	zettelTable   routingTable
	ur            UserRetriever
	mux           *http.ServeMux
	maxReqSize    int64
}

type routerData struct {
	log            *slog.Logger
	urlPrefix      string
	maxRequestSize int64
	auth           auth.TokenManager
	loopbackIdent  string
	loopbackZid    id.Zid
	profiling      bool
}

// initializeRouter creates a new, empty router with the given root handler.
func (rt *httpRouter) initializeRouter(rd routerData) {
	rt.log = rd.log
	rt.urlPrefix = rd.urlPrefix
	rt.auth = rd.auth
	rt.loopbackIdent = rd.loopbackIdent
	rt.loopbackZid = rd.loopbackZid
	rt.minKey = 255
	rt.maxKey = 0
	rt.reURL = regexp.MustCompile("^$")
	rt.mux = http.NewServeMux()
	rt.maxReqSize = rd.maxRequestSize

	if rd.profiling {
		rt.setRuntimeProfiling()
	}
}

func (rt *httpRouter) setRuntimeProfiling() {
	rt.mux.HandleFunc("GET /rtp/", pprof.Index)
	for _, profile := range rtprf.Profiles() {
		name := profile.Name()
		rt.mux.Handle("GET /rtp/"+name, pprof.Handler(name))
	}
	rt.mux.HandleFunc("GET /rtp/cmdline", pprof.Cmdline)
	rt.mux.HandleFunc("GET /rtp/profile", pprof.Profile)
	rt.mux.HandleFunc("GET /rtp/symbol", pprof.Symbol)
	rt.mux.HandleFunc("GET /rtp/trace", pprof.Trace)
}

func (rt *httpRouter) addRoute(key byte, method Method, handler http.Handler, table *routingTable) {
	// Set minKey and maxKey; re-calculate regexp.
	if key < rt.minKey || rt.maxKey < key {
		if key < rt.minKey {
			rt.minKey = key
		}
		if rt.maxKey < key {
			rt.maxKey = key
		}
		rt.reURL = regexp.MustCompile(
			"^/(?:([" + string(rt.minKey) + "-" + string(rt.maxKey) + "])(?:/(?:([0-9]{14})/?)?)?)$")
	}

	mh := table[key]
	if mh == nil {
		mh = new(methodHandler)
		table[key] = mh
	}
	mh[method] = handler
	if method == MethodGet {
		if prevHandler := mh[MethodHead]; prevHandler == nil {
			mh[MethodHead] = handler
		}
	}
}

// addListRoute adds a route for the given key and HTTP method to work with a list.
func (rt *httpRouter) addListRoute(key byte, method Method, handler http.Handler) {
	rt.addRoute(key, method, handler, &rt.listTable)
}

// addZettelRoute adds a route for the given key and HTTP method to work with a zettel.
func (rt *httpRouter) addZettelRoute(key byte, method Method, handler http.Handler) {
	rt.addRoute(key, method, handler, &rt.zettelTable)
}

// Handle registers the handler for the given pattern. If a handler already exists for pattern, Handle panics.
func (rt *httpRouter) Handle(pattern string, handler http.Handler) {
	rt.mux.Handle(pattern, handler)
}

func (rt *httpRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var withDebug bool
	if rt.log.Enabled(r.Context(), slog.LevelDebug) {
		withDebug = true
		w = &traceResponseWriter{original: w}
		rt.log.Debug("ServeHTTP", "method", r.Method, "uri", r.RequestURI, "remote", ip.GetRemoteAddr(r))
	}

	if prefixLen := len(rt.urlPrefix); prefixLen > 1 {
		if len(r.URL.Path) < prefixLen || r.URL.Path[:prefixLen] != rt.urlPrefix {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			if withDebug {
				rt.log.Debug("/ServeHTTP/prefix", "sc", w.(*traceResponseWriter).statusCode)
			}
			return
		}
		r.URL.Path = r.URL.Path[prefixLen-1:]
	}
	r.Body = http.MaxBytesReader(w, r.Body, rt.maxReqSize)
	match := rt.reURL.FindStringSubmatch(r.URL.Path)
	if len(match) != 3 {
		rt.mux.ServeHTTP(w, rt.addUserContext(r))
		if withDebug {
			rt.log.Debug("match other", "sc", w.(*traceResponseWriter).statusCode)
		}
		return
	}
	if withDebug {
		rt.log.Debug("path match", "key", match[1], "zid", match[2])
	}

	key := match[1][0]
	var mh *methodHandler
	if match[2] == "" {
		mh = rt.listTable[key]
	} else {
		mh = rt.zettelTable[key]
	}
	method, ok := mapMethod[r.Method]
	if ok && mh != nil {
		if handler := mh[method]; handler != nil {
			r.URL.Path = "/" + match[2]
			handler.ServeHTTP(w, rt.addUserContext(r))
			if withDebug {
				rt.log.Debug("/ServeHTTP", "sc", w.(*traceResponseWriter).statusCode)
			}
			return
		}
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	if withDebug {
		rt.log.Debug("no match", "sc", w.(*traceResponseWriter).statusCode)
	}
}

func (rt *httpRouter) addUserContext(r *http.Request) *http.Request {
	if rt.ur == nil {
		// No auth needed
		return r
	}
	ctx := r.Context()

	if rt.loopbackZid.IsValid() {
		if remoteAddr := ip.GetRemoteAddr(r); ip.IsLoopbackAddr(remoteAddr) {
			if u, err := rt.ur.GetUser(ctx, rt.loopbackZid, rt.loopbackIdent); err == nil {
				if u != nil {
					return r.WithContext(user.UpdateContext(ctx, u, nil))
				}
				rt.log.Error("No match to loopback-zid", "loopback-ident", rt.loopbackIdent)
			}
		}
	}

	k := auth.KindAPI
	t := getHeaderToken(r)
	if len(t) == 0 {
		rt.log.Debug("no jwt token found") // IP already logged: ServeHTTP
		k = auth.KindwebUI
		t = getSessionToken(r)
	}
	if len(t) == 0 {
		rt.log.Debug("no auth token found in request") // IP already logged: ServeHTTP
		return r
	}
	tokenData, err := rt.auth.CheckToken(t, k)
	if err != nil {
		rt.log.Info("invalid auth token", "err", err, "remote", ip.GetRemoteAddr(r))
		return r
	}
	u, err := rt.ur.GetUser(ctx, tokenData.Zid, tokenData.Ident)
	if err != nil {
		rt.log.Info("auth user not found",
			"zid", tokenData.Zid,
			"ident", tokenData.Ident,
			"err", err,
			"remote", ip.GetRemoteAddr(r))
		return r
	}
	return r.WithContext(user.UpdateContext(ctx, u, &tokenData))
}

func getSessionToken(r *http.Request) []byte {
	cookie, err := r.Cookie(sessionName)
	if err != nil {
		return nil
	}
	return []byte(cookie.Value)
}

func getHeaderToken(r *http.Request) []byte {
	h := r.Header["Authorization"]
	if h == nil {
		return nil
	}

	// “Multiple message-header fields with the same field-name MAY be
	// present in a message if and only if the entire field-value for that
	// header field is defined as a comma-separated list.”
	// — “Hypertext Transfer Protocol” RFC 2616, subsection 4.2
	auth := strings.Join(h, ", ")

	const prefix = "Bearer "
	// RFC 2617, subsection 1.2 defines the scheme token as case-insensitive.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return nil
	}
	return []byte(auth[len(prefix):])
}

type traceResponseWriter struct {
	original   http.ResponseWriter
	statusCode int
}

func (w *traceResponseWriter) Header() http.Header         { return w.original.Header() }
func (w *traceResponseWriter) Write(p []byte) (int, error) { return w.original.Write(p) }
func (w *traceResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.original.WriteHeader(statusCode)
}
func (w *traceResponseWriter) WriteString(s string) (int, error) {
	return io.WriteString(w.original, s)
}
