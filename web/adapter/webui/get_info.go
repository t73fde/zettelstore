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
	"context"
	"net/http"
	"slices"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/strfun"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/box"
	"zettelstore.de/z/collect"
	"zettelstore.de/z/encoder"
	"zettelstore.de/z/evaluator"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/query"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/server"
)

// MakeGetInfoHandler creates a new HTTP handler for the use case "get zettel".
func (wui *WebUI) MakeGetInfoHandler(
	ucParseZettel usecase.ParseZettel,
	ucEvaluate *usecase.Evaluate,
	ucGetZettel usecase.GetZettel,
	ucGetAllZettel usecase.GetAllZettel,
	ucQuery *usecase.Query,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()

		path := r.URL.Path[1:]
		zid, err := id.Parse(path)
		if err != nil {
			wui.reportError(ctx, w, box.ErrInvalidZid{Zid: path})
			return
		}

		zn, err := ucParseZettel.Run(ctx, zid, q.Get(meta.KeySyntax))
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}

		getTextTitle := wui.makeGetTextTitle(ctx, ucGetZettel)
		var lbMetadata sx.ListBuilder
		for _, pair := range zn.Meta.ComputedPairs() {
			key := pair.Key
			sxval := wui.writeHTMLMetaValue(key, pair.Value, getTextTitle)
			lbMetadata.Add(sx.Cons(sx.MakeString(key), sxval))
		}

		summary := collect.References(zn)
		locLinks, queryLinks, extLinks := wui.splitLocSeaExtLinks(append(summary.Links, summary.Embeds...))

		title := parser.NormalizedSpacedText(zn.InhMeta.GetTitle())
		phrase := q.Get(api.QueryKeyPhrase)
		if phrase == "" {
			phrase = title
		}
		unlinkedMeta, err := ucQuery.Run(ctx, createUnlinkedQuery(zid, phrase))
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}

		enc := wui.getSimpleHTMLEncoder(wui.getConfig(ctx, zn.InhMeta, meta.KeyLang))
		entries, _ := evaluator.QueryAction(ctx, nil, unlinkedMeta)
		bns := ucEvaluate.RunBlockNode(ctx, entries)
		unlinkedContent, _, err := enc.BlocksSxn(&bns)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		encTexts := encodingTexts()
		shadowLinks := getShadowLinks(ctx, zid, ucGetAllZettel)

		user := server.GetUser(ctx)
		env, rb := wui.createRenderEnv(ctx, "info", wui.getUserLang(ctx), title, user)
		rb.bindString("metadata", lbMetadata.List())
		rb.bindString("local-links", locLinks)
		rb.bindString("query-links", queryLinks)
		rb.bindString("ext-links", extLinks)
		rb.bindString("unlinked-content", unlinkedContent)
		rb.bindString("phrase", sx.MakeString(phrase))
		rb.bindString("query-key-phrase", sx.MakeString(api.QueryKeyPhrase))
		rb.bindString("enc-eval", wui.infoAPIMatrix(zid, false, encTexts))
		rb.bindString("enc-parsed", wui.infoAPIMatrixParsed(zid, encTexts))
		rb.bindString("shadow-links", shadowLinks)
		wui.bindCommonZettelData(ctx, &rb, user, zn.InhMeta, &zn.Content)
		if rb.err == nil {
			err = wui.renderSxnTemplate(ctx, w, id.ZidInfoTemplate, env)
		} else {
			err = rb.err
		}
		if err != nil {
			wui.reportError(ctx, w, err)
		}
	})
}

func (wui *WebUI) splitLocSeaExtLinks(links []*ast.Reference) (locLinks, queries, extLinks *sx.Pair) {
	var lbLoc, lbQueries, lbExt sx.ListBuilder
	for _, ref := range links {
		switch ref.State {
		case ast.RefStateHosted, ast.RefStateBased: // Local
			lbLoc.Add(sx.MakeString(ref.String()))

		case ast.RefStateQuery:
			lbQueries.Add(
				sx.Cons(
					sx.MakeString(ref.Value),
					sx.MakeString(wui.NewURLBuilder('h').AppendQuery(ref.Value).String())))

		case ast.RefStateExternal:
			lbExt.Add(sx.MakeString(ref.String()))
		}
	}
	return lbLoc.List(), lbQueries.List(), lbExt.List()
}

func createUnlinkedQuery(zid id.Zid, phrase string) *query.Query {
	var sb strings.Builder
	sb.Write(zid.Bytes())
	sb.WriteByte(' ')
	sb.WriteString(api.UnlinkedDirective)
	for _, word := range strfun.MakeWords(phrase) {
		sb.WriteByte(' ')
		sb.WriteString(api.PhraseDirective)
		sb.WriteByte(' ')
		sb.WriteString(word)
	}
	sb.WriteByte(' ')
	sb.WriteString(api.OrderDirective)
	sb.WriteByte(' ')
	sb.WriteString(meta.KeyID)
	return query.Parse(sb.String())
}

func encodingTexts() []string {
	encodings := encoder.GetEncodings()
	encTexts := make([]string, 0, len(encodings))
	for _, f := range encodings {
		encTexts = append(encTexts, f.String())
	}
	slices.Sort(encTexts)
	return encTexts
}

var apiParts = []string{api.PartZettel, api.PartMeta, api.PartContent}

func (wui *WebUI) infoAPIMatrix(zid id.Zid, parseOnly bool, encTexts []string) *sx.Pair {
	matrix := sx.Nil()
	u := wui.NewURLBuilder('z').SetZid(zid)
	for ip := len(apiParts) - 1; ip >= 0; ip-- {
		part := apiParts[ip]
		row := sx.Nil()
		for je := len(encTexts) - 1; je >= 0; je-- {
			enc := encTexts[je]
			if parseOnly {
				u.AppendKVQuery(api.QueryKeyParseOnly, "")
			}
			u.AppendKVQuery(api.QueryKeyPart, part)
			u.AppendKVQuery(api.QueryKeyEncoding, enc)
			row = row.Cons(sx.Cons(sx.MakeString(enc), sx.MakeString(u.String())))
			u.ClearQuery()
		}
		matrix = matrix.Cons(sx.Cons(sx.MakeString(part), row))
	}
	return matrix
}

func (wui *WebUI) infoAPIMatrixParsed(zid id.Zid, encTexts []string) *sx.Pair {
	matrix := wui.infoAPIMatrix(zid, true, encTexts)
	u := wui.NewURLBuilder('z').SetZid(zid)

	for i, row := 0, matrix; i < len(apiParts) && row != nil; row = row.Tail() {
		line, isLine := sx.GetPair(row.Car())
		if !isLine || line == nil {
			continue
		}
		last := line.LastPair()
		part := apiParts[i]
		u.AppendKVQuery(api.QueryKeyPart, part)
		last = last.AppendBang(sx.Cons(sx.MakeString("plain"), sx.MakeString(u.String())))
		u.ClearQuery()
		if i < 2 {
			u.AppendKVQuery(api.QueryKeyEncoding, api.EncodingData)
			u.AppendKVQuery(api.QueryKeyPart, part)
			last.AppendBang(sx.Cons(sx.MakeString("data"), sx.MakeString(u.String())))
			u.ClearQuery()
		}
		i++
	}
	return matrix
}

func getShadowLinks(ctx context.Context, zid id.Zid, getAllZettel usecase.GetAllZettel) *sx.Pair {
	var lb sx.ListBuilder
	if zl, err := getAllZettel.Run(ctx, zid); err == nil {
		for _, ztl := range zl {
			if boxNo, ok := ztl.Meta.Get(meta.KeyBoxNumber); ok {
				lb.Add(sx.MakeString(string(boxNo)))
			}
		}
	}
	return lb.List()
}
