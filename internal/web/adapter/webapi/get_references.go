//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

package webapi

import (
	"bytes"
	"iter"
	"net/http"

	"t73f.de/r/sx"
	zeroiter "t73f.de/r/zero/iter"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/webapi"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/content"
)

// MakeGetReferencesHandler creates a new HTTP handler to return various lists
// of zettel references.
func (a *WebAPI) MakeGetReferencesHandler(
	ucParseZettel usecase.ParseZettel,
	ucGetReferences usecase.GetReferences,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zid, err := id.Parse(r.URL.Path[1:])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ctx := r.Context()
		zn, err := ucParseZettel.Run(ctx, zid, "")
		if err != nil {
			a.reportUsecaseError(w, err)
			return
		}

		var seq iter.Seq[string]
		q := r.URL.Query()
		switch getPart(q, partZettel) {
		case partZettel:
			seq = zeroiter.CatSeq(
				ucGetReferences.RunByMeta(zn.InhMeta),
				getExternalURLs(zn.Blocks, ucGetReferences),
			)
		case partMeta:
			seq = ucGetReferences.RunByMeta(zn.InhMeta)
		case partContent:
			seq = getExternalURLs(zn.Blocks, ucGetReferences)
		}

		enc, _ := getEncoding(r, q)
		if enc == webapi.EncoderData {
			var lb sx.ListBuilder
			lb.Collect(zeroiter.MapSeq(seq, func(s string) sx.Object { return sx.MakeString(s) }))
			if err = a.writeObject(w, zid, lb.List()); err != nil {
				a.logger.Error("write sx data", "err", err, "zid", zid)
			}
			return
		}

		var buf bytes.Buffer
		for s := range seq {
			buf.WriteString(s)
			buf.WriteByte('\n')
		}
		if err = writeBuffer(w, &buf, content.PlainText); err != nil {
			a.logger.Error("write plain data", "err", err, "zid", zid)
		}
	})
}

func getExternalURLs(blocks *sx.Pair, ucGetReferences usecase.GetReferences) iter.Seq[string] {
	return zeroiter.MapSeq(
		ucGetReferences.RunByExternal(blocks),
		func(ref *sx.Pair) string {
			_, val := zsx.GetReference(ref)
			return val
		},
	)
}
