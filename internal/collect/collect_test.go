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

// Package collect_test provides some unit test for collectors.
package collect_test

import (
	"slices"
	"testing"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/collect"
)

func parseRef(s string) *ast.Reference {
	r := ast.ParseReference(s)
	if !r.IsValid() {
		panic(s)
	}
	return r
}

func TestReferenceSeq(t *testing.T) {
	t.Parallel()
	zn := &ast.ZettelNode{}
	summary := slices.Collect(collect.ReferenceSeq(zn))
	if len(summary) != 0 {
		t.Error("No references expected, but got:", summary)
	}

	intNode := &ast.LinkNode{Ref: parseRef("01234567890123")}
	para := ast.CreateParaNode(intNode, &ast.LinkNode{Ref: parseRef("https://zettelstore.de/z")})
	zn.BlocksAST = ast.BlockSlice{para}
	summary = slices.Collect(collect.ReferenceSeq(zn))
	if len(summary) != 2 {
		t.Error("2 refs expected, but got:", summary)
	}

	para.Inlines = append(para.Inlines, intNode)
	summary = slices.Collect(collect.ReferenceSeq(zn))
	if cnt := len(summary); cnt != 3 {
		t.Error("Ref count does not work. Expected: 3, got", summary)
	}

	zn = &ast.ZettelNode{
		BlocksAST: ast.BlockSlice{ast.CreateParaNode(&ast.EmbedRefNode{Ref: parseRef("12345678901234")})},
	}
	summary = slices.Collect(collect.ReferenceSeq(zn))
	if len(summary) != 1 {
		t.Error("Only one image ref expected, but got: ", summary)
	}
}
