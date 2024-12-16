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
	"net/http"
	"sync"
	"time"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxeval"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/api"
	"zettelstore.de/z/auth"
	"zettelstore.de/z/box"
	"zettelstore.de/z/config"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// WebUI holds all data for delivering the web ui.
type WebUI struct {
	log      *logger.Logger
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
	homeURL       string
	listZettelURL string
	listRolesURL  string
	listTagsURL   string
	refreshURL    string
	withAuth      bool
	loginURL      string
	logoutURL     string
	searchURL     string
	createNewURL  string

	rootBinding     *sxeval.Binding
	mxZettelBinding sync.Mutex
	zettelBinding   *sxeval.Binding
	dag             id.Digraph
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
func New(log *logger.Logger, ab server.AuthBuilder, authz auth.AuthzManager, rtConfig config.Config, token auth.TokenManager,
	mgr box.Manager, pol auth.Policy, evalZettel *usecase.Evaluate) *WebUI {
	loginoutBase := ab.NewURLBuilder('i')

	wui := &WebUI{
		log:      log,
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
		cssBaseURL:    ab.NewURLBuilder('z').SetZid(api.ZidBaseCSS).String(),
		cssUserURL:    ab.NewURLBuilder('z').SetZid(api.ZidUserCSS).String(),
		homeURL:       ab.NewURLBuilder('/').String(),
		listZettelURL: ab.NewURLBuilder('h').String(),
		listRolesURL:  ab.NewURLBuilder('h').AppendQuery(api.ActionSeparator + api.KeyRole).String(),
		listTagsURL:   ab.NewURLBuilder('h').AppendQuery(api.ActionSeparator + api.KeyTags).String(),
		refreshURL:    ab.NewURLBuilder('g').AppendKVQuery("_c", "r").String(),
		withAuth:      authz.WithAuth(),
		loginURL:      loginoutBase.String(),
		logoutURL:     loginoutBase.AppendKVQuery("logout", "").String(),
		searchURL:     ab.NewURLBuilder('h').String(),
		createNewURL:  ab.NewURLBuilder('c').String(),

		zettelBinding: nil,
		genHTML:       sxhtml.NewGenerator().SetNewline(),
	}
	wui.rootBinding = wui.createRenderBinding()
	wui.observe(box.UpdateInfo{Box: mgr, Reason: box.OnReload, Zid: id.Invalid})
	mgr.RegisterObserver(wui.observe)
	return wui
}

func (wui *WebUI) getConfig(ctx context.Context, m *meta.Meta, key string) string {
	return wui.rtConfig.Get(ctx, m, key)
}
func (wui *WebUI) getUserLang(ctx context.Context) string {
	return wui.getConfig(ctx, nil, api.KeyLang)
}

var (
	symDetail     = sx.MakeSymbol("DETAIL")
	symMetaHeader = sx.MakeSymbol("META-HEADER")
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

// GetURLPrefix returns the configured URL prefix of the web server.
func (wui *WebUI) GetURLPrefix() string { return wui.ab.GetURLPrefix() }

// NewURLBuilder creates a new URL builder object with the given key.
func (wui *WebUI) NewURLBuilder(key byte) *api.URLBuilder { return wui.ab.NewURLBuilder(key) }

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
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	if !wui.debug {
		h.Set("X-Frame-Options", "sameorigin")
	}
	w.WriteHeader(statusCode)
}
