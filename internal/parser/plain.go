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

// plain provides a parser for plain text data.

import (
	"bytes"

	"t73f.de/r/sx/sxreader"
	"t73f.de/r/zsc/attrs"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"

	"zettelstore.de/z/internal/ast"
)

func init() {
	register(&Info{
		Name:          meta.ValueSyntaxTxt,
		AltNames:      []string{meta.ValueSyntaxPlain, meta.ValueSyntaxText},
		IsASTParser:   false,
		IsTextFormat:  true,
		IsImageFormat: false,
		Parse:         parsePlain,
	})
	register(&Info{
		Name:          meta.ValueSyntaxHTML,
		AltNames:      []string{},
		IsASTParser:   false,
		IsTextFormat:  true,
		IsImageFormat: false,
		Parse:         parsePlainHTML,
	})
	register(&Info{
		Name:          meta.ValueSyntaxCSS,
		AltNames:      []string{},
		IsASTParser:   false,
		IsTextFormat:  true,
		IsImageFormat: false,
		Parse:         parsePlain,
	})
	register(&Info{
		Name:          meta.ValueSyntaxSVG,
		AltNames:      []string{},
		IsASTParser:   false,
		IsTextFormat:  true,
		IsImageFormat: true,
		Parse:         parsePlainSVG,
	})
	register(&Info{
		Name:          meta.ValueSyntaxSxn,
		AltNames:      []string{},
		IsASTParser:   false,
		IsTextFormat:  true,
		IsImageFormat: false,
		Parse:         parsePlainSxn,
	})
}

func parsePlain(inp *input.Input, _ *meta.Meta, syntax string) ast.BlockSlice {
	return doParsePlain(inp, syntax, ast.VerbatimCode)
}
func parsePlainHTML(inp *input.Input, _ *meta.Meta, syntax string) ast.BlockSlice {
	return doParsePlain(inp, syntax, ast.VerbatimHTML)
}
func doParsePlain(inp *input.Input, syntax string, kind ast.VerbatimKind) ast.BlockSlice {
	return ast.BlockSlice{
		&ast.VerbatimNode{
			Kind:    kind,
			Attrs:   attrs.Attributes{"": syntax},
			Content: inp.ScanLineContent(),
		},
	}
}

func parsePlainSVG(inp *input.Input, _ *meta.Meta, syntax string) ast.BlockSlice {
	is := parseSVGInlines(inp, syntax)
	if len(is) == 0 {
		return nil
	}
	return ast.BlockSlice{ast.CreateParaNode(is...)}
}

func parseSVGInlines(inp *input.Input, syntax string) ast.InlineSlice {
	svgSrc := scanSVG(inp)
	if svgSrc == "" {
		return nil
	}
	return ast.InlineSlice{&ast.EmbedBLOBNode{
		Blob:   []byte(svgSrc),
		Syntax: syntax,
	}}
}

func scanSVG(inp *input.Input) string {
	inp.SkipSpace()
	pos := inp.Pos
	if !inp.Accept("<svg") {
		return ""
	}
	ch := inp.Ch
	if input.IsSpace(ch) || input.IsEOLEOS(ch) || ch == '>' {
		// TODO: check proper end </svg>
		return string(inp.Src[pos:])
	}
	return ""
}

func parsePlainSxn(inp *input.Input, _ *meta.Meta, syntax string) ast.BlockSlice {
	rd := sxreader.MakeReader(bytes.NewReader(inp.Src))
	_, err := rd.ReadAll()
	result := ast.BlockSlice{
		&ast.VerbatimNode{
			Kind:    ast.VerbatimCode,
			Attrs:   attrs.Attributes{"": syntax},
			Content: inp.ScanLineContent(),
		},
	}
	if err != nil {
		result = append(result, ast.CreateParaNode(&ast.TextNode{
			Text: err.Error(),
		}))
	}
	return result
}
