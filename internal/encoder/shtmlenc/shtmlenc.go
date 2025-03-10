//-----------------------------------------------------------------------------
// Copyright (c) 2023-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2023-present Detlef Stern
//-----------------------------------------------------------------------------

// Package shtmlenc encodes the abstract syntax tree into a s-expr which represents HTML.
package shtmlenc

import (
	"io"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/encoder/szenc"
)

func init() {
	encoder.Register(api.EncoderSHTML, func(params *encoder.CreateParameter) encoder.Encoder { return Create(params) })
}

// Create a SHTML encoder
func Create(params *encoder.CreateParameter) *Encoder {
	// We need a new transformer every time, because tx.inVerse must be unique.
	// If we can refactor it out, the transformer can be created only once.
	return &Encoder{
		tx:   szenc.NewTransformer(),
		th:   shtml.NewEvaluator(1),
		lang: params.Lang,
	}
}

// Encoder contains all data needed for encoding.
type Encoder struct {
	tx   *szenc.Transformer
	th   *shtml.Evaluator
	lang string
}

// WriteZettel writes the encoded zettel to the writer.
func (enc *Encoder) WriteZettel(w io.Writer, zn *ast.ZettelNode) (int, error) {
	env := shtml.MakeEnvironment(enc.lang)
	metaSHTML, err := enc.th.Evaluate(enc.tx.GetMeta(zn.InhMeta), &env)
	if err != nil {
		return 0, err
	}
	contentSHTML, err := enc.th.Evaluate(enc.tx.GetSz(&zn.BlocksAST), &env)
	if err != nil {
		return 0, err
	}
	result := sx.Cons(metaSHTML, contentSHTML)
	return result.Print(w)
}

// WriteMeta encodes meta data as s-expression.
func (enc *Encoder) WriteMeta(w io.Writer, m *meta.Meta) (int, error) {
	env := shtml.MakeEnvironment(enc.lang)
	metaSHTML, err := enc.th.Evaluate(enc.tx.GetMeta(m), &env)
	if err != nil {
		return 0, err
	}
	return sx.Print(w, metaSHTML)
}

// WriteContent encodes the zettel content.
func (enc *Encoder) WriteContent(w io.Writer, zn *ast.ZettelNode) (int, error) {
	return enc.WriteBlocks(w, &zn.BlocksAST)
}

// WriteBlocks writes a block slice to the writer
func (enc *Encoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) (int, error) {
	env := shtml.MakeEnvironment(enc.lang)
	hval, err := enc.th.Evaluate(enc.tx.GetSz(bs), &env)
	if err != nil {
		return 0, err
	}
	return sx.Print(w, hval)
}

// WriteInlines writes an inline slice to the writer
func (enc *Encoder) WriteInlines(w io.Writer, is *ast.InlineSlice) (int, error) {
	env := shtml.MakeEnvironment(enc.lang)
	hval, err := enc.th.Evaluate(enc.tx.GetSz(is), &env)
	if err != nil {
		return 0, err
	}
	return sx.Print(w, hval)
}
