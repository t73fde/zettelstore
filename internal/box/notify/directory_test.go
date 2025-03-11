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

package notify

import (
	"testing"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

func TestSeekZid(t *testing.T) {
	testcases := []struct {
		name string
		zid  id.Zid
	}{
		{"", id.Invalid},
		{"1", id.Invalid},
		{"1234567890123", id.Invalid},
		{" 12345678901234", id.Invalid},
		{"12345678901234", id.Zid(12345678901234)},
		{"12345678901234.ext", id.Zid(12345678901234)},
		{"12345678901234 abc.ext", id.Zid(12345678901234)},
		{"12345678901234.abc.ext", id.Zid(12345678901234)},
		{"12345678901234 def", id.Zid(12345678901234)},
	}
	for _, tc := range testcases {
		gotZid := seekZid(tc.name)
		if gotZid != tc.zid {
			t.Errorf("seekZid(%q) == %v, but got %v", tc.name, tc.zid, gotZid)
		}
	}
}

func TestNewExtIsBetter(t *testing.T) {
	extVals := []string{
		// Main Formats
		meta.ValueSyntaxZmk, meta.ValueSyntaxDraw, meta.ValueSyntaxMarkdown, meta.ValueSyntaxMD,
		// Other supported text formats
		meta.ValueSyntaxCSS, meta.ValueSyntaxSxn, meta.ValueSyntaxTxt, meta.ValueSyntaxHTML,
		meta.ValueSyntaxText, meta.ValueSyntaxPlain,
		// Supported text graphics formats
		meta.ValueSyntaxSVG,
		meta.ValueSyntaxNone,
		// Supported binary graphic formats
		meta.ValueSyntaxGif, meta.ValueSyntaxPNG, meta.ValueSyntaxJPEG, meta.ValueSyntaxWebp, meta.ValueSyntaxJPG,

		// Unsupported syntax values
		"gz", "cpp", "tar", "cppc",
	}
	for oldI, oldExt := range extVals {
		for newI, newExt := range extVals {
			if oldI <= newI {
				continue
			}
			if !newExtIsBetter(oldExt, newExt) {
				t.Errorf("newExtIsBetter(%q, %q) == true, but got false", oldExt, newExt)
			}
			if newExtIsBetter(newExt, oldExt) {
				t.Errorf("newExtIsBetter(%q, %q) == false, but got true", newExt, oldExt)
			}
		}
	}
}
