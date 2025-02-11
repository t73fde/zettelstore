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

package parser_test

import (
	"testing"

	"t73f.de/r/app/set"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/parser"

	_ "zettelstore.de/z/parser/blob"       // Allow to use BLOB parser.
	_ "zettelstore.de/z/parser/draw"       // Allow to use draw parser.
	_ "zettelstore.de/z/parser/markdown"   // Allow to use markdown parser.
	_ "zettelstore.de/z/parser/none"       // Allow to use none parser.
	_ "zettelstore.de/z/parser/plain"      // Allow to use plain parser.
	_ "zettelstore.de/z/parser/zettelmark" // Allow to use zettelmark parser.
)

func TestParserType(t *testing.T) {
	syntaxSet := set.New(parser.GetSyntaxes()...)
	testCases := []struct {
		syntax string
		ast    bool
		image  bool
	}{
		{meta.ValueSyntaxHTML, false, false},
		{meta.ValueSyntaxCSS, false, false},
		{meta.ValueSyntaxDraw, true, false},
		{meta.ValueSyntaxGif, false, true},
		{meta.ValueSyntaxJPEG, false, true},
		{meta.ValueSyntaxJPG, false, true},
		{meta.ValueSyntaxMarkdown, true, false},
		{meta.ValueSyntaxMD, true, false},
		{meta.ValueSyntaxNone, false, false},
		{meta.ValueSyntaxPlain, false, false},
		{meta.ValueSyntaxPNG, false, true},
		{meta.ValueSyntaxSVG, false, true},
		{meta.ValueSyntaxSxn, false, false},
		{meta.ValueSyntaxText, false, false},
		{meta.ValueSyntaxTxt, false, false},
		{meta.ValueSyntaxWebp, false, true},
		{meta.ValueSyntaxZmk, true, false},
	}
	for _, tc := range testCases {
		syntaxSet.Remove(tc.syntax)
		if got := parser.IsASTParser(tc.syntax); got != tc.ast {
			t.Errorf("Syntax %q is AST: %v, but got %v", tc.syntax, tc.ast, got)
		}
		if got := parser.IsImageFormat(tc.syntax); got != tc.image {
			t.Errorf("Syntax %q is image: %v, but got %v", tc.syntax, tc.image, got)
		}
	}
	for syntax := range syntaxSet.Values() {
		t.Errorf("Forgot to test syntax %q", syntax)
	}
}
