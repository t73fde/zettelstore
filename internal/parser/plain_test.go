//-----------------------------------------------------------------------------
// Copyright (c) 2024-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2024-present Detlef Stern
//-----------------------------------------------------------------------------

package parser_test

import (
	"testing"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/parser"
)

func TestParsePlain(t *testing.T) {
	testCases := []struct {
		name      string
		syntax    string
		src       string
		allowHTML bool
		exp       string
	}{
		{name: "empty-default", syntax: "",
			src: "",
			exp: "(BLOCK (VERBATIM-CODE ((\"\" . \"\")) \"\"))"},
		{name: "empty-html", syntax: meta.ValueSyntaxHTML,
			src: "",
			exp: "(BLOCK (VERBATIM-CODE ((\"\" . \"html\")) \"\"))"},
		{name: "empty-html-allow", syntax: meta.ValueSyntaxHTML, allowHTML: true,
			src: "",
			exp: "(BLOCK (VERBATIM-HTML ((\"\" . \"html\")) \"\"))"},
		{name: "empty-sxn", syntax: meta.ValueSyntaxSxn,
			src: "",
			exp: "(BLOCK (VERBATIM-CODE ((\"\" . \"sxn\")) \"\"))"},
		{name: "valid-sxn", syntax: meta.ValueSyntaxSxn,
			src: "(+ 3 4)",
			exp: "(BLOCK (VERBATIM-CODE ((\"\" . \"sxn\")) \"(+ 3 4)\"))"},
		{name: "invalid-sxn", syntax: meta.ValueSyntaxSxn,
			src: "(+ 3 4",
			exp: "(BLOCK (VERBATIM-CODE ((\"\" . \"sxn\")) \"(+ 3 4\") (PARA (TEXT \"ReaderError 1-6: unexpected EOF\")))"},

		{name: "svg-common", syntax: meta.ValueSyntaxSVG,
			src: " <svg bla",
			exp: "(BLOCK (PARA (EMBED-BLOB () \"svg\" \"<svg bla\")))"},
		{name: "svg-inkscape", syntax: meta.ValueSyntaxSVG,
			src: "<svg\nbla",
			exp: "(BLOCK (PARA (EMBED-BLOB () \"svg\" \"<svg\\nbla\")))"},
		{name: "svg-selfmade", syntax: meta.ValueSyntaxSVG,
			src: "<svg>",
			exp: "(BLOCK (PARA (EMBED-BLOB () \"svg\" \"<svg>\")))"},
		{name: "svg-error", syntax: meta.ValueSyntaxSVG,
			src: "<svgbla",
			exp: "(BLOCK)"},
		{name: "svg-error-", syntax: meta.ValueSyntaxSVG,
			src: "<svg-bla",
			exp: "(BLOCK)"},
		{name: "svg-error#", syntax: meta.ValueSyntaxSVG,
			src: "<svg2bla",
			exp: "(BLOCK)"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inp := input.NewInput([]byte(tc.src))
			alst := sx.Nil()
			if tc.allowHTML {
				alst = alst.Cons(sx.Cons(parser.SymAllowHTML, nil))
			}
			node := parser.Parse(inp, nil, tc.syntax, alst)
			if got := node.String(); tc.exp != got {
				t.Errorf("\nexp: %q\ngot: %q", tc.exp, got)
			}
		})
	}
}
