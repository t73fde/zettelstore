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

import (
	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx/input"
)

// none provides a none-parser, e.g. for zettel with just metadata.

func init() {
	register(&Info{
		Name:          meta.ValueSyntaxNone,
		AltNames:      []string{},
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: false,
		Parse: func(inp *input.Input, _ *meta.Meta, _ string, _ *sx.Pair) *sx.Pair {
			return sz.ParseNoneBlocks(inp)
		},
	})
}
