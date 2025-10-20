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

// htmlenc encodes the abstract syntax tree into HTML5 via zettelstore-client.

import (
	"io"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/ast/sztrans"
)

// htmlEncoder contains all data needed for encoding.
type htmlEncoder struct {
	tx      sztrans.SzTransformer
	th      *shtml.Evaluator
	lang    string
	textEnc TextEncoder
}

// WriteZettel encodes a full zettel as HTML5.
func (he *htmlEncoder) WriteZettel(w io.Writer, zn *ast.Zettel) error {
	env := shtml.MakeEnvironment(he.lang)
	hm, err := he.th.Evaluate(sztrans.GetMetaSz(zn.InhMeta), &env)
	if err != nil {
		return err
	}

	var isTitle ast.InlineSlice
	var htitle *sx.Pair
	plainTitle, hasTitle := zn.InhMeta.Get(meta.KeyTitle)
	if hasTitle {
		isTitle = ast.ParseSpacedTextAST(string(plainTitle))
		xtitle := he.tx.GetSz(&isTitle)
		htitle, err = he.th.Evaluate(xtitle, &env)
		if err != nil {
			return err
		}
	}

	xast := he.tx.GetSz(&zn.BlocksAST)
	hast, err := he.th.Evaluate(xast, &env)
	if err != nil {
		return err
	}
	hen := shtml.Endnotes(&env)

	var head sx.ListBuilder
	head.AddN(
		shtml.SymHead,
		sx.Nil().Cons(sx.Nil().Cons(sx.Cons(sxhtml.MakeSymbol("charset"), sx.MakeString("utf-8")))).Cons(shtml.SymMeta),
	)
	head.ExtendBang(hm)
	var sb strings.Builder
	if hasTitle {
		_ = he.textEnc.WriteInlines(&sb, &isTitle)
	} else {
		sb.Write(zn.Meta.Zid.Bytes())
	}
	head.Add(sx.MakeList(shtml.SymAttrTitle, sx.MakeString(sb.String())))

	var body sx.ListBuilder
	body.Add(shtml.SymBody)
	if hasTitle {
		body.Add(htitle.Cons(shtml.SymH1))
	}
	body.ExtendBang(hast)
	if hen != nil {
		body.AddN(sx.Cons(shtml.SymHR, nil), hen)
	}

	doc := sx.MakeList(
		sxhtml.SymDoctype,
		sx.MakeList(shtml.SymHTML, head.List(), body.List()),
	)

	gen := sxhtml.NewGenerator().SetNewline()
	return gen.WriteHTML(w, doc)
}

// WriteMeta encodes meta data as HTML5.
func (he *htmlEncoder) WriteMeta(w io.Writer, m *meta.Meta) error {
	env := shtml.MakeEnvironment(he.lang)
	hm, err := he.th.Evaluate(sztrans.GetMetaSz(m), &env)
	if err != nil {
		return err
	}
	gen := sxhtml.NewGenerator().SetNewline()
	return gen.WriteListHTML(w, hm)
}

// WriteSz encodes SZ represented zettel content.
func (he *htmlEncoder) WriteSz(w io.Writer, node *sx.Pair) error {
	env := shtml.MakeEnvironment(he.lang)
	hobj, err := he.th.Evaluate(node, &env)
	if err == nil {
		gen := sxhtml.NewGenerator()
		if err = gen.WriteListHTML(w, hobj); err != nil {
			return err
		}

		return gen.WriteHTML(w, shtml.Endnotes(&env))
	}
	return err
}

// WriteBlocks encodes a block slice.
func (he *htmlEncoder) WriteBlocks(w io.Writer, bs *ast.BlockSlice) error {
	env := shtml.MakeEnvironment(he.lang)
	hobj, err := he.th.Evaluate(he.tx.GetSz(bs), &env)
	if err == nil {
		gen := sxhtml.NewGenerator()
		if err = gen.WriteListHTML(w, hobj); err != nil {
			return err
		}

		return gen.WriteHTML(w, shtml.Endnotes(&env))
	}
	return err
}
