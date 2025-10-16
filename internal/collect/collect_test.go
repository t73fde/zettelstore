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

	"t73f.de/r/sx"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/collect"
)

func parseRef(s string) *sx.Pair {
	r := sz.ScanReference(s)
	sym, _ := zsx.GetReference(r)
	if zsx.SymRefStateInvalid.IsEqualSymbol(sym) {
		panic(s)
	}
	return r
}

func TestReferenceSeq(t *testing.T) {
	t.Parallel()
	summary := slices.Collect(collect.ReferenceSeq(nil))
	if len(summary) != 0 {
		t.Error("No references expected, but got:", summary)
	}

	intNode := zsx.MakeLink(nil, parseRef("01234567890123"), nil)
	para := zsx.MakePara(intNode, zsx.MakeLink(nil, parseRef("https://zettelstore.de/z"), nil))
	blocks := zsx.MakeBlock(para)
	summary = slices.Collect(collect.ReferenceSeq(blocks))
	if len(summary) != 2 {
		t.Error("2 refs expected, but got:", summary)
	}

	para.LastPair().AppendBang(intNode)
	summary = slices.Collect(collect.ReferenceSeq(blocks))
	if cnt := len(summary); cnt != 3 {
		t.Error("Ref count does not work. Expected: 3, got", summary)
	}

	blocks = zsx.MakeBlock(zsx.MakePara(zsx.MakeEmbed(nil, parseRef("12345678901234"), "", nil)))
	summary = slices.Collect(collect.ReferenceSeq(blocks))
	if len(summary) != 1 {
		t.Error("Only one image ref expected, but got: ", summary)
	}
}
