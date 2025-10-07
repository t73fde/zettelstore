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

// blob provides a parser of binary data.

import (
	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx"
	"t73f.de/r/zsx/input"
)

func init() {
	register(&Info{
		Name:          meta.ValueSyntaxGif,
		AltNames:      nil,
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: true,
		Parse:         parseBlob,
	})
	register(&Info{
		Name:          meta.ValueSyntaxJPEG,
		AltNames:      []string{meta.ValueSyntaxJPG},
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: true,
		Parse:         parseBlob,
	})
	register(&Info{
		Name:          meta.ValueSyntaxPNG,
		AltNames:      nil,
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: true,
		Parse:         parseBlob,
	})
	register(&Info{
		Name:          meta.ValueSyntaxWebp,
		AltNames:      nil,
		IsASTParser:   false,
		IsTextFormat:  false,
		IsImageFormat: true,
		Parse:         parseBlob,
	})
}

func parseBlob(inp *input.Input, m *meta.Meta, syntax string) *sx.Pair {
	if p := Get(syntax); p != nil {
		syntax = p.Name
	}
	return zsx.MakeBlock(zsx.MakeBLOB(nil, syntax, inp.Src, ParseDescription(m)))
}
