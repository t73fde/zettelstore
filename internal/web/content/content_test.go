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

package content_test

import (
	"testing"

	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/web/content"

	_ "zettelstore.de/z/internal/parser/blob"       // Allow to use BLOB parser.
	_ "zettelstore.de/z/internal/parser/draw"       // Allow to use draw parser.
	_ "zettelstore.de/z/internal/parser/markdown"   // Allow to use markdown parser.
	_ "zettelstore.de/z/internal/parser/none"       // Allow to use none parser.
	_ "zettelstore.de/z/internal/parser/plain"      // Allow to use plain parser.
	_ "zettelstore.de/z/internal/parser/zettelmark" // Allow to use zettelmark parser.
)

func TestSupportedSyntax(t *testing.T) {
	for _, syntax := range parser.GetSyntaxes() {
		mt := content.MIMEFromSyntax(syntax)
		if mt == content.UnknownMIME {
			t.Errorf("No MIME type registered for syntax %q", syntax)
			continue
		}

		newSyntax := content.SyntaxFromMIME(mt, nil)
		pinfo := parser.Get(newSyntax)
		if pinfo == nil {
			t.Errorf("MIME type for syntax %q is %q, but this has no corresponding syntax", syntax, mt)
			continue
		}
	}
}
