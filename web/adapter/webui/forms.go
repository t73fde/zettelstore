//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package webui

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"zettelstore.de/c/api"
	"zettelstore.de/z/input"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/web/content"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

type formZettelData struct {
	Heading       string
	FormActionURL string
	MetaTitle     string
	MetaRole      string
	HasRoleData   bool
	RoleData      []string
	HasSyntaxData bool
	SyntaxData    []string
	MetaTags      string
	MetaSyntax    string
	MetaPairsRest []meta.Pair
	IsTextContent bool
	Content       string
}

var (
	bsCRLF = []byte{'\r', '\n'}
	bsLF   = []byte{'\n'}
)

var errMissingContent = errors.New("missing zettel content")

func parseZettelForm(r *http.Request, zid id.Zid) (bool, zettel.Zettel, error) {
	maxRequestSize := kernel.Main.GetConfig(kernel.WebService, kernel.WebMaxRequestSize).(int64)
	err := r.ParseMultipartForm(maxRequestSize)
	if err != nil {
		return false, zettel.Zettel{}, err
	}
	_, doSave := r.Form["save"]

	var m *meta.Meta
	if postMeta, ok := trimmedFormValue(r, "meta"); ok {
		m = meta.NewFromInput(zid, input.NewInput(removeEmptyLines([]byte(postMeta))))
		m.Sanitize()
	} else {
		m = meta.New(zid)
	}
	if postTitle, ok := trimmedFormValue(r, "title"); ok {
		m.Set(api.KeyTitle, meta.RemoveNonGraphic(postTitle))
	}
	if postTags, ok := trimmedFormValue(r, "tags"); ok {
		if tags := strings.Fields(meta.RemoveNonGraphic(postTags)); len(tags) > 0 {
			m.SetList(api.KeyTags, tags)
		}
	}
	if postRole, ok := trimmedFormValue(r, "role"); ok {
		m.Set(api.KeyRole, meta.RemoveNonGraphic(postRole))
	}
	if postSyntax, ok := trimmedFormValue(r, "syntax"); ok {
		m.Set(api.KeySyntax, meta.RemoveNonGraphic(postSyntax))
	}

	if data := textContent(r); data != nil {
		return doSave, zettel.Zettel{Meta: m, Content: zettel.NewContent(data)}, nil
	}
	if data, m2 := uploadedContent(r, m); data != nil {
		return doSave, zettel.Zettel{Meta: m2, Content: zettel.NewContent(data)}, nil
	}

	if allowEmptyContent(m) {
		return doSave, zettel.Zettel{Meta: m, Content: zettel.NewContent(nil)}, nil
	}
	return doSave, zettel.Zettel{Meta: m, Content: zettel.NewContent(nil)}, errMissingContent
}

func trimmedFormValue(r *http.Request, key string) (string, bool) {
	if values, ok := r.PostForm[key]; ok && len(values) > 0 {
		value := strings.TrimSpace(values[0])
		if len(value) > 0 {
			return value, true
		}
	}
	return "", false
}

func textContent(r *http.Request) []byte {
	if values, found := r.PostForm["content"]; found && len(values) > 0 {
		result := bytes.ReplaceAll([]byte(values[0]), bsCRLF, bsLF)
		if bytes.IndexFunc(result, func(ch rune) bool { return !unicode.IsSpace(ch) }) >= 0 {
			return result
		}
	}
	return nil
}

func uploadedContent(r *http.Request, m *meta.Meta) ([]byte, *meta.Meta) {
	file, fh, err := r.FormFile("file")
	if file != nil {
		defer file.Close()
		if err == nil {
			data, err2 := io.ReadAll(file)
			if err2 != nil {
				return nil, m
			}
			if cts, found := fh.Header["Content-Type"]; found && len(cts) > 0 {
				ct := cts[0]
				if fileSyntax := content.SyntaxFromMIME(ct, data); fileSyntax != "" {
					m = m.Clone()
					m.Set(api.KeySyntax, fileSyntax)
				}
			}
			return data, m
		}
	}
	return nil, m
}

func allowEmptyContent(m *meta.Meta) bool {
	if syntax, found := m.Get(api.KeySyntax); found {
		if syntax == api.ValueSyntaxNone {
			return true
		}
		if pinfo := parser.Get(syntax); pinfo != nil {
			return pinfo.IsTextFormat
		}
	}
	return true
}

var reEmptyLines = regexp.MustCompile(`(\n|\r)+\s*(\n|\r)+`)

func removeEmptyLines(s []byte) []byte {
	b := bytes.TrimSpace(s)
	return reEmptyLines.ReplaceAllLiteral(b, []byte{'\n'})
}
