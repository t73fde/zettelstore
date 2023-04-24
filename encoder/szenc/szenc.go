//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package szenc encodes the abstract syntax tree into a s-expr for zettel.
package szenc

import (
	"io"

	"codeberg.org/t73fde/sxpf"
	"zettelstore.de/c/api"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/domain/meta"
	"zettelstore.de/z/encoder"
)

func init() {
	encoder.Register(api.EncoderSz, func() encoder.Encoder { return Create() })
}

// Create a S-expr encoder
func Create() *Encoder {
	// We need a new transformer every time, because trans.inVerse must be unique.
	// If we can refactor it out, the transformer can be created only once.
	return &Encoder{trans: NewTransformer()}
}

type Encoder struct {
	trans *Transformer
}

// WriteZettel writes the encoded zettel to the writer.
func (enc *Encoder) WriteZettel(w io.Writer, zn *ast.ZettelNode, evalMeta encoder.EvalMetaFunc) (int, error) {
	content := enc.trans.GetSz(&zn.Ast)
	meta := enc.trans.GetMeta(zn.InhMeta, evalMeta)
	return sxpf.MakeList(meta, content).Print(w)
}

// WriteMeta encodes meta data as s-expression.
func (enc *Encoder) WriteMeta(w io.Writer, m *meta.Meta, evalMeta encoder.EvalMetaFunc) (int, error) {
	return enc.trans.GetMeta(m, evalMeta).Print(w)
}

func (enc *Encoder) WriteContent(w io.Writer, zn *ast.ZettelNode) (int, error) {
	return enc.WriteBlocks(w, &zn.Ast)
}

// WriteBlocks writes a block slice to the writer
func (enc *Encoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) (int, error) {
	return enc.trans.GetSz(bs).Print(w)
}

// WriteInlines writes an inline slice to the writer
func (enc *Encoder) WriteInlines(w io.Writer, is *ast.InlineSlice) (int, error) {
	return enc.trans.GetSz(is).Print(w)
}
