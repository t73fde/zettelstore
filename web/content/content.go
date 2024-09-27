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

// Package content manages content handling within the web package.
// It translates syntax values into content types, and vice versa.
package content

import (
	"mime"
	"net/http"

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/meta"
)

// Some MIME encoding values.
const (
	UnknownMIME  = "application/octet-stream"
	mimeGIF      = "image/gif"
	mimeHTML     = "text/html; charset=utf-8"
	mimeJPEG     = "image/jpeg"
	mimeMarkdown = "text/markdown; charset=utf-8"
	PlainText    = "text/plain; charset=utf-8"
	mimePNG      = "image/png"
	SXPF         = PlainText
	mimeWEBP     = "image/webp"
)

var encoding2mime = map[api.EncodingEnum]string{
	api.EncoderHTML:  mimeHTML,
	api.EncoderMD:    mimeMarkdown,
	api.EncoderSz:    SXPF,
	api.EncoderSHTML: SXPF,
	api.EncoderText:  PlainText,
	api.EncoderZmk:   PlainText,
}

// MIMEFromEncoding returns the MIME encoding for a given zettel encoding
func MIMEFromEncoding(enc api.EncodingEnum) string {
	if m, found := encoding2mime[enc]; found {
		return m
	}
	return UnknownMIME
}

var syntax2mime = map[string]string{
	meta.SyntaxCSS:      "text/css; charset=utf-8",
	meta.SyntaxDraw:     PlainText,
	meta.SyntaxGif:      mimeGIF,
	meta.SyntaxHTML:     mimeHTML,
	meta.SyntaxJPEG:     mimeJPEG,
	meta.SyntaxJPG:      mimeJPEG,
	meta.SyntaxMarkdown: mimeMarkdown,
	meta.SyntaxMD:       mimeMarkdown,
	meta.SyntaxNone:     "",
	meta.SyntaxPlain:    PlainText,
	meta.SyntaxPNG:      mimePNG,
	meta.SyntaxSVG:      "image/svg+xml",
	meta.SyntaxSxn:      SXPF,
	meta.SyntaxText:     PlainText,
	meta.SyntaxTxt:      PlainText,
	meta.SyntaxWebp:     mimeWEBP,
	meta.SyntaxZmk:      "text/x-zmk; charset=utf-8",

	// Additional syntaxes that are parsed as plain text.
	"js":  "text/javascript; charset=utf-8",
	"pdf": "application/pdf",
	"xml": "text/xml; charset=utf-8",
}

// MIMEFromSyntax returns a MIME encoding for a given syntax value.
func MIMEFromSyntax(syntax string) string {
	if mt, found := syntax2mime[syntax]; found {
		return mt
	}
	return UnknownMIME
}

var mime2syntax = map[string]string{
	mimeGIF:         meta.SyntaxGif,
	mimeJPEG:        meta.SyntaxJPEG,
	mimePNG:         meta.SyntaxPNG,
	mimeWEBP:        meta.SyntaxWebp,
	"text/html":     meta.SyntaxHTML,
	"text/markdown": meta.SyntaxMarkdown,
	"text/plain":    meta.SyntaxText,

	// Additional syntaxes
	"application/pdf": "pdf",
	"text/javascript": "js",
}

// SyntaxFromMIME returns the syntax for a zettel based on MIME encoding value
// and the actual data.
func SyntaxFromMIME(m string, data []byte) string {
	mt, _, _ := mime.ParseMediaType(m)
	if syntax, found := mime2syntax[mt]; found {
		return syntax
	}
	if len(data) > 0 {
		ct := http.DetectContentType(data)
		mt, _, _ = mime.ParseMediaType(ct)
		if syntax, found := mime2syntax[mt]; found {
			return syntax
		}
		if ext, err := mime.ExtensionsByType(mt); err != nil && len(ext) > 0 {
			return ext[0][1:]
		}
		if zettel.IsBinary(data) {
			return "binary"
		}
	}
	return "plain"
}
