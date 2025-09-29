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
	"strings"
	"testing"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxreader"
	"zettelstore.de/z/internal/parser"
)

func TestCleaner(t *testing.T) {
	var testcases = []struct {
		name      string
		src       string
		allowHTML bool
		exp       string
	}{
		{name: "nil", src: "()", exp: "()"},

		{name: "simple heading",
			src: "(HEADING 1 () \"\" \"\" (TEXT \"Heading\"))",
			exp: "(HEADING 1 () \"heading\" \"heading\" (TEXT \"Heading\"))"},
		{name: "same simple heading",
			src: "(BLOCK (HEADING 1 () \"\" \"\" (TEXT \"Heading\")) (HEADING 1 () \"\" \"\" (TEXT \"Heading\")))",
			exp: "(BLOCK (HEADING 1 () \"heading\" \"heading\" (TEXT \"Heading\")) (HEADING 1 () \"heading\" \"heading-1\" (TEXT \"Heading\")))"},

		{name: "simple mark, no text",
			src: "(MARK \"m\" \"\" \"\")",
			exp: "(MARK \"m\" \"m\" \"m\")"},
		{name: "same simple mark, no text",
			src: "(PARA (MARK \"m\" \"\" \"\") (MARK \"m\" \"\" \"\"))",
			exp: "(PARA (MARK \"m\" \"m\" \"m\") (MARK \"m\" \"m\" \"m-1\"))"},
		{name: "mark before heading",
			src: "(BLOCK (HEADING 1 () \"\" \"\" (TEXT \"x\")) (PARA (MARK \"x\" \"\" \"\")))",
			exp: "(BLOCK (HEADING 1 () \"x\" \"x\" (TEXT \"x\")) (PARA (MARK \"x\" \"x\" \"x-1\")))"},

		{name: "remove-html-0",
			src: "(BLOCK (VERBATIM-HTML () \"<h1>Heading</h1>\"))",
			exp: "(BLOCK)"},
		{name: "remove-html-0-1",
			src: "(BLOCK (VERBATIM-HTML () \"<h1>Heading</h1>\") (PARA (TEXT \"ABC\")))",
			exp: "(BLOCK (PARA (TEXT \"ABC\")))"},
		{name: "remove-html-1-0",
			src: "(BLOCK (PARA (TEXT \"ABC\")) (VERBATIM-HTML () \"<h1>Heading</h1>\"))",
			exp: "(BLOCK (PARA (TEXT \"ABC\")))"},

		{name: "allow HTML", allowHTML: true,
			src: "(BLOCK (VERBATIM-HTML () \"<h1>Heading</h1>\"))",
			exp: "(BLOCK (VERBATIM-HTML () \"<h1>Heading</h1>\"))"},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			rd := sxreader.MakeReader(strings.NewReader(tc.src))
			obj, err := rd.Read()
			if err != nil {
				t.Error(err)
				return
			}
			node, isPair := sx.GetPair(obj)
			if !isPair {
				t.Error("not a pair:", obj)
			}
			parser.Clean(node, tc.allowHTML)
			if got := node.String(); got != tc.exp {
				t.Errorf("\nexpected: %q\n but got: %q", tc.exp, got)
			}
		})
	}
}
