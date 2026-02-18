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

package webapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sexp"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/domain"
	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter"
	"zettelstore.de/z/internal/web/content"
)

// MakeGetZettelHandler creates a new HTTP handler to return a zettel in various encodings.
func (a *WebAPI) MakeGetZettelHandler(
	getZettel usecase.GetZettel,
	parseZettel usecase.ParseZettel,
	evaluate usecase.Evaluate,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zid, err := id.Parse(r.URL.Path[1:])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()
		part := getPart(q, partContent)
		ctx := r.Context()
		switch enc, encStr := getEncoding(r, q); enc {
		case webapi.EncoderPlain:
			a.writePlainData(ctx, w, zid, part, getZettel)

		case webapi.EncoderData:
			a.writeSzData(ctx, w, zid, part, getZettel)

		default:
			var zn *domain.Zettel
			if q.Has(webapi.QueryKeyParseOnly) {
				zn, err = parseZettel.Run(ctx, zid, q.Get(meta.KeySyntax))
			} else {
				zn, err = evaluate.Run(ctx, zid, q.Get(meta.KeySyntax))
			}
			if err != nil {
				a.reportUsecaseError(w, err)
				return
			}
			a.writeEncodedZettelPart(ctx, w, zn, enc, encStr, part)
		}
	})
}

func (a *WebAPI) writePlainData(ctx context.Context, w http.ResponseWriter, zid id.Zid, part partType, getZettel usecase.GetZettel) {
	var buf bytes.Buffer
	var contentType string
	var err error

	z, err := getZettel.Run(box.NoEnrichContext(ctx), zid)
	if err != nil {
		a.reportUsecaseError(w, err)
		return
	}

	switch part {
	case partZettel:
		_, err = z.Meta.Write(&buf)
		if err == nil {
			err = buf.WriteByte('\n')
		}
		if err == nil {
			_, err = z.Content.Write(&buf)
		}

	case partMeta:
		contentType = content.PlainText
		_, err = z.Meta.Write(&buf)

	case partContent:
		contentType = content.MIMEFromSyntax(string(z.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax)))
		_, err = z.Content.Write(&buf)
	}

	if err != nil {
		a.logger.Error("Unable to store plain zettel/part in buffer", "err", err, "zid", zid)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err = writeBuffer(w, &buf, contentType); err != nil {
		a.logger.Error("write plain data", "err", err, "zid", zid)
	}
}

func (a *WebAPI) writeSzData(ctx context.Context, w http.ResponseWriter, zid id.Zid, part partType, getZettel usecase.GetZettel) {
	z, err := getZettel.Run(ctx, zid)
	if err != nil {
		a.reportUsecaseError(w, err)
		return
	}
	var obj sx.Object
	switch part {
	case partZettel:
		zContent, zEncoding := z.Content.Encode()
		obj = sexp.EncodeZettel(webapi.ZettelData{
			Meta:     z.Meta.Map(),
			Rights:   a.getRights(ctx, z.Meta),
			Encoding: zEncoding,
			Content:  zContent,
		})

	case partMeta:
		obj = sexp.EncodeMetaRights(webapi.MetaRights{
			Meta:   z.Meta.Map(),
			Rights: a.getRights(ctx, z.Meta),
		})
	}
	if err = a.writeObject(w, zid, obj); err != nil {
		a.logger.Error("write sx data", "err", err, "zid", zid)
	}
}

func (a *WebAPI) writeEncodedZettelPart(
	ctx context.Context,
	w http.ResponseWriter, zn *domain.Zettel,
	enc webapi.EncodingEnum, encStr string, part partType,
) {
	encdr := encoder.Create(
		enc,
		&encoder.CreateParameter{
			Lang: a.rtConfig.Get(ctx, zn.InhMeta, meta.KeyLang),
		})
	if encdr == nil {
		adapter.BadRequest(w, fmt.Sprintf("Zettel %q not available in encoding %q", zn.Meta.Zid, encStr))
		return
	}
	var err error
	var buf bytes.Buffer
	switch part {
	case partZettel:
		err = encdr.WriteZettel(&buf, zn)
	case partMeta:
		err = encdr.WriteMeta(&buf, zn.InhMeta)
	case partContent:
		err = encdr.WriteSz(&buf, zn.Blocks)
	}
	if err != nil {
		a.logger.Error("Unable to store data in buffer", "err", err, "zid", zn.Zid)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if buf.Len() == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err = writeBuffer(w, &buf, content.MIMEFromEncoding(enc)); err != nil {
		a.logger.Error("Write encoded zettel", "err", err, "zid", zn.Zid)
	}
}
