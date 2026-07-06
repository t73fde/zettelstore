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

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/zettel"
)

// Some MIME encoding values.
const (
	UnknownMIME      = "application/octet-stream"
	charsetUTF8      = "; charset=utf-8"
	mimeCSS          = "text/css"
	mimeCSSUTF8      = mimeCSS + charsetUTF8
	mimeGIF          = "image/gif"
	mimeHTML         = "text/html"
	mimeHTMLUTF8     = mimeHTML + charsetUTF8
	mimeJPEG         = "image/jpeg"
	mimeJS           = "text/javascript"
	mimeJSUTF8       = mimeJS + charsetUTF8
	mimeMarkdown     = "text/markdown"
	mimeMarkdownUTF8 = mimeMarkdown + charsetUTF8
	mimePlain        = "text/plain"
	PlainTextUTF8    = mimePlain + charsetUTF8
	mimePNG          = "image/png"
	mimeSVG          = "image/svg+xml"
	SXPFUTF8         = PlainTextUTF8
	mimeWEBP         = "image/webp"
)

var encoding2mime = map[webapi.EncodingEnum]string{
	webapi.EncoderHTML:  mimeHTMLUTF8,
	webapi.EncoderMD:    mimeMarkdownUTF8,
	webapi.EncoderSz:    SXPFUTF8,
	webapi.EncoderSHTML: SXPFUTF8,
	webapi.EncoderText:  PlainTextUTF8,
	webapi.EncoderZmk:   PlainTextUTF8,
}

// MIMEFromEncoding returns the MIME encoding for a given zettel encoding
func MIMEFromEncoding(enc webapi.EncodingEnum) string {
	if m, found := encoding2mime[enc]; found {
		return m
	}
	return UnknownMIME
}

var syntax2mime = map[string]string{
	meta.ValueSyntaxCommonMark: mimeMarkdownUTF8,
	meta.ValueSyntaxCMark:      mimeMarkdownUTF8,
	meta.ValueSyntaxCSS:        mimeCSSUTF8,
	meta.ValueSyntaxDraw:       PlainTextUTF8,
	meta.ValueSyntaxEMark:      mimeMarkdownUTF8,
	meta.ValueSyntaxGif:        mimeGIF,
	meta.ValueSyntaxHTML:       mimeHTMLUTF8,
	meta.ValueSyntaxJPEG:       mimeJPEG,
	meta.ValueSyntaxJPG:        mimeJPEG,
	meta.ValueSyntaxJS:         mimeJSUTF8,
	meta.ValueSyntaxMarkdown:   mimeMarkdownUTF8,
	meta.ValueSyntaxMD:         mimeMarkdownUTF8,
	meta.ValueSyntaxNone:       "",
	meta.ValueSyntaxPlain:      PlainTextUTF8,
	meta.ValueSyntaxPNG:        mimePNG,
	meta.ValueSyntaxSVG:        mimeSVG,
	meta.ValueSyntaxSxn:        SXPFUTF8,
	meta.ValueSyntaxText:       PlainTextUTF8,
	meta.ValueSyntaxTxt:        PlainTextUTF8,
	meta.ValueSyntaxWebp:       mimeWEBP,
	meta.ValueSyntaxZmk:        "text/x-zmk; charset=utf-8",

	// Additional syntaxes that are parsed as plain text.
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
	mimeGIF:      meta.ValueSyntaxGif,
	mimeJPEG:     meta.ValueSyntaxJPEG,
	mimeJS:       meta.ValueSyntaxJS,
	mimePNG:      meta.ValueSyntaxPNG,
	mimeWEBP:     meta.ValueSyntaxWebp,
	mimeCSS:      meta.ValueSyntaxCSS,
	mimeHTML:     meta.ValueSyntaxHTML,
	mimeMarkdown: meta.ValueSyntaxMarkdown,
	mimePlain:    meta.ValueSyntaxText,
	mimeSVG:      meta.ValueSyntaxSVG,

	// Additional syntaxes
	"application/pdf":          "pdf",
	"application/x-javascript": meta.ValueSyntaxJS,
	"text/xml":                 "xml",
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
