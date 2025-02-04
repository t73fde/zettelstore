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

// Package blob provides a parser of binary data.
package blob

import (
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/parser"
)

func init() {
	parser.Register(&parser.Info{
		Name:          meta.SyntaxGif,
		AltNames:      nil,
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: true,
		ParseBlocks:   parseBlocks,
	})
	parser.Register(&parser.Info{
		Name:          meta.SyntaxJPEG,
		AltNames:      []string{meta.SyntaxJPG},
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: true,
		ParseBlocks:   parseBlocks,
	})
	parser.Register(&parser.Info{
		Name:          meta.SyntaxPNG,
		AltNames:      nil,
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: true,
		ParseBlocks:   parseBlocks,
	})
	parser.Register(&parser.Info{
		Name:          meta.SyntaxWebp,
		AltNames:      nil,
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: true,
		ParseBlocks:   parseBlocks,
	})
}

func parseBlocks(inp *input.Input, m *meta.Meta, syntax string) ast.BlockSlice {
	if p := parser.Get(syntax); p != nil {
		syntax = p.Name
	}
	return ast.BlockSlice{&ast.BLOBNode{
		Description: parser.ParseDescription(m),
		Syntax:      syntax,
		Blob:        []byte(inp.Src),
	}}
}
