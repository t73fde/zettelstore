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
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxbuiltins"
	"t73f.de/r/sx/sxeval"
	"t73f.de/r/sx/sxreader"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/collect"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/web/adapter"
	"zettelstore.de/z/internal/zettel"
)

func (wui *WebUI) createRootBinding() *sxeval.Binding {
	root := sxeval.MakeRootBinding(len(specials) + len(builtins) + 32)
	_ = sxbuiltins.LoadPrelude(root)
	_ = sxeval.BindSpecials(root, specials...)
	_ = sxeval.BindBuiltins(root, builtins...)
	_ = sxeval.BindBuiltins(root,
		&sxeval.Builtin{
			Name:     "url-to-html",
			MinArity: 1,
			MaxArity: 1,
			TestPure: sxeval.AssertPure,
			Fn1: func(_ *sxeval.Environment, arg sx.Object, _ *sxeval.Frame) (sx.Object, error) {
				text, err := sxbuiltins.GetString(arg, 0)
				if err != nil {
					return nil, err
				}
				return wui.url2html(text), nil
			},
		},
		&sxeval.Builtin{
			Name:     "zid-content-path",
			MinArity: 1,
			MaxArity: 1,
			TestPure: sxeval.AssertPure,
			Fn1: func(_ *sxeval.Environment, arg sx.Object, _ *sxeval.Frame) (sx.Object, error) {
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
		},
		&sxeval.Builtin{
			Name:     "query->url",
			MinArity: 1,
			MaxArity: 1,
			TestPure: sxeval.AssertPure,
			Fn1: func(_ *sxeval.Environment, arg sx.Object, _ *sxeval.Frame) (sx.Object, error) {
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
		&sxbuiltins.SetXS,                      // set!
		&sxbuiltins.IfS,                        // if
		&sxbuiltins.BeginS,                     // begin
		&sxbuiltins.DefMacroS,                  // defmacro
		&sxbuiltins.LetS, &sxbuiltins.LetStarS, // let, let*
		&sxbuiltins.AndS, &sxbuiltins.OrS, // and, or
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
		&sxbuiltins.List,          // list
		&sxbuiltins.Append,        // append
		&sxbuiltins.Assoc,         // assoc
		&sxbuiltins.Map,           // map
		&sxbuiltins.Apply,         // apply
		&sxbuiltins.Concat,        // concat
		&sxbuiltins.SymbolBoundP,  // symbol-bound?
		&sxbuiltins.DefinedP,      // defined?
		&sxbuiltins.CurrentFrame,  // current-frame
		&sxbuiltins.ResolveSymbol, // resolve-symbol
	}
)

func (wui *WebUI) url2html(text sx.String) sx.Object {
	if u, errURL := url.Parse(text.GetValue()); errURL == nil {
		if us := u.String(); us != "" {
			return sx.MakeList(
				shtml.SymA,
				sx.MakeList(
					sx.Cons(shtml.SymAttrHref, sx.MakeString(us)),
					sx.Cons(shtml.SymAttrTarget, sx.MakeString("_blank")),
					sx.Cons(shtml.SymAttrRel, sx.MakeString("external noreferrer")),
				),
				text)
		}
	}
	return text
}

func (wui *WebUI) getParentBinding(ctx context.Context) (*sxeval.Binding, error) {
	wui.mxZettelBinding.Lock()
	defer wui.mxZettelBinding.Unlock()
	if parentBind := wui.zettelBinding; parentBind != nil {
		return parentBind, nil
	}
	dag, zettelBind, err := wui.loadAllSxnCodeZettel(ctx)
	if err != nil {
		wui.logger.Error("loading zettel sxn", "err", err)
		return nil, err
	}
	wui.dag = dag
	wui.zettelBinding = zettelBind
	return zettelBind, nil
}

// createRenderEnvironment creates a new environment and populates it with all
// relevant data for the base template.
func (wui *WebUI) createRenderEnvironment(ctx context.Context, name, lang, title string, user *meta.Meta) (*sxeval.Environment, renderBinder) {
	userIsValid, userZettelURL, userIdent := wui.getUserRenderData(user)
	parentBind, err := wui.getParentBinding(ctx)
	bind := parentBind.MakeChildBinding(name, 128)
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

	nestH := sxeval.MakeNestingLimitHandler(wui.sxMaxNesting, sxeval.DefaultHandler{})
	var handler sxeval.ComputeHandler = nestH
	if logger := wui.logger; logger.Handler().Enabled(context.Background(), logging.LevelTrace) {
		stepsH := sxeval.MakeStepsHandler(nestH)
		handler = &computeLogHandler{logger: logger, nest: nestH, next: stepsH}
	}
	env := sxeval.MakeEnvironment(bind).SetComputeHandler(handler)
	return env, rb
}

func (wui *WebUI) getUserRenderData(user *meta.Meta) (bool, string, string) {
	if user == nil {
		return false, "", ""
	}
	return true, wui.NewURLBuilder('h').SetZid(user.Zid).String(), string(user.GetDefault(meta.KeyUserID, ""))
}

type computeLogHandler struct {
	logger *slog.Logger
	nest   *sxeval.NestingLimitHandler
	next   *sxeval.StepsHandler
}

func (clh *computeLogHandler) Compute(env *sxeval.Environment, expr sxeval.Expr, frame *sxeval.Frame) (sx.Object, error) {
	fname := "nil"
	if frame != nil {
		fname = frame.Name()
	}
	curNesting, _ := clh.nest.Nesting()
	var sb strings.Builder
	_, _ = expr.Print(&sb)
	logging.LogTrace(clh.logger, "compute",
		slog.String("frame", fname),
		slog.Int("steps", clh.next.Steps),
		slog.Int("level", curNesting),
		slog.String("expr", sb.String()))
	obj, err := clh.next.Compute(env, expr, frame)
	if err == nil {
		logging.LogTrace(clh.logger, "result ",
			slog.String("frame", fname),
			slog.Int("steps", clh.next.Steps),
			slog.Int("level", curNesting),
			slog.Any("object", obj))
	}
	return obj, err
}

func (clh *computeLogHandler) Reset() { clh.next.Reset() }

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
func (rb *renderBinder) bindKeyValue(key string, value meta.Value) {
	rb.bindString("meta-"+key, sx.MakeString(string(value)))
	if kt := meta.Type(key); kt.IsSet {
		rb.bindString("set-meta-"+key, makeStringList(value.AsSlice()))
	}
}
func (rb *renderBinder) rebindResolved(key, extraKey string) {
	if rb.err == nil {
		sym := sx.MakeSymbol(key)
		for curr := rb.binding; curr != nil; curr = curr.Parent() {
			if obj, found := curr.Lookup(sym); found {
				rb.bindString(extraKey, obj)
				return
			}
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
			wui.bindCreateURL(rb, zid, "copy-url", valueActionCopy)
		}
		wui.bindCreateURL(rb, zid, "sequel-url", valueActionSequel)
		wui.bindCreateURL(rb, zid, "folge-url", valueActionFolge)
	}
	if wui.canDelete(ctx, user, m) {
		rb.bindString("delete-url", sx.MakeString(newURLBuilder('d').SetZid(zid).String()))
	}
	if val, found := m.Get(meta.KeyUselessFiles); found {
		rb.bindString("useless", sx.Cons(sx.MakeString(string(val)), nil))
	}
	wui.bindQueryURL(rb, strZid, "context-url", api.ContextDirective)
	wui.bindQueryURL(rb, strZid, "context-full-url", api.ContextDirective+" "+api.FullDirective)
	canFolgeQuery := m.Has(meta.KeyPrecursor) || m.Has(meta.KeyFolge)
	canSequelQuery := m.Has(meta.KeyPrequel) || m.Has(meta.KeySequel)
	if canFolgeQuery || canSequelQuery {
		wui.bindQueryURL(rb, strZid, "thread-query-url", api.ThreadDirective)
		if canFolgeQuery {
			wui.bindQueryURL(rb, strZid, "folge-query-url", api.FolgeDirective)
		}
		if canSequelQuery {
			wui.bindQueryURL(rb, strZid, "sequel-query-url", api.SequelDirective)
		}
	}

	if wui.canRefresh(user) {
		rb.bindString("reindex-url", sx.MakeString(newURLBuilder('h').AppendQuery(
			strZid+" "+api.IdentDirective+api.ActionSeparator+api.ReIndexAction).String()))
	}

	// Ensure to have title, role, tags, and syntax included as "meta-*"
	rb.bindKeyValue(meta.KeyTitle, m.GetDefault(meta.KeyTitle, ""))
	rb.bindKeyValue(meta.KeyRole, m.GetDefault(meta.KeyRole, ""))
	rb.bindKeyValue(meta.KeyTags, m.GetDefault(meta.KeyTags, ""))
	rb.bindKeyValue(meta.KeySyntax, m.GetDefault(meta.KeySyntax, meta.DefaultSyntax))
	var metaPairs sx.ListBuilder
	for key, val := range m.Computed() {
		metaPairs.Add(sx.Cons(sx.MakeString(key), sx.MakeString(string(val))))
		rb.bindKeyValue(key, val)
	}
	rb.bindString("metapairs", metaPairs.List())
}
func (wui *WebUI) bindCreateURL(rb *renderBinder, zid id.Zid, symName, actionName string) {
	rb.bindString(symName,
		sx.MakeString(wui.NewURLBuilder('c').SetZid(zid).AppendKVQuery(queryKeyAction, actionName).String()))
}
func (wui *WebUI) bindQueryURL(rb *renderBinder, strZid, symName, directive string) {
	rb.bindString(symName,
		sx.MakeString(wui.NewURLBuilder('h').AppendQuery(strZid+" "+directive+" "+api.DirectedDirective).String()))
}

func (wui *WebUI) buildListsMenuSxn(ctx context.Context, lang string) *sx.Pair {
	var zn *ast.Zettel
	if menuZid, err := id.Parse(wui.getConfig(ctx, nil, config.KeyListsMenuZettel)); err == nil {
		if zn, err = wui.evalZettel.Run(ctx, menuZid, ""); err != nil {
			zn = nil
		}
	}
	if zn == nil {
		ctx = box.NoEnrichContext(ctx)
		ztl, err := wui.box.GetZettel(ctx, id.ZidTOCListsMenu)
		if err != nil {
			return nil
		}
		zn = wui.evalZettel.RunZettel(ctx, ztl, "")
	}

	htmlgen := wui.getSimpleHTMLEncoder(lang)
	var lb sx.ListBuilder
	for ln := range collect.Order(zn.Blocks).Pairs() {
		lb.Add(htmlgen.szToSxHTML(ln.Head()))
	}
	return lb.List()
}

func (wui *WebUI) fetchNewTemplatesSxn(ctx context.Context, user *meta.Meta) *sx.Pair {
	if !wui.canCreate(ctx, user) {
		return nil
	}
	ctx = box.NoEnrichContext(ctx)
	menu, err := wui.box.GetZettel(ctx, id.ZidTOCNewTemplate)
	if err != nil {
		return nil
	}
	var lb sx.ListBuilder
	zn := parser.ParseZettel(ctx, menu, "", wui.rtConfig)
	for ln := range collect.Order(zn.Blocks).Pairs() {
		_, ref, _ := zsx.GetLink(ln.Head())
		sym, val := zsx.GetReference(ref)
		if !sz.SymRefStateZettel.IsEqualSymbol(sym) {
			continue
		}
		zid, err2 := id.Parse(val)
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
		text := sx.MakeString(sz.NormalizedSpacedText(z.Meta.GetTitle()))
		link := sx.MakeString(wui.NewURLBuilder('c').SetZid(zid).
			AppendKVQuery(queryKeyAction, valueActionNew).String())

		lb.Add(sx.Cons(text, link))
	}
	return lb.List()
}

func (wui *WebUI) calculateFooterSxn(ctx context.Context) *sx.Pair {
	if footerZid, err := id.Parse(wui.getConfig(ctx, nil, config.KeyFooterZettel)); err == nil {
		if zn, err2 := wui.evalZettel.Run(ctx, footerZid, ""); err2 == nil {
			htmlEnc := wui.getSimpleHTMLEncoder(wui.getConfig(ctx, zn.InhMeta, meta.KeyLang)).SetUnique("footer-")
			if content, endnotes, err3 := htmlEnc.BlocksSxn(zn.Blocks); err3 == nil {
				if content != nil && endnotes != nil {
					content.LastPair().SetCdr(sx.Cons(endnotes, nil))
				}
				return content
			}
		}
	}
	return nil
}

func (wui *WebUI) getSxnTemplate(ctx context.Context, zid id.Zid, env *sxeval.Environment) (sxeval.Expr, error) {
	if t := wui.getSxnCache(zid); t != nil {
		return t, nil
	}

	reader, err := wui.makeZettelReader(ctx, zid)
	if err != nil {
		return nil, err
	}

	objs, err := reader.ReadAll()
	if err != nil {
		wui.logger.Error("reading sxn template", "err", err, "zid", zid)
		return nil, err
	}
	if len(objs) != 1 {
		return nil, fmt.Errorf("expected 1 expression in template, but got %d", len(objs))
	}
	t, err := env.Parse(objs[0], nil)
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

func (wui *WebUI) evalSxnTemplate(ctx context.Context, zid id.Zid, env *sxeval.Environment) (sx.Object, error) {
	templateExpr, err := wui.getSxnTemplate(ctx, zid, env)
	if err != nil {
		return nil, err
	}
	return env.Run(templateExpr, nil)
}

func (wui *WebUI) renderSxnTemplate(ctx context.Context, w http.ResponseWriter, templateID id.Zid, env *sxeval.Environment) error {
	return wui.renderSxnTemplateStatus(ctx, w, http.StatusOK, templateID, env)
}
func (wui *WebUI) renderSxnTemplateStatus(ctx context.Context, w http.ResponseWriter, code int, templateID id.Zid, env *sxeval.Environment) error {
	detailObj, err := wui.evalSxnTemplate(ctx, templateID, env)
	if err != nil {
		return err
	}
	if err = env.BindGlobal(symDetail, detailObj); err != nil {
		return err
	}

	pageObj, err := wui.evalSxnTemplate(ctx, id.ZidBaseTemplate, env)
	if err != nil {
		return err
	}
	wui.logger.Debug("render", "page", pageObj)

	gen := sxhtml.NewGenerator().SetNewline()
	var sb bytes.Buffer
	if err = gen.WriteHTML(&sb, pageObj); err != nil {
		return err
	}
	wui.prepareAndWriteHeader(w, code)
	if _, err = w.Write(sb.Bytes()); err != nil {
		wui.logger.Error("Unable to write HTML via template", "err", err)
	}
	return nil // No error reporting, since we do not know what happened during write to client.
}

func (wui *WebUI) reportError(ctx context.Context, w http.ResponseWriter, err error) {
	ctx = context.WithoutCancel(ctx) // Ignore any cancel / timeouts to write an error message.
	code, text := adapter.CodeMessageFromError(err)
	if code == http.StatusInternalServerError {
		wui.logger.Error(err.Error())
	} else {
		wui.logger.Debug("reportError", "err", err)
	}
	user := auth.GetCurrentUser(ctx)
	bind, rb := wui.createRenderEnvironment(ctx, "error", meta.ValueLangEN, "Error", user)
	rb.bindString("heading", sx.MakeString(http.StatusText(code)))
	rb.bindString("message", sx.MakeString(text))
	if rb.err == nil {
		rb.err = wui.renderSxnTemplateStatus(ctx, w, code, id.ZidErrorTemplate, bind)
	}
	errSx := rb.err
	if errSx == nil {
		return
	}
	wui.logger.Error("while rendering error message", "err", errSx)

	// if errBind != nil, the HTTP header was not written
	wui.prepareAndWriteHeader(w, http.StatusInternalServerError)
	_, _ = fmt.Fprintf(
		w,
		`<!DOCTYPE html>
<html>
<head><title>Internal server error</title></head>
<body>
<h1>Internal server error</h1>
<p>When generating error code %d with message:</p><pre>%v</pre><p>an error occurred:</p><pre>%v</pre>
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
