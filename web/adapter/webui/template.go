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

package webui

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxbuiltins"
	"t73f.de/r/sx/sxeval"
	"t73f.de/r/sx/sxreader"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/box"
	"zettelstore.de/z/collect"
	"zettelstore.de/z/config"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/zettel"
)

func (wui *WebUI) createRenderBinding() *sxeval.Binding {
	root := sxeval.MakeRootBinding(len(specials) + len(builtins) + 3)
	for _, syntax := range specials {
		root.BindSpecial(syntax)
	}
	for _, b := range builtins {
		root.BindBuiltin(b)
	}
	_ = root.Bind(sx.MakeSymbol("NIL"), sx.Nil())
	_ = root.Bind(sx.MakeSymbol("T"), sx.MakeSymbol("T"))
	root.BindBuiltin(&sxeval.Builtin{
		Name:     "url-to-html",
		MinArity: 1,
		MaxArity: 1,
		TestPure: sxeval.AssertPure,
		Fn1: func(_ *sxeval.Environment, arg sx.Object) (sx.Object, error) {
			text, err := sxbuiltins.GetString(arg, 0)
			if err != nil {
				return nil, err
			}
			return wui.url2html(text), nil
		},
	})
	root.BindBuiltin(&sxeval.Builtin{
		Name:     "zid-content-path",
		MinArity: 1,
		MaxArity: 1,
		TestPure: sxeval.AssertPure,
		Fn1: func(_ *sxeval.Environment, arg sx.Object) (sx.Object, error) {
			s, err := sxbuiltins.GetString(arg, 0)
			if err != nil {
				return nil, err
			}
			zid, err := id.Parse(s.GetValue())
			if err != nil {
				return nil, fmt.Errorf("parsing zettel identifier %q: %w", s.GetValue(), err)
			}
			ub := wui.NewURLBuilder('z').SetZid(zid)
			return sx.MakeString(ub.String()), nil
		},
	})
	root.BindBuiltin(&sxeval.Builtin{
		Name:     "query->url",
		MinArity: 1,
		MaxArity: 1,
		TestPure: sxeval.AssertPure,
		Fn1: func(_ *sxeval.Environment, arg sx.Object) (sx.Object, error) {
			qs, err := sxbuiltins.GetString(arg, 0)
			if err != nil {
				return nil, err
			}
			u := wui.NewURLBuilder('h').AppendQuery(qs.GetValue())
			return sx.MakeString(u.String()), nil
		},
	})
	root.Freeze()
	return root
}

var (
	specials = []*sxeval.Special{
		&sxbuiltins.QuoteS, &sxbuiltins.QuasiquoteS, // quote, quasiquote
		&sxbuiltins.UnquoteS, &sxbuiltins.UnquoteSplicingS, // unquote, unquote-splicing
		&sxbuiltins.DefVarS,                     // defvar
		&sxbuiltins.DefunS, &sxbuiltins.LambdaS, // defun, lambda
		&sxbuiltins.SetXS,     // set!
		&sxbuiltins.IfS,       // if
		&sxbuiltins.BeginS,    // begin
		&sxbuiltins.DefMacroS, // defmacro
		&sxbuiltins.LetS,      // let
	}
	builtins = []*sxeval.Builtin{
		&sxbuiltins.Equal,                // =
		&sxbuiltins.NumGreater,           // >
		&sxbuiltins.NullP,                // null?
		&sxbuiltins.PairP,                // pair?
		&sxbuiltins.Car, &sxbuiltins.Cdr, // car, cdr
		&sxbuiltins.Caar, &sxbuiltins.Cadr, &sxbuiltins.Cdar, &sxbuiltins.Cddr,
		&sxbuiltins.Caaar, &sxbuiltins.Caadr, &sxbuiltins.Cadar, &sxbuiltins.Caddr,
		&sxbuiltins.Cdaar, &sxbuiltins.Cdadr, &sxbuiltins.Cddar, &sxbuiltins.Cdddr,
		&sxbuiltins.List,           // list
		&sxbuiltins.Append,         // append
		&sxbuiltins.Assoc,          // assoc
		&sxbuiltins.Map,            // map
		&sxbuiltins.Apply,          // apply
		&sxbuiltins.Concat,         // concat
		&sxbuiltins.BoundP,         // bound?
		&sxbuiltins.Defined,        // defined?
		&sxbuiltins.CurrentBinding, // current-binding
		&sxbuiltins.BindingLookup,  // binding-lookup
	}
)

func (wui *WebUI) url2html(text sx.String) sx.Object {
	if u, errURL := url.Parse(text.GetValue()); errURL == nil {
		if us := u.String(); us != "" {
			return sx.MakeList(
				shtml.SymA,
				sx.MakeList(
					sxhtml.SymAttr,
					sx.Cons(shtml.SymAttrHref, sx.MakeString(us)),
					sx.Cons(shtml.SymAttrTarget, sx.MakeString("_blank")),
					sx.Cons(shtml.SymAttrRel, sx.MakeString("external noreferrer")),
				),
				text)
		}
	}
	return text
}

func (wui *WebUI) getParentEnv(ctx context.Context) (*sxeval.Binding, error) {
	wui.mxZettelBinding.Lock()
	defer wui.mxZettelBinding.Unlock()
	if parentEnv := wui.zettelBinding; parentEnv != nil {
		return parentEnv, nil
	}
	dag, zettelEnv, err := wui.loadAllSxnCodeZettel(ctx)
	if err != nil {
		wui.log.Error().Err(err).Msg("loading zettel sxn")
		return nil, err
	}
	wui.dag = dag
	wui.zettelBinding = zettelEnv
	return zettelEnv, nil
}

// createRenderEnv creates a new environment and populates it with all relevant data for the base template.
func (wui *WebUI) createRenderEnv(ctx context.Context, name, lang, title string, user *meta.Meta) (*sxeval.Binding, renderBinder) {
	userIsValid, userZettelURL, userIdent := wui.getUserRenderData(user)
	parentEnv, err := wui.getParentEnv(ctx)
	bind := parentEnv.MakeChildBinding(name, 128)
	rb := makeRenderBinder(bind, err)
	rb.bindString("lang", sx.MakeString(lang))
	rb.bindString("css-base-url", sx.MakeString(wui.cssBaseURL))
	rb.bindString("css-user-url", sx.MakeString(wui.cssUserURL))
	rb.bindString("title", sx.MakeString(title))
	rb.bindString("home-url", sx.MakeString(wui.homeURL))
	rb.bindString("with-auth", sx.MakeBoolean(wui.withAuth))
	rb.bindString("user-is-valid", sx.MakeBoolean(userIsValid))
	rb.bindString("user-zettel-url", sx.MakeString(userZettelURL))
	rb.bindString("user-ident", sx.MakeString(userIdent))
	rb.bindString("login-url", sx.MakeString(wui.loginURL))
	rb.bindString("logout-url", sx.MakeString(wui.logoutURL))
	rb.bindString("list-urls", wui.buildListsMenuSxn(ctx, lang))
	if wui.canRefresh(user) {
		rb.bindString("refresh-url", sx.MakeString(wui.refreshURL))
	}
	rb.bindString("new-zettel-links", wui.fetchNewTemplatesSxn(ctx, user))
	rb.bindString("search-url", sx.MakeString(wui.searchURL))
	rb.bindString("query-key-query", sx.MakeString(api.QueryKeyQuery))
	rb.bindString("query-key-seed", sx.MakeString(api.QueryKeySeed))
	rb.bindString("FOOTER", wui.calculateFooterSxn(ctx)) // TODO: use real footer
	rb.bindString("debug-mode", sx.MakeBoolean(wui.debug))
	rb.bindSymbol(symMetaHeader, sx.Nil())
	rb.bindSymbol(symDetail, sx.Nil())
	return bind, rb
}

func (wui *WebUI) getUserRenderData(user *meta.Meta) (bool, string, string) {
	if user == nil {
		return false, "", ""
	}
	return true, wui.NewURLBuilder('h').SetZid(user.Zid).String(), user.GetDefault(api.KeyUserID, "")
}

type renderBinder struct {
	err     error
	binding *sxeval.Binding
}

func makeRenderBinder(bind *sxeval.Binding, err error) renderBinder {
	return renderBinder{binding: bind, err: err}
}
func (rb *renderBinder) bindString(key string, obj sx.Object) {
	if rb.err == nil {
		rb.err = rb.binding.Bind(sx.MakeSymbol(key), obj)
	}
}
func (rb *renderBinder) bindSymbol(sym *sx.Symbol, obj sx.Object) {
	if rb.err == nil {
		rb.err = rb.binding.Bind(sym, obj)
	}
}
func (rb *renderBinder) bindKeyValue(key string, value string) {
	rb.bindString("meta-"+key, sx.MakeString(value))
	if kt := meta.Type(key); kt.IsSet {
		rb.bindString("set-meta-"+key, makeStringList(meta.ListFromValue(value)))
	}
}
func (rb *renderBinder) rebindResolved(key, defKey string) {
	if rb.err == nil {
		if obj, found := rb.binding.Resolve(sx.MakeSymbol(key)); found {
			rb.bindString(defKey, obj)
		}
	}
}

func (wui *WebUI) bindCommonZettelData(ctx context.Context, rb *renderBinder, user, m *meta.Meta, content *zettel.Content) {
	zid := m.Zid
	strZid := zid.String()
	newURLBuilder := wui.NewURLBuilder

	rb.bindString("zid", sx.MakeString(strZid))
	rb.bindString("web-url", sx.MakeString(newURLBuilder('h').SetZid(zid).String()))
	if content != nil && wui.canWrite(ctx, user, m, *content) {
		rb.bindString("edit-url", sx.MakeString(newURLBuilder('e').SetZid(zid).String()))
	}
	rb.bindString("info-url", sx.MakeString(newURLBuilder('i').SetZid(zid).String()))
	if wui.canCreate(ctx, user) {
		if content != nil && !content.IsBinary() {
			rb.bindString("copy-url", sx.MakeString(newURLBuilder('c').SetZid(zid).AppendKVQuery(queryKeyAction, valueActionCopy).String()))
		}
		rb.bindString("version-url", sx.MakeString(newURLBuilder('c').SetZid(zid).AppendKVQuery(queryKeyAction, valueActionVersion).String()))
		rb.bindString("sequel-url", sx.MakeString(newURLBuilder('c').SetZid(zid).AppendKVQuery(queryKeyAction, valueActionSequel).String()))
		rb.bindString("folge-url", sx.MakeString(newURLBuilder('c').SetZid(zid).AppendKVQuery(queryKeyAction, valueActionFolge).String()))
	}
	if wui.canDelete(ctx, user, m) {
		rb.bindString("delete-url", sx.MakeString(newURLBuilder('d').SetZid(zid).String()))
	}
	if val, found := m.Get(api.KeyUselessFiles); found {
		rb.bindString("useless", sx.Cons(sx.MakeString(val), nil))
	}
	queryContext := strZid + " " + api.ContextDirective
	rb.bindString("context-url", sx.MakeString(newURLBuilder('h').AppendQuery(queryContext).String()))
	queryContext += " " + api.FullDirective
	rb.bindString("context-full-url", sx.MakeString(newURLBuilder('h').AppendQuery(queryContext).String()))
	if wui.canRefresh(user) {
		rb.bindString("reindex-url", sx.MakeString(newURLBuilder('h').AppendQuery(
			strZid+" "+api.IdentDirective+api.ActionSeparator+api.ReIndexAction).String()))
	}

	// Ensure to have title, role, tags, and syntax included as "meta-*"
	rb.bindKeyValue(api.KeyTitle, m.GetDefault(api.KeyTitle, ""))
	rb.bindKeyValue(api.KeyRole, m.GetDefault(api.KeyRole, ""))
	rb.bindKeyValue(api.KeyTags, m.GetDefault(api.KeyTags, ""))
	rb.bindKeyValue(api.KeySyntax, m.GetDefault(api.KeySyntax, meta.DefaultSyntax))
	var metaPairs sx.ListBuilder
	for _, p := range m.ComputedPairs() {
		key, value := p.Key, p.Value
		metaPairs.Add(sx.Cons(sx.MakeString(key), sx.MakeString(value)))
		rb.bindKeyValue(key, value)
	}
	rb.bindString("metapairs", metaPairs.List())
}

func (wui *WebUI) buildListsMenuSxn(ctx context.Context, lang string) *sx.Pair {
	var zn *ast.ZettelNode
	if menuZid, err := id.Parse(wui.getConfig(ctx, nil, config.KeyListsMenuZettel)); err == nil {
		if zn, err = wui.evalZettel.Run(ctx, menuZid, ""); err != nil {
			zn = nil
		}
	}
	if zn == nil {
		ctx = box.NoEnrichContext(ctx)
		ztl, err := wui.box.GetZettel(ctx, api.ZidTOCListsMenu)
		if err != nil {
			return nil
		}
		zn = wui.evalZettel.RunZettel(ctx, ztl, "")
	}

	htmlgen := wui.getSimpleHTMLEncoder(lang)
	var lb sx.ListBuilder
	for _, ln := range collect.Order(zn) {
		lb.Add(htmlgen.nodeSxHTML(ln))
	}
	return lb.List()
}

func (wui *WebUI) fetchNewTemplatesSxn(ctx context.Context, user *meta.Meta) *sx.Pair {
	if !wui.canCreate(ctx, user) {
		return nil
	}
	ctx = box.NoEnrichContext(ctx)
	menu, err := wui.box.GetZettel(ctx, api.ZidTOCNewTemplate)
	if err != nil {
		return nil
	}
	var lb sx.ListBuilder
	for _, ln := range collect.Order(parser.ParseZettel(ctx, menu, "", wui.rtConfig)) {
		ref := ln.Ref
		if !ref.IsZettel() {
			continue
		}
		zid, err2 := id.Parse(ref.URL.Path)
		if err2 != nil {
			continue
		}
		z, err2 := wui.box.GetZettel(ctx, zid)
		if err2 != nil {
			continue
		}
		if !wui.policy.CanRead(user, z.Meta) {
			continue
		}
		text := sx.MakeString(parser.NormalizedSpacedText(z.Meta.GetTitle()))
		link := sx.MakeString(wui.NewURLBuilder('c').SetZid(zid).
			AppendKVQuery(queryKeyAction, valueActionNew).String())

		lb.Add(sx.Cons(text, link))
	}
	return lb.List()
}

func (wui *WebUI) calculateFooterSxn(ctx context.Context) *sx.Pair {
	if footerZid, err := id.Parse(wui.getConfig(ctx, nil, config.KeyFooterZettel)); err == nil {
		if zn, err2 := wui.evalZettel.Run(ctx, footerZid, ""); err2 == nil {
			htmlEnc := wui.getSimpleHTMLEncoder(wui.getConfig(ctx, zn.InhMeta, api.KeyLang)).SetUnique("footer-")
			if content, endnotes, err3 := htmlEnc.BlocksSxn(&zn.Ast); err3 == nil {
				if content != nil && endnotes != nil {
					content.LastPair().SetCdr(sx.Cons(endnotes, nil))
				}
				return content
			}
		}
	}
	return nil
}

func (wui *WebUI) getSxnTemplate(ctx context.Context, zid id.Zid, bind *sxeval.Binding) (sxeval.Expr, error) {
	if t := wui.getSxnCache(zid); t != nil {
		return t, nil
	}

	reader, err := wui.makeZettelReader(ctx, zid)
	if err != nil {
		return nil, err
	}

	objs, err := reader.ReadAll()
	if err != nil {
		wui.log.Error().Err(err).Zid(zid).Msg("reading sxn template")
		return nil, err
	}
	if len(objs) != 1 {
		return nil, fmt.Errorf("expected 1 expression in template, but got %d", len(objs))
	}
	env := sxeval.MakeExecutionEnvironment(bind)
	t, err := env.Compile(objs[0])
	if err != nil {
		return nil, err
	}

	wui.setSxnCache(zid, t)
	return t, nil
}
func (wui *WebUI) makeZettelReader(ctx context.Context, zid id.Zid) (*sxreader.Reader, error) {
	ztl, err := wui.box.GetZettel(ctx, zid)
	if err != nil {
		return nil, err
	}

	reader := sxreader.MakeReader(bytes.NewReader(ztl.Content.AsBytes()))
	return reader, nil
}

func (wui *WebUI) evalSxnTemplate(ctx context.Context, zid id.Zid, bind *sxeval.Binding) (sx.Object, error) {
	templateExpr, err := wui.getSxnTemplate(ctx, zid, bind)
	if err != nil {
		return nil, err
	}
	env := sxeval.MakeExecutionEnvironment(bind)
	return env.Run(templateExpr)
}

func (wui *WebUI) renderSxnTemplate(ctx context.Context, w http.ResponseWriter, templateID id.Zid, bind *sxeval.Binding) error {
	return wui.renderSxnTemplateStatus(ctx, w, http.StatusOK, templateID, bind)
}
func (wui *WebUI) renderSxnTemplateStatus(ctx context.Context, w http.ResponseWriter, code int, templateID id.Zid, bind *sxeval.Binding) error {
	detailObj, err := wui.evalSxnTemplate(ctx, templateID, bind)
	if err != nil {
		return err
	}
	bind.Bind(symDetail, detailObj)

	pageObj, err := wui.evalSxnTemplate(ctx, api.ZidBaseTemplate, bind)
	if err != nil {
		return err
	}
	if msg := wui.log.Debug(); msg != nil {
		// pageObj.String() can be expensive to calculate.
		msg.Str("page", pageObj.String()).Msg("render")
	}

	gen := sxhtml.NewGenerator().SetNewline()
	var sb bytes.Buffer
	_, err = gen.WriteHTML(&sb, pageObj)
	if err != nil {
		return err
	}
	wui.prepareAndWriteHeader(w, code)
	if _, err = w.Write(sb.Bytes()); err != nil {
		wui.log.Error().Err(err).Msg("Unable to write HTML via template")
	}
	return nil // No error reporting, since we do not know what happended during write to client.
}

func (wui *WebUI) reportError(ctx context.Context, w http.ResponseWriter, err error) {
	ctx = context.WithoutCancel(ctx) // Ignore any cancel / timeouts to write an error message.
	code, text := adapter.CodeMessageFromError(err)
	if code == http.StatusInternalServerError {
		wui.log.Error().Msg(err.Error())
	} else {
		wui.log.Debug().Err(err).Msg("reportError")
	}
	user := server.GetUser(ctx)
	env, rb := wui.createRenderEnv(ctx, "error", api.ValueLangEN, "Error", user)
	rb.bindString("heading", sx.MakeString(http.StatusText(code)))
	rb.bindString("message", sx.MakeString(text))
	if rb.err == nil {
		rb.err = wui.renderSxnTemplateStatus(ctx, w, code, api.ZidErrorTemplate, env)
	}
	errSx := rb.err
	if errSx == nil {
		return
	}
	wui.log.Error().Err(errSx).Msg("while rendering error message")

	// if errBind != nil, the HTTP header was not written
	wui.prepareAndWriteHeader(w, http.StatusInternalServerError)
	fmt.Fprintf(
		w,
		`<!DOCTYPE html>
<html>
<head><title>Internal server error</title></head>
<body>
<h1>Internal server error</h1>
<p>When generating error code %d with message:</p><pre>%v</pre><p>an error occured:</p><pre>%v</pre>
</body>
</html>`, code, text, errSx)
}

func makeStringList(sl []string) *sx.Pair {
	var lb sx.ListBuilder
	for _, s := range sl {
		lb.Add(sx.MakeString(s))
	}
	return lb.List()
}
