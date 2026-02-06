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

package webapi

import (
	"net/http"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter"
	"zettelstore.de/z/internal/web/content"
	"zettelstore.de/z/internal/zettel"
)

// MakePostCreateZettelHandler creates a new HTTP handler to store content of
// an existing zettel.
func (a *WebAPI) MakePostCreateZettelHandler(createZettel *usecase.CreateZettel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		enc, encStr := getEncoding(r, q)
		var zettel zettel.Zettel
		var err error
		switch enc {
		case webapi.EncoderPlain:
			zettel, err = buildZettelFromPlainData(r, id.Invalid)
		case webapi.EncoderData:
			zettel, err = buildZettelFromData(r, id.Invalid)
		default:
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		if err != nil {
			a.reportUsecaseError(w, adapter.NewErrBadRequest(err.Error()))
			return
		}

		ctx := r.Context()
		newZid, err := createZettel.Run(ctx, zettel)
		if err != nil {
			a.reportUsecaseError(w, err)
			return
		}

		var result []byte
		var contentType string
		location := a.NewURLBuilder('z').SetZid(newZid)
		switch enc {
		case webapi.EncoderPlain:
			result = newZid.Bytes()
			contentType = content.PlainText
		case webapi.EncoderData:
			result = []byte(sx.Int64(newZid).String())
			contentType = content.SXPF
		default:
			panic(encStr)
		}

		h := adapter.PrepareHeader(w, contentType)
		h.Set(webapi.HeaderLocation, location.String())
		w.WriteHeader(http.StatusCreated)
		if _, err = w.Write(result); err != nil {
			a.logger.Error("Create zettel", "err", err, "zid", newZid)
		}
	})
}
