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
	cmark := meta.ValueSyntaxCMark
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
			exp: `(BLOCK (UNORDERED () (ITEM () (PARA (TEXT "T1"))) (ITEM () (PARA (TEXT "T2")))))`},
		{name: "strikethrough-simple-no-cmark",
			src:    "~~Hi~~ Hello, ~there~ world!",
			syntax: cmark,
			exp:    `(BLOCK (PARA (TEXT "~~Hi~~ Hello, ~there~ world") (TEXT "!")))`},
		{name: "strikethrough-simple-491",
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
		{name: "table-simple-no-cmark",
			src:    "| foo | bar |\n| --- | --- |\n| baz | bim |",
			syntax: cmark,
			exp:    `(BLOCK (PARA (TEXT "| foo | bar |") (SOFT) (TEXT "| --- | --- |") (SOFT) (TEXT "| baz | bim |")))`},
		{name: "table-simple-198",
			src:    "| foo | bar |\n| --- | --- |\n| baz | bim |",
			syntax: emark,
			exp:    `(BLOCK (TABLE () (ROW () (CELL () (TEXT "foo")) (CELL () (TEXT "bar"))) (ROW () (CELL () (TEXT "baz")) (CELL () (TEXT "bim")))))`},
		{name: "table-align-199",
			src:    "| abc | defghi |\n:-: | -----------:\nbar | baz",
			syntax: emark,
			exp:    `(BLOCK (TABLE () (ROW () (CELL ((align . "center")) (TEXT "abc")) (CELL ((align . "right")) (TEXT "defghi"))) (ROW () (CELL ((align . "center")) (TEXT "bar")) (CELL ((align . "right")) (TEXT "baz")))))`},
		{name: "table-escape-pipe-200",
			src:    "| f\\|oo  |\n| ------ |\n| b `\\|` az |\n| b **\\|** im |",
			syntax: emark,
			exp:    `(BLOCK (TABLE () (ROW () (CELL () (TEXT "f|oo"))) (ROW () (CELL () (TEXT "b ") (LITERAL-CODE () "|") (TEXT " az"))) (ROW () (CELL () (TEXT "b ") (FORMAT-STRONG () (TEXT "|")) (TEXT " im")))))`},
		{name: "table-broken-block-201",
			src:    "| abc | def |\n| --- | --- |\n| bar | baz |\n> bar",
			syntax: emark,
			exp:    `(BLOCK (TABLE () (ROW () (CELL () (TEXT "abc")) (CELL () (TEXT "def"))) (ROW () (CELL () (TEXT "bar")) (CELL () (TEXT "baz")))) (QUOTATION () (ITEM () (PARA (TEXT "bar")))))`},
		{name: "table-broken-para-202",
			src:    "| abc | def |\n| --- | --- |\n| bar | baz |\nbar\n\nbar",
			syntax: emark,
			exp:    `(BLOCK (TABLE () (ROW () (CELL () (TEXT "abc")) (CELL () (TEXT "def"))) (ROW () (CELL () (TEXT "bar")) (CELL () (TEXT "baz"))) (ROW () (CELL () (TEXT "bar")) (CELL ()))) (PARA (TEXT "bar")))`},
		{name: "table-header-nomatch-delim-203",
			src:    "| abc | def |\n| --- |\n| bar |",
			syntax: emark,
			exp:    `(BLOCK (PARA (TEXT "| abc | def |") (SOFT) (TEXT "| --- |") (SOFT) (TEXT "| bar |")))`},
		{name: "table-ignore-delim-excess-204",
			src:    "| abc | def |\n| --- | --- |\n| bar |\n| bar | baz | boo |",
			syntax: emark,
			exp:    `(BLOCK (TABLE () (ROW () (CELL () (TEXT "abc")) (CELL () (TEXT "def"))) (ROW () (CELL () (TEXT "bar")) (CELL ())) (ROW () (CELL () (TEXT "bar")) (CELL () (TEXT "baz")))))`},
		{name: "table-nobody-205",
			src:    "| abc | def |\n| --- | --- |",
			syntax: emark,
			exp:    `(BLOCK (TABLE () (ROW () (CELL () (TEXT "abc")) (CELL () (TEXT "def")))))`},
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
