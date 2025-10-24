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
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/auth/user"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/evaluator"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/usecase"
)

// MakeGetInfoHandler creates a new HTTP handler for the use case "get zettel".
func (wui *WebUI) MakeGetInfoHandler(
	ucParseZettel usecase.ParseZettel,
	ucGetReferences usecase.GetReferences,
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
		for key, val := range zn.Meta.Computed() {
			sxval := wui.writeHTMLMetaValue(key, val, getTextTitle)
			lbMetadata.Add(sx.Cons(sx.MakeString(key), sxval))
		}

		locLinks, extLinks, queryLinks := wui.getLocalExtQueryLinks(ucGetReferences, zn.Blocks)

		title := sz.NormalizedSpacedText(zn.InhMeta.GetTitle())
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
		blocks := ucEvaluate.RunBlockNode(ctx, entries)
		unlinkedContent, _, err := enc.BlocksSxn(blocks)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		encTexts := encodingTexts()
		shadowLinks := getShadowLinks(ctx, zid, zn.InhMeta.GetDefault(meta.KeyBoxNumber, ""), ucGetAllZettel)

		user := user.GetCurrentUser(ctx)
		env, rb := wui.createRenderEnvironment(ctx, "info", wui.getUserLang(ctx), title, user)
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

func (wui *WebUI) getLocalExtQueryLinks(ucGetReferences usecase.GetReferences, blocks *sx.Pair) (locLinks, extLinks, queries *sx.Pair) {
	locRefs, extRefs, queryRefs := ucGetReferences.RunByState(blocks)
	var lbLoc, lbQueries, lbExt sx.ListBuilder
	for ref := range locRefs.Pairs() {
		_, value := zsx.GetReference(ref.Head())
		lbLoc.Add(sx.MakeString(value))
	}
	for ref := range extRefs.Pairs() {
		_, value := zsx.GetReference(ref.Head())
		lbExt.Add(sx.MakeString(value))
	}
	for ref := range queryRefs.Pairs() {
		_, value := zsx.GetReference(ref.Head())
		lbQueries.Add(
			sx.Cons(
				sx.MakeString(value),
				sx.MakeString(wui.NewURLBuilder('h').AppendQuery(value).String())))
	}
	return lbLoc.List(), lbExt.List(), lbQueries.List()
}

func createUnlinkedQuery(zid id.Zid, phrase string) *query.Query {
	var sb strings.Builder
	sb.Write(zid.Bytes())
	sb.WriteByte(' ')
	sb.WriteString(api.UnlinkedDirective)
	for _, word := range zerostrings.MakeWords(phrase) {
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

func getShadowLinks(ctx context.Context, zid id.Zid, boxNumber meta.Value, getAllZettel usecase.GetAllZettel) *sx.Pair {
	var lb sx.ListBuilder
	if zl, err := getAllZettel.Run(ctx, zid); err == nil {
		for _, ztl := range zl {
			if boxNo, ok := ztl.Meta.Get(meta.KeyBoxNumber); ok && boxNo != boxNumber {
				lb.Add(sx.MakeString(string(boxNo)))
			}
		}
	}
	return lb.List()
}
