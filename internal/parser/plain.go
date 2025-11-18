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
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"
	"t73f.de/r/zsx/input"
)

func parsePlain(inp *input.Input, _ *meta.Meta, syntax string, alst *sx.Pair) *sx.Pair {
	result := sz.ParsePlainBlocks(inp, syntax)
	if syntax == meta.ValueSyntaxHTML && alst.Assoc(SymAllowHTML) == nil {
		zsx.WalkIt(removeHTMLVisitor{}, result, nil)
	}
	return result
}

type removeHTMLVisitor struct{}

func (removeHTMLVisitor) VisitItBefore(node *sx.Pair, _ *sx.Pair) bool {
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol && zsx.SymVerbatimHTML.IsEqualSymbol(sym) {
		node.SetCar(zsx.SymVerbatimCode)
		return true
	}
	return false
}
func (removeHTMLVisitor) VisitItAfter(*sx.Pair, *sx.Pair) {}

func parsePlainSVG(inp *input.Input, _ *meta.Meta, syntax string, _ *sx.Pair) *sx.Pair {
	is := parseSVGInlines(inp, syntax)
	if is == nil {
		return zsx.MakeBlock()
	}
	return zsx.MakeBlock(zsx.MakeParaList(is))
}

func parseSVGInlines(inp *input.Input, syntax string) *sx.Pair {
	svgSrc := scanSVG(inp)
	if svgSrc == "" {
		return nil
	}
	return sx.Cons(zsx.MakeEmbedBLOBuncode(nil, syntax, svgSrc, nil), sx.Nil())
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

func parsePlainSxn(inp *input.Input, _ *meta.Meta, syntax string, _ *sx.Pair) *sx.Pair {
	rd := sxreader.MakeReader(bytes.NewReader(inp.Src))
	_, err := rd.ReadAll()

	var blocks sx.ListBuilder
	blocks.Add(zsx.MakeVerbatim(
		zsx.SymVerbatimCode,
		sx.Cons(sx.Cons(sx.MakeString(""), sx.MakeString(syntax)), sx.Nil()),
		string(inp.ScanLineContent()),
	))
	if err != nil {
		blocks.Add(zsx.MakeParaList(sx.Cons(zsx.MakeText(err.Error()), sx.Nil())))
	}
	return zsx.MakeBlockList(blocks.List())
}
