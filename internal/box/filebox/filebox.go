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

// Package filebox provides boxes that are stored in a file.
package filebox

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/box/manager"
	"zettelstore.de/z/internal/kernel"
)

func init() {
	manager.Register("file", func(u *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
		path := getFilepathFromURL(u)
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".zip" {
			return nil, errors.New("unknown extension '" + ext + "' in box URL: " + u.String())
		}
		return &zipBox{
			logger:   kernel.Main.GetLogger(kernel.BoxService).With("box", "zip", "boxnum", cdata.Number),
			number:   cdata.Number,
			name:     path,
			enricher: cdata.Enricher,
			notify:   cdata.Notify,
		}, nil
	})
}

func getFilepathFromURL(u *url.URL) string {
	name := u.Opaque
	if name == "" {
		name = u.Path
	}
	components := strings.Split(name, "/")
	fileName := filepath.Join(components...)
	if len(components) > 0 && components[0] == "" {
		return "/" + fileName
	}
	return fileName
}

var alternativeSyntax = map[string]meta.Value{
	"htm": "html",
}

func calculateSyntax(ext string) meta.Value {
	ext = strings.ToLower(ext)
	if syntax, ok := alternativeSyntax[ext]; ok {
		return syntax
	}
	return meta.Value(ext)
}

// CalcDefaultMeta returns metadata with default values for the given entry.
func CalcDefaultMeta(zid id.Zid, ext string) *meta.Meta {
	m := meta.New(zid)
	m.Set(meta.KeySyntax, calculateSyntax(ext))
	return m
}

// CleanupMeta enhances the given metadata.
func CleanupMeta(m *meta.Meta, zid id.Zid, ext string, inMeta bool, uselessFiles []string) {
	if inMeta {
		if syntax, ok := m.Get(meta.KeySyntax); !ok || syntax == "" {
			dm := CalcDefaultMeta(zid, ext)
			syntax, ok = dm.Get(meta.KeySyntax)
			if !ok {
				panic("Default meta must contain syntax")
			}
			m.Set(meta.KeySyntax, syntax)
		}
	}

	if len(uselessFiles) > 0 {
		m.Set(meta.KeyUselessFiles, meta.Value(strings.Join(uselessFiles, " ")))
	}
}
