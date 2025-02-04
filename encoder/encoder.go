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
	"fmt"
	"io"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/ast"
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
	if create, ok := registry[enc]; ok {
		return create(params)
	}
	return nil
}

// CreateFunc produces a new encoder.
type CreateFunc func(*CreateParameter) Encoder

// CreateParameter contains values that are needed to create an encoder.
type CreateParameter struct {
	Lang string // default language
}

var registry = map[api.EncodingEnum]CreateFunc{}

// Register the encoder for later retrieval.
func Register(enc api.EncodingEnum, create CreateFunc) {
	if _, ok := registry[enc]; ok {
		panic(fmt.Sprintf("Encoder %q already registered", enc))
	}
	registry[enc] = create
}

// GetEncodings returns all registered encodings, ordered by encoding value.
func GetEncodings() []api.EncodingEnum {
	result := make([]api.EncodingEnum, 0, len(registry))
	for enc := range registry {
		result = append(result, enc)
	}
	return result
}
