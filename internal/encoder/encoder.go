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
	"errors"
	"io"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"

	"zettelstore.de/z/internal/ast"
)

// Encoder is an interface that allows to encode different parts of a zettel.
type Encoder interface {
	WriteZettel(io.Writer, *ast.ZettelNode) (int, error)
	WriteMeta(io.Writer, *meta.Meta) (int, error)
	WriteContent(io.Writer, *ast.ZettelNode) (int, error)
	WriteBlocks(io.Writer, *ast.BlockSlice) (int, error)
	WriteInlines(io.Writer, *ast.InlineSlice) (int, error)
}

// Some errors to signal when encoder methods are not implemented.
var (
	ErrNoWriteZettel  = errors.New("method WriteZettel is not implemented")
	ErrNoWriteMeta    = errors.New("method WriteMeta is not implemented")
	ErrNoWriteContent = errors.New("method WriteContent is not implemented")
	ErrNoWriteBlocks  = errors.New("method WriteBlocks is not implemented")
	ErrNoWriteInlines = errors.New("method WriteInlines is not implemented")
)

// Create builds a new encoder with the given options.
func Create(enc api.EncodingEnum, params *CreateParameter) Encoder {
	switch enc {
	case api.EncoderHTML:
		// We need a new transformer every time, because tx.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &htmlEncoder{
			tx:      NewSzTransformer(),
			th:      shtml.NewEvaluator(1),
			lang:    params.Lang,
			textEnc: Create(api.EncoderText, nil),
		}
	case api.EncoderMD:
		return &mdEncoder{lang: params.Lang}
	case api.EncoderSHTML:
		// We need a new transformer every time, because tx.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &shtmlEncoder{
			tx:   NewSzTransformer(),
			th:   shtml.NewEvaluator(1),
			lang: params.Lang,
		}
	case api.EncoderSz:
		// We need a new transformer every time, because trans.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &szEncoder{trans: NewSzTransformer()}
	case api.EncoderText:
		return &theOnlyTextEncoder
	case api.EncoderZmk:
		return &theOnlyZmkEncoder
	}
	return nil
}

// CreateParameter contains values that are needed to create some encoder.
type CreateParameter struct {
	Lang string // default language
}

// GetEncodings returns all registered encodings, ordered by encoding value.
func GetEncodings() []api.EncodingEnum {
	return []api.EncodingEnum{api.EncoderHTML, api.EncoderMD, api.EncoderSHTML, api.EncoderSz, api.EncoderText, api.EncoderZmk}
}
