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

	"zettelstore.de/z/parser"
	"zettelstore.de/z/strfun"
	"zettelstore.de/z/zettel/meta"

	_ "zettelstore.de/z/parser/blob"       // Allow to use BLOB parser.
	_ "zettelstore.de/z/parser/draw"       // Allow to use draw parser.
	_ "zettelstore.de/z/parser/markdown"   // Allow to use markdown parser.
	_ "zettelstore.de/z/parser/none"       // Allow to use none parser.
	_ "zettelstore.de/z/parser/plain"      // Allow to use plain parser.
	_ "zettelstore.de/z/parser/zettelmark" // Allow to use zettelmark parser.
)

func TestParserType(t *testing.T) {
	syntaxSet := strfun.NewSet(parser.GetSyntaxes()...)
	testCases := []struct {
		syntax string
		ast    bool
		image  bool
	}{
		{meta.SyntaxHTML, false, false},
		{meta.SyntaxCSS, false, false},
		{meta.SyntaxDraw, true, false},
		{meta.SyntaxGif, false, true},
		{meta.SyntaxJPEG, false, true},
		{meta.SyntaxJPG, false, true},
		{meta.SyntaxMarkdown, true, false},
		{meta.SyntaxMD, true, false},
		{meta.SyntaxNone, false, false},
		{meta.SyntaxPlain, false, false},
		{meta.SyntaxPNG, false, true},
		{meta.SyntaxSVG, false, true},
		{meta.SyntaxSxn, false, false},
		{meta.SyntaxText, false, false},
		{meta.SyntaxTxt, false, false},
		{meta.SyntaxWebp, false, true},
		{meta.SyntaxZmk, true, false},
	}
	for _, tc := range testCases {
		delete(syntaxSet, tc.syntax)
		if got := parser.IsASTParser(tc.syntax); got != tc.ast {
			t.Errorf("Syntax %q is AST: %v, but got %v", tc.syntax, tc.ast, got)
		}
		if got := parser.IsImageFormat(tc.syntax); got != tc.image {
			t.Errorf("Syntax %q is image: %v, but got %v", tc.syntax, tc.image, got)
		}
	}
	for syntax := range syntaxSet {
		t.Errorf("Forgot to test syntax %q", syntax)
	}
}
