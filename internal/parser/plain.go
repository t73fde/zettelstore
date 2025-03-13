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

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxreader"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"
	"t73f.de/r/zsc/sz"
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

func parsePlain(inp *input.Input, _ *meta.Meta, syntax string) *sx.Pair {
	return doParsePlain(inp, syntax, sz.SymVerbatimCode)
}
func parsePlainHTML(inp *input.Input, _ *meta.Meta, syntax string) *sx.Pair {
	return doParsePlain(inp, syntax, sz.SymVerbatimHTML)
}
func doParsePlain(inp *input.Input, syntax string, kind *sx.Symbol) *sx.Pair {
	return sz.MakeBlock(sz.MakeVerbatim(
		kind,
		sx.Cons(sx.Cons(sx.MakeString(""), sx.MakeString(syntax)), sx.Nil()),
		string(inp.ScanLineContent()),
	))
}

func parsePlainSVG(inp *input.Input, _ *meta.Meta, syntax string) *sx.Pair {
	is := parseSVGInlines(inp, syntax)
	if is == nil {
		return nil
	}
	return sz.MakeBlock(sz.MakePara(is))
}

func parseSVGInlines(inp *input.Input, syntax string) *sx.Pair {
	svgSrc := scanSVG(inp)
	if svgSrc == "" {
		return nil
	}
	return sx.Cons(sz.MakeEmbedBLOB(nil, syntax, svgSrc, nil), sx.Nil())
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

func parsePlainSxn(inp *input.Input, _ *meta.Meta, syntax string) *sx.Pair {
	rd := sxreader.MakeReader(bytes.NewReader(inp.Src))
	_, err := rd.ReadAll()

	var blocks sx.ListBuilder
	blocks.Add(sz.MakeVerbatim(
		sz.SymVerbatimCode,
		sx.Cons(sx.Cons(sx.MakeString(""), sx.MakeString(syntax)), sx.Nil()),
		string(inp.ScanLineContent()),
	))
	if err != nil {
		blocks.Add(sz.MakePara(sx.Cons(sz.MakeText(err.Error()), sx.Nil())))
	}
	return sz.MakeBlockList(blocks.List())
}
