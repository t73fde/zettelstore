//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package api provides api handlers for web requests.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"zettelstore.de/z/ast"
	"zettelstore.de/z/collect"
	"zettelstore.de/z/domain/id"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
)

type jsonGetLinks struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Links struct {
		Incoming []jsonIDURL `json:"incoming"`
		Outgoing []jsonIDURL `json:"outgoing"`
		Local    []string    `json:"local"`
		External []string    `json:"external"`
	} `json:"links"`
	Images struct {
		Outgoing []jsonIDURL `json:"outgoing"`
		Local    []string    `json:"local"`
		External []string    `json:"external"`
	} `json:"images"`
	Cites []string `json:"cites"`
}

// MakeGetLinksHandler creates a new API handler to return links to other material.
func MakeGetLinksHandler(parseZettel usecase.ParseZettel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		zid, err := id.Parse(r.URL.Path[1:])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ctx := r.Context()
		q := r.URL.Query()
		zn, err := parseZettel.Run(ctx, zid, q.Get("syntax"))
		if err != nil {
			adapter.ReportUsecaseError(w, err)
			return
		}
		summary := collect.References(zn)

		kind := getKindFromValue(q.Get("kind"))
		matter := getMatterFromValue(q.Get("matter"))
		if !validKindMatter(kind, matter) {
			adapter.BadRequest(w, "Invalid kind/matter")
			return
		}

		outData := jsonGetLinks{
			ID:  zid.String(),
			URL: adapter.NewURLBuilder('z').SetZid(zid).String(),
		}
		if kind&kindLink != 0 {
			if matter&matterIncoming != 0 {
				// Backlinks not yet implemented
				outData.Links.Incoming = []jsonIDURL{}
			}
			zetRefs, locRefs, extRefs := collect.DivideReferences(summary.Links, false)
			if matter&matterOutgoing != 0 {
				outData.Links.Outgoing = idURLRefs(zetRefs)
			}
			if matter&matterLocal != 0 {
				outData.Links.Local = stringRefs(locRefs)
			}
			if matter&matterExternal != 0 {
				outData.Links.External = stringRefs(extRefs)
			}
		}
		if kind&kindImage != 0 {
			zetRefs, locRefs, extRefs := collect.DivideReferences(summary.Images, false)
			if matter&matterOutgoing != 0 {
				outData.Images.Outgoing = idURLRefs(zetRefs)
			}
			if matter&matterLocal != 0 {
				outData.Images.Local = stringRefs(locRefs)
			}
			if matter&matterExternal != 0 {
				outData.Images.External = stringRefs(extRefs)
			}
		}
		if kind&kindCite != 0 {
			outData.Cites = stringCites(summary.Cites)
		}

		w.Header().Set("Content-Type", format2ContentType("json"))
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(&outData)
	}
}

func idURLRefs(refs []*ast.Reference) []jsonIDURL {
	result := make([]jsonIDURL, 0, len(refs))
	for _, ref := range refs {
		path := ref.URL.Path
		ub := adapter.NewURLBuilder('z').AppendPath(path)
		if fragment := ref.URL.Fragment; len(fragment) > 0 {
			ub.SetFragment(fragment)
		}
		result = append(result, jsonIDURL{ID: path, URL: ub.String()})
	}
	return result
}

func stringRefs(refs []*ast.Reference) []string {
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref.String())
	}
	return result
}

func stringCites(cites []*ast.CiteNode) []string {
	mapKey := make(map[string]bool)
	result := make([]string, 0, len(cites))
	for _, cn := range cites {
		if _, ok := mapKey[cn.Key]; !ok {
			mapKey[cn.Key] = true
			result = append(result, cn.Key)
		}
	}
	return result
}

type kindType int

const (
	_ kindType = 1 << iota
	kindLink
	kindImage
	kindCite
)

var mapKind = map[string]kindType{
	"":      kindLink | kindImage | kindCite,
	"link":  kindLink,
	"image": kindImage,
	"cite":  kindCite,
	"both":  kindLink | kindImage,
	"all":   kindLink | kindImage | kindCite,
}

func getKindFromValue(value string) kindType {
	if k, ok := mapKind[value]; ok {
		return k
	}
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return kindType(n)
	}
	return 0
}

type matterType int

const (
	_ matterType = 1 << iota
	matterIncoming
	matterOutgoing
	matterLocal
	matterExternal
)

var mapMatter = map[string]matterType{
	"":         matterIncoming | matterOutgoing | matterLocal | matterExternal,
	"incoming": matterIncoming,
	"outgoing": matterOutgoing,
	"local":    matterLocal,
	"external": matterExternal,
	"zettel":   matterIncoming | matterOutgoing,
	"material": matterLocal | matterExternal,
	"all":      matterIncoming | matterOutgoing | matterLocal | matterExternal,
}

func getMatterFromValue(value string) matterType {
	if m, ok := mapMatter[value]; ok {
		return m
	}
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return matterType(n)
	}
	return 0
}

func validKindMatter(kind kindType, matter matterType) bool {
	if kind == 0 {
		return false
	}
	if kind&kindLink != 0 {
		if matter == 0 {
			return false
		}
		return true
	}
	if kind&kindImage != 0 {
		if matter == 0 || matter == matterIncoming {
			return false
		}
		return true
	}
	if kind&kindCite != 0 {
		return matter == matterOutgoing
	}
	return false
}
