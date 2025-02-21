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

// Package zettelmark provides a parser for zettelmarkup.
package zettelmark

import (
	"log"

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"
	"t73f.de/r/zsc/sz/zmk"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/ast/sztrans"
	"zettelstore.de/z/parser"
)

func init() {
	parser.Register(&parser.Info{
		Name:          meta.ValueSyntaxZmk,
		AltNames:      nil,
		IsASTParser:   true,
		IsTextFormat:  true,
		IsImageFormat: false,
		Parse:         parseZmkBlocks,
	})
}

func parseZmkBlocks(inp *input.Input, _ *meta.Meta, _ string) ast.BlockSlice {
	if obj := zmk.ParseBlocks(inp); obj != nil {
		bs, err := sztrans.GetBlockSlice(obj)
		if err == nil {
			return bs
		}
		log.Printf("sztrans error: %v, for %v\n", err, obj)
	}
	return nil
}
