//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package encoder_test

import (
	"testing"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/parser"
)

type blobTestCase struct {
	descr  string
	syntax string
	blob   []byte
	expect expectMap
}

var pngTestCases = []blobTestCase{
	{
		descr:  "Minimal PNG",
		syntax: "png",
		blob: []byte{
			0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
			0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00, 0x3a, 0x7e, 0x9b,
			0x55, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x62, 0x00, 0x00, 0x00,
			0x06, 0x00, 0x03, 0x36, 0x37, 0x7c, 0xa8, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
			0x42, 0x60, 0x82,
		},
		expect: expectMap{
			encoderHTML:  `<p><img alt="Minimal PNG" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAACklEQVR4nGNiAAAABgADNjd8qAAAAABJRU5ErkJggg=="></p>`,
			encoderSz:    `(BLOCK (BLOB () "png" "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAACklEQVR4nGNiAAAABgADNjd8qAAAAABJRU5ErkJggg==" (TEXT "Minimal PNG")))`,
			encoderSHTML: `((p (img ((alt . "Minimal PNG") (src . "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAACklEQVR4nGNiAAAABgADNjd8qAAAAABJRU5ErkJggg==")))))`,
			encoderText:  "",
			encoderZmk:   `%% Unable to display BLOB with description 'Minimal PNG' and syntax 'png'.`,
		},
	},
}

func TestBlob(t *testing.T) {
	m := meta.New(id.Invalid)
	for testNum, tc := range pngTestCases {
		m.Set(meta.KeyTitle, meta.Value(tc.descr))
		inp := input.NewInput(tc.blob)
		node, bs := parser.Parse(inp, m, tc.syntax, config.NoHTML)
		checkEncodings(t, testNum+1000, node, false, tc.descr, tc.expect, "???")
		checkEncodingsAST(t, testNum, bs, false, tc.descr, tc.expect, "???")
	}
}
