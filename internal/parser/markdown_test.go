//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

package parser_test

import (
	"testing"

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"
	"zettelstore.de/z/internal/parser"
)

func TestMarkdown(t *testing.T) {
	testcases := []struct {
		name string
		src  string
		exp  string
	}{
		{"empty", "", "(BLOCK)"},
		{name: "simple-list",
			src: "*   T1\n*   T2",
			exp: `(BLOCK (UNORDERED () (BLOCK (PARA (TEXT "T1"))) (BLOCK (PARA (TEXT "T2")))))`},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			inp := input.NewInput([]byte(tc.src))
			node := parser.Parse(inp, nil, meta.ValueSyntaxMD, nil)
			if got := node.String(); got != tc.exp {
				t.Errorf("\nExp: %s\nGot: %s", tc.exp, got)
			}
		})
	}
}
