//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package parser

// draw provides a parser to create SVG from ASCII drawing.

import (
	"strconv"

	"t73f.de/r/sx"
	"t73f.de/r/webs/aasvg"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx"
	"t73f.de/r/zsx/input"
)

const (
	defaultFont   = ""
	defaultScaleX = 10
	defaultScaleY = 20
)

func parseDraw(inp *input.Input, m *meta.Meta, _ string, _ *sx.Pair) *sx.Pair {
	font := m.GetDefault("font", defaultFont)
	scaleX := m.GetNumber("x-scale", defaultScaleX)
	scaleY := m.GetNumber("y-scale", defaultScaleY)
	if scaleX < 1 || 1000000 < scaleX {
		scaleX = defaultScaleX
	}
	if scaleY < 1 || 1000000 < scaleY {
		scaleY = defaultScaleY
	}

	canvas, err := aasvg.NewCanvas(inp.Src[inp.Pos:])
	if err != nil {
		return zsx.MakeBlock(zsx.MakeParaList(canvasErrMsg(err)))
	}
	svg := aasvg.CanvasToSVG(canvas, string(font), int(scaleX), int(scaleY))
	if len(svg) == 0 {
		return zsx.MakeBlock(zsx.MakeParaList(noSVGErrMsg()))
	}
	return zsx.MakeBlock(zsx.MakeBLOB(nil, meta.ValueSyntaxSVG, svg, ParseDescription(m)))
}

// ParseDrawBlock parses the content of an eval verbatim node into an SVG image BLOB.
func ParseDrawBlock(attrs *sx.Pair, content []byte) *sx.Pair {
	a := zsx.GetAttributes(attrs)
	font := defaultFont
	if val, found := a.Get("font"); found {
		font = val
	}
	scaleX := getScaleAST(a, "x-scale", defaultScaleX)
	scaleY := getScaleAST(a, "y-scale", defaultScaleY)

	canvas, err := aasvg.NewCanvas(content)
	if err != nil {
		return zsx.MakePara(zsx.MakeText("Error: " + err.Error()))
	}
	if scaleX < 1 || 1000000 < scaleX {
		scaleX = defaultScaleX
	}
	if scaleY < 1 || 1000000 < scaleY {
		scaleY = defaultScaleY
	}
	svg := aasvg.CanvasToSVG(canvas, font, scaleX, scaleY)
	if len(svg) == 0 {
		return zsx.MakePara(zsx.MakeText("NO IMAGE"))
	}
	return zsx.MakeBLOB(
		nil,
		meta.ValueSyntaxSVG,
		svg,
		nil, // TODO: look for attribute "summary" / "title" for a description.
	)
}

func getScaleAST(a zsx.Attributes, key string, defVal int) int {
	if val, found := a.Get(key); found {
		if n, err := strconv.Atoi(val); err == nil && 0 < n && n < 100000 {
			return n
		}
	}
	return defVal
}

func canvasErrMsg(err error) *sx.Pair {
	return sx.Cons(zsx.MakeText("Error: "+err.Error()), sx.Nil())
}

func noSVGErrMsg() *sx.Pair {
	return sx.Cons(zsx.MakeText("NO IMAGE"), sx.Nil())
}
