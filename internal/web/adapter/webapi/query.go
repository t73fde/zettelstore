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

package webapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"slices"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sexp"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter"
	"zettelstore.de/z/internal/web/content"
)

// MakeQueryHandler creates a new HTTP handler to perform a query.
func (a *WebAPI) MakeQueryHandler(
	queryMeta *usecase.Query,
	tagZettel *usecase.TagZettel,
	roleZettel *usecase.RoleZettel,
	reIndex *usecase.ReIndex,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				if slices.Contains(actions, webapi.RedirectAction) {
					zid := metaSeq[0].Zid
					ub := a.NewURLBuilder('z').SetZid(zid)
					a.redirectFound(w, r, ub, zid)
					return
				}
			}
		}

		var encoder zettelEncoder
		var contentType string
		switch enc, _ := getEncoding(r, urlQuery); enc {
		case webapi.EncoderPlain:
			encoder = &plainZettelEncoder{}
			contentType = content.PlainText

		case webapi.EncoderData:
			encoder = &dataZettelEncoder{
				sq:        sq,
				getRights: func(m *meta.Meta) webapi.ZettelRights { return a.getRights(ctx, m) },
			}
			contentType = content.SXPF

		default:
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		var buf bytes.Buffer
		err = queryAction(&buf, encoder, metaSeq, actions)
		if err != nil {
			a.logger.Error("execute query action", "err", err, "query", sq)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		if err = writeBuffer(w, &buf, contentType); err != nil {
			a.logger.Error("write result buffer", "err", err)
		}
	})
}
func queryAction(w io.Writer, enc zettelEncoder, ml []*meta.Meta, actions []string) error {
	minVal, maxVal := -1, -1
	if len(actions) > 0 {
		acts := make([]string, 0, len(actions))
		for _, act := range actions {
			if strings.HasPrefix(act, webapi.MinAction) {
				if num, err := strconv.Atoi(act[3:]); err == nil && num > 0 {
					minVal = num
					continue
				}
			}
			if strings.HasPrefix(act, webapi.MaxAction) {
				if num, err := strconv.Atoi(act[3:]); err == nil && num > 0 {
					maxVal = num
					continue
				}
			}
			acts = append(acts, act)
		}
		for _, act := range acts {
			if act == webapi.KeysAction {
				return encodeKeysArrangement(w, enc, ml, act)
			}
			switch key := strings.ToLower(act); meta.Type(key) {
			case meta.TypeWord, meta.TypeTagSet:
				return encodeMetaKeyArrangement(w, enc, ml, key, minVal, maxVal)
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

func encodeMetaKeyArrangement(w io.Writer, enc zettelEncoder, ml []*meta.Meta, key string, minVal, maxVal int) error {
	arr0 := meta.CreateArrangement(ml, key)
	arr := make(meta.Arrangement, len(arr0))
	for k0, ml0 := range arr0 {
		if len(ml0) < minVal || (maxVal > 0 && len(ml0) > maxVal) {
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
		_, err := io.WriteString(w, string(key))
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
	getRights func(*meta.Meta) webapi.ZettelRights
}

func (dze *dataZettelEncoder) writeMetaList(w io.Writer, ml []*meta.Meta) error {
	var lb sx.ListBuilder
	lb.AddN(
		sx.MakeSymbol("meta-list"),
		sx.MakeList(sx.MakeSymbol("query"), sx.MakeString(dze.sq.String())),
		sx.MakeList(sx.MakeSymbol("human"), sx.MakeString(dze.sq.Human())),
	)
	symID, symZettel := sx.MakeSymbol("id"), sx.MakeSymbol("zettel")
	for _, m := range ml {
		msz := sexp.EncodeMetaRights(webapi.MetaRights{
			Meta:   m.Map(),
			Rights: dze.getRights(m),
		})
		msz = sx.Cons(sx.MakeList(symID, sx.Int64(m.Zid)), msz.Cdr()).Cons(symZettel)
		lb.Add(msz)
	}
	_, err := sx.Print(w, lb.List())
	return err
}
func (dze *dataZettelEncoder) writeArrangement(w io.Writer, act string, arr meta.Arrangement) error {
	var lb sx.ListBuilder
	lb.AddN(
		sx.MakeSymbol("meta-list"),
		sx.MakeString(act),
		sx.MakeList(sx.MakeSymbol("query"), sx.MakeString(dze.sq.String())),
		sx.MakeList(sx.MakeSymbol("human"), sx.MakeString(dze.sq.Human())),
	)
	for aggKey, metaList := range arr {
		var lbMeta sx.ListBuilder
		lbMeta.Add(sx.MakeString(aggKey))
		for _, m := range metaList {
			lbMeta.Add(sx.Int64(m.Zid))
		}
		lb.Add(lbMeta.List())
	}
	_, err := sx.Print(w, lb.List())
	return err
}

func (a *WebAPI) handleTagZettel(w http.ResponseWriter, r *http.Request, tagZettel *usecase.TagZettel, vals url.Values) bool {
	tag := vals.Get(webapi.QueryKeyTag)
	if tag == "" {
		return false
	}
	ctx := r.Context()
	z, err := tagZettel.Run(ctx, meta.Value(tag))
	if err != nil {
		a.reportUsecaseError(w, err)
		return true
	}
	zid := z.Meta.Zid
	newURL := a.NewURLBuilder('z').SetZid(zid)
	for key, slVals := range vals {
		if key == webapi.QueryKeyTag {
			continue
		}
		for _, val := range slVals {
			newURL.AppendKVQuery(key, val)
		}
	}
	a.redirectFound(w, r, newURL, zid)
	return true
}

func (a *WebAPI) handleRoleZettel(w http.ResponseWriter, r *http.Request, roleZettel *usecase.RoleZettel, vals url.Values) bool {
	role := vals.Get(webapi.QueryKeyRole)
	if role == "" {
		return false
	}
	ctx := r.Context()
	z, err := roleZettel.Run(ctx, meta.Value(role))
	if err != nil {
		a.reportUsecaseError(w, err)
		return true
	}
	zid := z.Meta.Zid
	newURL := a.NewURLBuilder('z').SetZid(zid)
	for key, slVals := range vals {
		if key == webapi.QueryKeyRole {
			continue
		}
		for _, val := range slVals {
			newURL.AppendKVQuery(key, val)
		}
	}
	a.redirectFound(w, r, newURL, zid)
	return true
}

func (a *WebAPI) redirectFound(w http.ResponseWriter, r *http.Request, ub *webapi.URLBuilder, zid id.Zid) {
	w.Header().Set(webapi.HeaderContentType, content.PlainText)
	http.Redirect(w, r, ub.String(), http.StatusFound)
	if _, err := io.WriteString(w, zid.String()); err != nil {
		a.logger.Error("redirect body", "err", err)
	}
}
