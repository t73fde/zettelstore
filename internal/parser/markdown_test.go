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
	cmark := meta.ValueSyntaxEMark
	emark := meta.ValueSyntaxEMark
	testcases := []struct {
		name   string
		src    string
		syntax string
		exp    string
	}{
		{"empty", "", "", "(BLOCK)"},
		{name: "simple-list",
			src: "*   T1\n*   T2",
			exp: `(BLOCK (UNORDERED () (BLOCK (PARA (TEXT "T1"))) (BLOCK (PARA (TEXT "T2")))))`},
		{name: "strikethroug-simple-491",
			src:    "~~Hi~~ Hello, ~there~ world!",
			syntax: emark,
			exp:    `(BLOCK (PARA (FORMAT-DELETE () (TEXT "Hi")) (TEXT " Hello, ") (FORMAT-DELETE () (TEXT "there")) (TEXT " world") (TEXT "!")))`},
		{name: "striketrough-paragraph-492",
			src:    "This ~~has a\n\nnew paragraph~~.",
			syntax: emark,
			exp:    `(BLOCK (PARA (TEXT "This ~~") (TEXT "has a")) (PARA (TEXT "new paragraph~~") (TEXT ".")))`},
		{name: "striketrough-tildes-493",
			src:    "This will ~~~not~~~ strike.",
			syntax: emark,
			exp:    `(BLOCK (PARA (TEXT "This will ~~~not~~") (TEXT "~ strike.")))`},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			inp := input.NewInput([]byte(tc.src))
			syntaxes := []string{tc.syntax}
			if syntaxes[0] == "" {
				syntaxes = []string{cmark, emark}
			}
			for _, syntax := range syntaxes {
				pinfo := parser.Get(syntax)
				node := pinfo.Parse(inp, nil, syntax, nil)
				if got := node.String(); got != tc.exp {
					t.Errorf("\nExp: %s\nGot: %s", tc.exp, got)
				}
			}
		})
	}
}
