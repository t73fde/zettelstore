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

package plain_test

import (
	"testing"

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"
	"zettelstore.de/z/config"
	"zettelstore.de/z/encoder/szenc"
	"zettelstore.de/z/parser"
)

func TestParseSVG(t *testing.T) {
	testCases := []struct {
		name string
		src  string
		exp  string
	}{
		{"common", " <svg bla", "(BLOCK (PARA (EMBED-BLOB () \"svg\" \"<svg bla\")))"},
		{"inkscape", "<svg\nbla", "(BLOCK (PARA (EMBED-BLOB () \"svg\" \"<svg\\nbla\")))"},
		{"selfmade", "<svg>", "(BLOCK (PARA (EMBED-BLOB () \"svg\" \"<svg>\")))"},
		{"error", "<svgbla", "(BLOCK)"},
		{"error-", "<svg-bla", "(BLOCK)"},
		{"error#", "<svg2bla", "(BLOCK)"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inp := input.NewInput([]byte(tc.src))
			bs := parser.ParseBlocks(inp, nil, meta.SyntaxSVG, config.NoHTML)
			trans := szenc.NewTransformer()
			lst := trans.GetSz(&bs)
			if got := lst.String(); tc.exp != got {
				t.Errorf("\nexp: %q\ngot: %q", tc.exp, got)
			}
		})
	}
}
