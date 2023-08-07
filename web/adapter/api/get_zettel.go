//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"zettelstore.de/client.fossil/api"
	"zettelstore.de/client.fossil/sexp"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/box"
	"zettelstore.de/z/encoder"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/content"
	"zettelstore.de/z/zettel/id"
)

// MakeGetZettelHandler creates a new HTTP handler to return a zettel in various encodings.
func (a *API) MakeGetZettelHandler(getZettel usecase.GetZettel, parseZettel usecase.ParseZettel, evaluate usecase.Evaluate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		zid, err := id.Parse(r.URL.Path[1:])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()
		part := getPart(q, partContent)
		ctx := r.Context()
		switch enc, encStr := getEncoding(r, q); enc {
		case api.EncoderPlain:
			a.writePlainData(w, ctx, zid, part, getZettel)

		case api.EncoderData:
			a.writeSzData(w, ctx, zid, part, getZettel)

		default:
			var zn *ast.ZettelNode
			var em func(value string) ast.InlineSlice
			if q.Has(api.QueryKeyParseOnly) {
				zn, err = parseZettel.Run(ctx, zid, q.Get(api.KeySyntax))
				em = parser.ParseMetadata
			} else {
				zn, err = evaluate.Run(ctx, zid, q.Get(api.KeySyntax))
				em = func(value string) ast.InlineSlice {
					return evaluate.RunMetadata(ctx, value)
				}
			}
			if err != nil {
				a.reportUsecaseError(w, err)
				return
			}
			a.writeEncodedZettelPart(w, zn, em, enc, encStr, part)
		}
	}
}

func (a *API) writePlainData(w http.ResponseWriter, ctx context.Context, zid id.Zid, part partType, getZettel usecase.GetZettel) {
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
		contentType = content.MIMEFromSyntax(z.Meta.GetDefault(api.KeySyntax, ""))
		_, err = z.Content.Write(&buf)
	}

	if err != nil {
		a.log.Fatal().Err(err).Zid(zid).Msg("Unable to store plain zettel/part in buffer")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	err = writeBuffer(w, &buf, contentType)
	a.log.IfErr(err).Zid(zid).Msg("Write Plain data")
}

func (a *API) writeSzData(w http.ResponseWriter, ctx context.Context, zid id.Zid, part partType, getZettel usecase.GetZettel) {
	z, err := getZettel.Run(ctx, zid)
	if err != nil {
		a.reportUsecaseError(w, err)
		return
	}
	var obj sx.Object
	switch part {
	case partZettel:
		zContent, zEncoding := z.Content.Encode()
		obj = sexp.EncodeZettel(api.ZettelData{
			Meta:     z.Meta.Map(),
			Rights:   a.getRights(ctx, z.Meta),
			Encoding: zEncoding,
			Content:  zContent,
		})

	case partMeta:
		obj = sexp.EncodeMetaRights(api.MetaRights{
			Meta:   z.Meta.Map(),
			Rights: a.getRights(ctx, z.Meta),
		})
	}
	err = a.writeObject(w, zid, obj)
	a.log.IfErr(err).Zid(zid).Msg("write sx data")
}

func (a *API) writeEncodedZettelPart(
	w http.ResponseWriter, zn *ast.ZettelNode,
	evalMeta encoder.EvalMetaFunc,
	enc api.EncodingEnum, encStr string, part partType,
) {
	encdr := encoder.Create(enc)
	if encdr == nil {
		adapter.BadRequest(w, fmt.Sprintf("Zettel %q not available in encoding %q", zn.Meta.Zid, encStr))
		return
	}
	var err error
	var buf bytes.Buffer
	switch part {
	case partZettel:
		_, err = encdr.WriteZettel(&buf, zn, evalMeta)
	case partMeta:
		_, err = encdr.WriteMeta(&buf, zn.InhMeta, evalMeta)
	case partContent:
		_, err = encdr.WriteContent(&buf, zn)
	}
	if err != nil {
		a.log.Fatal().Err(err).Zid(zn.Zid).Msg("Unable to store data in buffer")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if buf.Len() == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	err = writeBuffer(w, &buf, content.MIMEFromEncoding(enc))
	a.log.IfErr(err).Zid(zn.Zid).Msg("Write Encoded Zettel")
}
