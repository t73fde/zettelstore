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
	"io"
	"net/http"
	"net/url"

	"t73f.de/r/sx/sxreader"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sexp"
	"t73f.de/r/zsc/webapi"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/zettel"
)

// getEncoding returns the data encoding selected by the caller.
func getEncoding(r *http.Request, q url.Values) (webapi.EncodingEnum, string) {
	encoding := q.Get(webapi.QueryKeyEncoding)
	if encoding != "" {
		return webapi.Encoder(encoding), encoding
	}
	if enc, ok := getOneEncoding(r, webapi.HeaderAccept); ok {
		return webapi.Encoder(enc), enc
	}
	if enc, ok := getOneEncoding(r, webapi.HeaderContentType); ok {
		return webapi.Encoder(enc), enc
	}
	return webapi.EncoderPlain, webapi.EncoderPlain.String()
}

func getOneEncoding(r *http.Request, key string) (string, bool) {
	if values, ok := r.Header[key]; ok {
		for _, value := range values {
			if enc, ok2 := contentType2encoding(value); ok2 {
				return enc, true
			}
		}
	}
	return "", false
}

var mapCT2encoding = map[string]string{
	"text/html": webapi.EncodingHTML,
}

func contentType2encoding(contentType string) (string, bool) {
	// TODO: only check before first ';'
	enc, ok := mapCT2encoding[contentType]
	return enc, ok
}

type partType int

const (
	_ partType = iota
	partMeta
	partContent
	partZettel
)

var partMap = map[string]partType{
	webapi.PartMeta:    partMeta,
	webapi.PartContent: partContent,
	webapi.PartZettel:  partZettel,
}

func getPart(q url.Values, defPart partType) partType {
	if part, ok := partMap[q.Get(webapi.QueryKeyPart)]; ok {
		return part
	}
	return defPart
}

func buildZettelFromPlainData(r *http.Request, zid id.Zid) (zettel.Zettel, error) {
	defer func() { _ = r.Body.Close() }()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return zettel.Zettel{}, err
	}
	inp := input.NewInput(b)
	m := meta.NewFromInput(zid, inp)
	return zettel.Zettel{
		Meta:    m,
		Content: zettel.NewContent(inp.Src[inp.Pos:]),
	}, nil
}

func buildZettelFromData(r *http.Request, zid id.Zid) (zettel.Zettel, error) {
	defer func() { _ = r.Body.Close() }()
	rdr := sxreader.MakeReader(r.Body)
	obj, err := rdr.Read()
	if err != nil {
		return zettel.Zettel{}, err
	}
	zd, err := sexp.ParseZettel(obj)
	if err != nil {
		return zettel.Zettel{}, err
	}

	m := meta.New(zid)
	for k, v := range zd.Meta {
		if !meta.IsComputed(k) {
			m.Set(meta.RemoveNonGraphic(k), meta.Value(meta.RemoveNonGraphic(v)))
		}
	}

	var content zettel.Content
	if err = content.SetDecoded(zd.Content, zd.Encoding); err != nil {
		return zettel.Zettel{}, err
	}

	return zettel.Zettel{
		Meta:    m,
		Content: content,
	}, nil
}
