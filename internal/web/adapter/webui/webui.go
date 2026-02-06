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

// Package webui provides web-UI handlers for web requests.
package webui

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxeval"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zero/graph"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter"
	"zettelstore.de/z/internal/web/server"
	"zettelstore.de/z/internal/zettel"
)

// WebUI holds all data for delivering the web ui.
type WebUI struct {
	logger   *slog.Logger
	debug    bool
	ab       server.AuthBuilder
	authz    auth.AuthzManager
	rtConfig config.Config
	token    auth.TokenManager
	box      webuiBox
	policy   auth.Policy

	evalZettel *usecase.Evaluate

	mxCache       sync.RWMutex
	templateCache map[id.Zid]sxeval.Expr

	tokenLifetime time.Duration
	cssBaseURL    string
	cssUserURL    string
	jsBaseURL     string
	jsCopyRefURL  string
	homeURL       string
	refreshURL    string
	withAuth      bool
	loginURL      string
	logoutURL     string
	searchURL     string
	createNewURL  string

	sxMaxNesting    int
	rootBinding     *sxeval.Binding
	mxZettelBinding sync.Mutex
	zettelBinding   *sxeval.Binding
	dag             graph.Digraph[id.Zid]
	genHTML         *sxhtml.Generator
}

// webuiBox contains all box methods that are needed for WebUI operation.
//
// Note: these function must not do auth checking.
type webuiBox interface {
	CanCreateZettel(context.Context) bool
	GetZettel(context.Context, id.Zid) (zettel.Zettel, error)
	GetMeta(context.Context, id.Zid) (*meta.Meta, error)
	CanUpdateZettel(context.Context, zettel.Zettel) bool
	CanDeleteZettel(context.Context, id.Zid) bool
}

// New creates a new WebUI struct.
func New(logger *slog.Logger, ab server.AuthBuilder, authz auth.AuthzManager, rtConfig config.Config, token auth.TokenManager,
	mgr box.Manager, pol auth.Policy, evalZettel *usecase.Evaluate) *WebUI {
	loginoutBase := ab.NewURLBuilder('i')

	wui := &WebUI{
		logger:   logger,
		debug:    kernel.Main.GetConfig(kernel.CoreService, kernel.CoreDebug).(bool),
		ab:       ab,
		rtConfig: rtConfig,
		authz:    authz,
		token:    token,
		box:      mgr,
		policy:   pol,

		evalZettel: evalZettel,

		templateCache: make(map[id.Zid]sxeval.Expr, 32),

		tokenLifetime: kernel.Main.GetConfig(kernel.WebService, kernel.WebTokenLifetimeHTML).(time.Duration),
		cssBaseURL:    ab.NewURLBuilder('z').SetZid(id.ZidBaseCSS).String(),
		cssUserURL:    ab.NewURLBuilder('z').SetZid(id.ZidUserCSS).String(),
		jsBaseURL:     ab.NewURLBuilder('z').SetZid(id.ZidBaseJS).String(),
		jsCopyRefURL:  ab.NewURLBuilder('z').SetZid(id.ZidCopyRefJS).String(),
		homeURL:       ab.NewURLBuilder('/').String(),
		refreshURL:    ab.NewURLBuilder('g').AppendKVQuery("_c", "r").String(),
		withAuth:      authz.WithAuth(),
		loginURL:      loginoutBase.String(),
		logoutURL:     loginoutBase.AppendKVQuery("logout", "").String(),
		searchURL:     ab.NewURLBuilder('h').String(),
		createNewURL:  ab.NewURLBuilder('c').String(),

		sxMaxNesting:  min(max(kernel.Main.GetConfig(kernel.WebService, kernel.WebSxMaxNesting).(int), 0), math.MaxInt),
		zettelBinding: nil,
		genHTML:       sxhtml.NewGenerator().SetNewline(),
	}
	wui.rootBinding = wui.createRootBinding()
	wui.observe(box.UpdateInfo{Box: mgr, Reason: box.OnReload, Zid: id.Invalid})
	mgr.RegisterObserver(wui.observe)
	return wui
}

func (wui *WebUI) getConfig(ctx context.Context, m *meta.Meta, key string) string {
	return wui.rtConfig.Get(ctx, m, key)
}
func (wui *WebUI) getUserLang(ctx context.Context) string {
	return wui.getConfig(ctx, nil, meta.KeyLang)
}

var (
	symDetail         = sx.MakeSymbol("DETAIL")
	symMetaHeader     = sx.MakeSymbol("META-HEADER")
	symJSScripts      = sx.MakeSymbol("JS-SCRIPTS")
	symJSScriptsAsync = sx.MakeSymbol("JS-SCRIPTS-ASYNC")
)

func (wui *WebUI) observe(ci box.UpdateInfo) {
	wui.mxCache.Lock()
	if ci.Reason == box.OnReload {
		clear(wui.templateCache)
	} else {
		delete(wui.templateCache, ci.Zid)
	}
	wui.mxCache.Unlock()

	wui.mxZettelBinding.Lock()
	if ci.Reason == box.OnReload || wui.dag.HasVertex(ci.Zid) {
		wui.zettelBinding = nil
		wui.dag = nil
	}
	wui.mxZettelBinding.Unlock()
}

func (wui *WebUI) setSxnCache(zid id.Zid, expr sxeval.Expr) {
	wui.mxCache.Lock()
	wui.templateCache[zid] = expr
	wui.mxCache.Unlock()
}
func (wui *WebUI) getSxnCache(zid id.Zid) sxeval.Expr {
	wui.mxCache.RLock()
	expr, found := wui.templateCache[zid]
	wui.mxCache.RUnlock()
	if found {
		return expr
	}
	return nil
}

func (wui *WebUI) canCreate(ctx context.Context, user *meta.Meta) bool {
	m := meta.New(id.Invalid)
	return wui.policy.CanCreate(user, m) && wui.box.CanCreateZettel(ctx)
}

func (wui *WebUI) canWrite(
	ctx context.Context, user, meta *meta.Meta, content zettel.Content) bool {
	return wui.policy.CanWrite(user, meta, meta) &&
		wui.box.CanUpdateZettel(ctx, zettel.Zettel{Meta: meta, Content: content})
}

func (wui *WebUI) canDelete(ctx context.Context, user, m *meta.Meta) bool {
	return wui.policy.CanDelete(user, m) && wui.box.CanDeleteZettel(ctx, m.Zid)
}

func (wui *WebUI) canRefresh(user *meta.Meta) bool {
	return wui.policy.CanRefresh(user)
}

func (wui *WebUI) getSimpleHTMLEncoder(lang string) *htmlGenerator {
	return wui.createGenerator(wui, lang)
}

// NewURLBuilder creates a new URL builder object with the given key.
func (wui *WebUI) NewURLBuilder(key byte) *webapi.URLBuilder { return wui.ab.NewURLBuilder(key) }

func (wui *WebUI) clearToken(ctx context.Context, w http.ResponseWriter) context.Context {
	return wui.ab.ClearToken(ctx, w)
}

func (wui *WebUI) setToken(w http.ResponseWriter, token []byte) {
	wui.ab.SetToken(w, token, wui.tokenLifetime)
}

func (wui *WebUI) prepareAndWriteHeader(w http.ResponseWriter, statusCode int) {
	h := adapter.PrepareHeader(w, "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'self'; img-src * data:; style-src 'self' 'unsafe-inline'")
	h.Set("Permissions-Policy", "payment=(), interest-cohort=()")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("X-Content-Type-Options", "nosniff")
	if !wui.debug {
		h.Set("X-Frame-Options", "sameorigin")
	}
	w.WriteHeader(statusCode)
}
