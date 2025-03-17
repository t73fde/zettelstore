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
)

// szEncoder contains all data needed for encoding.
type szEncoder struct {
	trans SzTransformer
}

// WriteZettel writes the encoded zettel to the writer.
func (enc *szEncoder) WriteZettel(w io.Writer, zn *ast.ZettelNode) (int, error) {
	content := enc.trans.GetSz(&zn.BlocksAST)
	meta := enc.trans.GetMeta(zn.InhMeta)
	return sx.MakeList(meta, content).Print(w)
}

// WriteMeta encodes meta data as s-expression.
func (enc *szEncoder) WriteMeta(w io.Writer, m *meta.Meta) (int, error) {
	return enc.trans.GetMeta(m).Print(w)
}

// WriteBlocks writes a block slice to the writer
func (enc *szEncoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) (int, error) {
	return enc.trans.GetSz(bs).Print(w)
}
