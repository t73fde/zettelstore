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

package encoder

// shtmlenc encodes the abstract syntax tree into a s-expr which represents HTML.

import (
	"io"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"

	"zettelstore.de/z/internal/ast"
)

// shtmlEncoder contains all data needed for encoding.
type shtmlEncoder struct {
	th   *shtml.Evaluator
	lang string
}

// WriteZettel writes the encoded zettel to the writer.
func (enc *shtmlEncoder) WriteZettel(w io.Writer, zn *ast.Zettel) error {
	env := shtml.MakeEnvironment(enc.lang)
	metaSHTML, err := enc.th.Evaluate(ast.GetMetaSz(zn.InhMeta), &env)
	if err != nil {
		return err
	}
	contentSHTML, err := enc.th.Evaluate(zn.Blocks, &env)
	if err != nil {
		return err
	}
	result := sx.Cons(metaSHTML, contentSHTML)
	_, err = result.Print(w)
	return err
}

// WriteMeta encodes meta data as s-expression.
func (enc *shtmlEncoder) WriteMeta(w io.Writer, m *meta.Meta) error {
	env := shtml.MakeEnvironment(enc.lang)
	metaSHTML, err := enc.th.Evaluate(ast.GetMetaSz(m), &env)
	if err != nil {
		return err
	}
	_, err = sx.Print(w, metaSHTML)
	return err
}

// WriteSz encodes SZ represented zettel content.
func (enc *shtmlEncoder) WriteSz(w io.Writer, node *sx.Pair) error {
	env := shtml.MakeEnvironment(enc.lang)
	hval, err := enc.th.Evaluate(node, &env)
	if err != nil {
		return err
	}
	_, err = hval.Print(w)
	return err
}
