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

package encoder

// szenc encodes the abstract syntax tree into a s-expr for zettel.

import (
	"io"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/ast/sztrans"
)

// szEncoder contains all data needed for encoding.
type szEncoder struct {
	trans sztrans.SzTransformer
}

// WriteZettel writes the encoded zettel to the writer.
func (enc *szEncoder) WriteZettel(w io.Writer, zn *ast.Zettel) error {
	content := enc.trans.GetSz(&zn.BlocksAST)
	meta := sztrans.GetMetaSz(zn.InhMeta)
	_, err := sx.MakeList(meta, content).Print(w)
	return err
}

// WriteMeta encodes meta data as s-expression.
func (enc *szEncoder) WriteMeta(w io.Writer, m *meta.Meta) error {
	_, err := sztrans.GetMetaSz(m).Print(w)
	return err
}

// WriteSz encodes SZ represented zettel content.
func (*szEncoder) WriteSz(w io.Writer, node *sx.Pair) error {
	_, err := node.Print(w)
	return err
}

// WriteBlocks writes a block slice to the writer
func (enc *szEncoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) error {
	_, err := enc.trans.GetSz(bs).Print(w)
	return err
}
