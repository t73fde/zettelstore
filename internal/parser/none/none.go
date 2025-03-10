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

// Package none provides a none-parser, e.g. for zettel with just metadata.
package none

import (
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/parser"
)

func init() {
	parser.Register(&parser.Info{
		Name:          meta.ValueSyntaxNone,
		AltNames:      []string{},
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: false,
		Parse:         func(*input.Input, *meta.Meta, string) ast.BlockSlice { return nil },
	})
}
