//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

package parser

// zettelmark provides a parser for zettelmarkup.

import (
	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"
	"t73f.de/r/zsc/sz/zmk"
)

func init() {
	register(&Info{
		Name:          meta.ValueSyntaxZmk,
		AltNames:      nil,
		IsASTParser:   true,
		IsTextFormat:  true,
		IsImageFormat: false,
		Parse: func(inp *input.Input, _ *meta.Meta, _ string) *sx.Pair {
			var parser zmk.Parser
			parser.Initialize(inp)
			return parser.Parse()
		},
	})
}
