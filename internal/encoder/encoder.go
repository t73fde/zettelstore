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

// Package encoder provides a generic interface to encode the abstract syntax
// tree into some text form.
package encoder

import (
	"io"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/domain"
)

// Encoder is an interface that allows to encode different parts of a zettel.
type Encoder interface {
	// WriteZettel encodes a whole zettel and writes it to the Writer.
	WriteZettel(io.Writer, *domain.Zettel) error

	// WriteMeta encodes just the metadata.
	WriteMeta(io.Writer, *meta.Meta) error

	// WriteSz encodes  SZ represented zettel content.
	WriteSz(io.Writer, *sx.Pair) error
}

// Create builds a new encoder with the given options.
func Create(enc webapi.EncodingEnum, params *CreateParameter) Encoder {
	switch enc {
	case webapi.EncoderHTML:
		// We need a new transformer every time, because tx.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &htmlEncoder{
			th:   shtml.NewEvaluator(1),
			lang: params.Lang,
		}
	case webapi.EncoderMD:
		return &mdEncoder{lang: params.Lang}
	case webapi.EncoderSHTML:
		// We need a new transformer every time, because tx.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &shtmlEncoder{
			th:   shtml.NewEvaluator(1),
			lang: params.Lang,
		}
	case webapi.EncoderSz:
		// We need a new transformer every time, because trans.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &szEncoder{}
	case webapi.EncoderText:
		return (*TextEncoder)(nil)
	case webapi.EncoderZmk:
		return (*zmkEncoder)(nil)
	}
	return nil
}

// CreateParameter contains values that are needed to create some encoder.
type CreateParameter struct {
	Lang string // default language
}

// GetEncodings returns all registered encodings, ordered by encoding value.
func GetEncodings() []webapi.EncodingEnum {
	return []webapi.EncodingEnum{
		webapi.EncoderHTML, webapi.EncoderMD, webapi.EncoderSHTML,
		webapi.EncoderSz, webapi.EncoderText, webapi.EncoderZmk,
	}
}
