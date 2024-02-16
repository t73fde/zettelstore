//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"zettelstore.de/client.fossil/api"
	"zettelstore.de/client.fossil/sexp"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/z/query"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/content"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// MakeQueryHandler creates a new HTTP handler to perform a query.
func (a *API) MakeQueryHandler(queryMeta *usecase.Query, tagZettel *usecase.TagZettel, roleZettel *usecase.RoleZettel, reIndex *usecase.ReIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		urlQuery := r.URL.Query()
		if a.handleTagZettel(w, r, tagZettel, urlQuery) || a.handleRoleZettel(w, r, roleZettel, urlQuery) {
			return
		}

		sq := adapter.GetQuery(urlQuery)
		metaSeq, err := queryMeta.Run(ctx, sq)
		if err != nil {
			a.reportUsecaseError(w, err)
			return
		}

		actions, err := adapter.TryReIndex(ctx, sq.Actions(), metaSeq, reIndex)
		if err != nil {
			a.reportUsecaseError(w, err)
			return
		}
		if len(actions) > 0 {
			if len(metaSeq) > 0 {
				for _, act := range actions {
					if act == api.RedirectAction {
						zid := metaSeq[0].Zid
						ub := a.NewURLBuilder('z').SetZid(zid.ZettelID())
						a.redirectFound(w, r, ub, zid)
						return
					}
				}
			}
		}

		var encoder zettelEncoder
		var contentType string
		switch enc, _ := getEncoding(r, urlQuery); enc {
		case api.EncoderPlain:
			encoder = &plainZettelEncoder{}
			contentType = content.PlainText

		case api.EncoderData:
			encoder = &dataZettelEncoder{
				sq:        sq,
				getRights: func(m *meta.Meta) api.ZettelRights { return a.getRights(ctx, m) },
			}
			contentType = content.SXPF

		default:
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		var buf bytes.Buffer
		err = queryAction(&buf, encoder, metaSeq, actions)
		if err != nil {
			a.log.Error().Err(err).Str("query", sq.String()).Msg("execute query action")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		if err = writeBuffer(w, &buf, contentType); err != nil {
			a.log.Error().Err(err).Msg("write result buffer")
		}
	}
}
func queryAction(w io.Writer, enc zettelEncoder, ml []*meta.Meta, actions []string) error {
	min, max := -1, -1
	if len(actions) > 0 {
		acts := make([]string, 0, len(actions))
		for _, act := range actions {
			if strings.HasPrefix(act, api.MinAction) {
				if num, err := strconv.Atoi(act[3:]); err == nil && num > 0 {
					min = num
					continue
				}
			}
			if strings.HasPrefix(act, api.MaxAction) {
				if num, err := strconv.Atoi(act[3:]); err == nil && num > 0 {
					max = num
					continue
				}
			}
			acts = append(acts, act)
		}
		for _, act := range acts {
			if act == api.KeysAction {
				return encodeKeysArrangement(w, enc, ml, act)
			}
			switch key := strings.ToLower(act); meta.Type(key) {
			case meta.TypeWord, meta.TypeTagSet:
				return encodeMetaKeyArrangement(w, enc, ml, key, min, max)
			}
		}
	}
	return enc.writeMetaList(w, ml)
}

func encodeKeysArrangement(w io.Writer, enc zettelEncoder, ml []*meta.Meta, act string) error {
	arr := make(meta.Arrangement, 128)
	for _, m := range ml {
		for k := range m.Map() {
			arr[k] = append(arr[k], m)
		}
	}
	return enc.writeArrangement(w, act, arr)
}

func encodeMetaKeyArrangement(w io.Writer, enc zettelEncoder, ml []*meta.Meta, key string, min, max int) error {
	arr0 := meta.CreateArrangement(ml, key)
	arr := make(meta.Arrangement, len(arr0))
	for k0, ml0 := range arr0 {
		if len(ml0) < min || (max > 0 && len(ml0) > max) {
			continue
		}
		arr[k0] = ml0
	}
	return enc.writeArrangement(w, key, arr)
}

type zettelEncoder interface {
	writeMetaList(w io.Writer, ml []*meta.Meta) error
	writeArrangement(w io.Writer, act string, arr meta.Arrangement) error
}

type plainZettelEncoder struct{}

func (*plainZettelEncoder) writeMetaList(w io.Writer, ml []*meta.Meta) error {
	for _, m := range ml {
		_, err := fmt.Fprintln(w, m.Zid.String(), m.GetTitle())
		if err != nil {
			return err
		}
	}
	return nil
}
func (*plainZettelEncoder) writeArrangement(w io.Writer, _ string, arr meta.Arrangement) error {
	for key, ml := range arr {
		_, err := io.WriteString(w, key)
		if err != nil {
			return err
		}
		for i, m := range ml {
			if i == 0 {
				_, err = io.WriteString(w, "\t")
			} else {
				_, err = io.WriteString(w, " ")
			}
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, m.Zid.String())
			if err != nil {
				return err
			}
		}
		_, err = io.WriteString(w, "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

type dataZettelEncoder struct {
	sq        *query.Query
	getRights func(*meta.Meta) api.ZettelRights
}

func (dze *dataZettelEncoder) writeMetaList(w io.Writer, ml []*meta.Meta) error {
	result := make(sx.Vector, len(ml)+1)
	result[0] = sx.SymbolList
	symID, symZettel := sx.MakeSymbol("id"), sx.MakeSymbol("zettel")
	for i, m := range ml {
		msz := sexp.EncodeMetaRights(api.MetaRights{
			Meta:   m.Map(),
			Rights: dze.getRights(m),
		})
		msz = sx.Cons(sx.MakeList(symID, sx.Int64(m.Zid)), msz.Cdr()).Cons(symZettel)
		result[i+1] = msz
	}

	_, err := sx.Print(w, sx.MakeList(
		sx.MakeSymbol("meta-list"),
		sx.MakeList(sx.MakeSymbol("query"), sx.String(dze.sq.String())),
		sx.MakeList(sx.MakeSymbol("human"), sx.String(dze.sq.Human())),
		sx.MakeList(result...),
	))
	return err
}
func (dze *dataZettelEncoder) writeArrangement(w io.Writer, act string, arr meta.Arrangement) error {
	result := sx.Nil()
	for aggKey, metaList := range arr {
		sxMeta := sx.Nil()
		for i := len(metaList) - 1; i >= 0; i-- {
			sxMeta = sxMeta.Cons(sx.Int64(metaList[i].Zid))
		}
		sxMeta = sxMeta.Cons(sx.String(aggKey))
		result = result.Cons(sxMeta)
	}
	_, err := sx.Print(w, sx.MakeList(
		sx.MakeSymbol("aggregate"),
		sx.String(act),
		sx.MakeList(sx.MakeSymbol("query"), sx.String(dze.sq.String())),
		sx.MakeList(sx.MakeSymbol("human"), sx.String(dze.sq.Human())),
		result.Cons(sx.SymbolList),
	))
	return err
}

func (a *API) handleTagZettel(w http.ResponseWriter, r *http.Request, tagZettel *usecase.TagZettel, vals url.Values) bool {
	tag := vals.Get(api.QueryKeyTag)
	if tag == "" {
		return false
	}
	ctx := r.Context()
	z, err := tagZettel.Run(ctx, tag)
	if err != nil {
		a.reportUsecaseError(w, err)
		return true
	}
	zid := z.Meta.Zid
	newURL := a.NewURLBuilder('z').SetZid(zid.ZettelID())
	for key, slVals := range vals {
		if key == api.QueryKeyTag {
			continue
		}
		for _, val := range slVals {
			newURL.AppendKVQuery(key, val)
		}
	}
	a.redirectFound(w, r, newURL, zid)
	return true
}

func (a *API) handleRoleZettel(w http.ResponseWriter, r *http.Request, roleZettel *usecase.RoleZettel, vals url.Values) bool {
	role := vals.Get(api.QueryKeyRole)
	if role == "" {
		return false
	}
	ctx := r.Context()
	z, err := roleZettel.Run(ctx, role)
	if err != nil {
		a.reportUsecaseError(w, err)
		return true
	}
	zid := z.Meta.Zid
	newURL := a.NewURLBuilder('z').SetZid(zid.ZettelID())
	for key, slVals := range vals {
		if key == api.QueryKeyRole {
			continue
		}
		for _, val := range slVals {
			newURL.AppendKVQuery(key, val)
		}
	}
	a.redirectFound(w, r, newURL, zid)
	return true
}

func (a *API) redirectFound(w http.ResponseWriter, r *http.Request, ub *api.URLBuilder, zid id.Zid) {
	w.Header().Set(api.HeaderContentType, content.PlainText)
	http.Redirect(w, r, ub.String(), http.StatusFound)
	if _, err := io.WriteString(w, zid.String()); err != nil {
		a.log.Error().Err(err).Msg("redirect body")
	}
}
