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

	"t73f.de/r/webs/aasvg"
	"t73f.de/r/zsc/attrs"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"

	"zettelstore.de/z/internal/ast"
)

func init() {
	register(&Info{
		Name:          meta.ValueSyntaxDraw,
		AltNames:      []string{},
		IsASTParser:   true,
		IsTextFormat:  true,
		IsImageFormat: false,
		Parse:         parseDraw,
	})
}

const (
	defaultFont   = ""
	defaultScaleX = 10
	defaultScaleY = 20
)

func parseDraw(inp *input.Input, m *meta.Meta, _ string) ast.BlockSlice {
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
		return ast.BlockSlice{ast.CreateParaNode(canvasErrMsg(err)...)}
	}
	svg := aasvg.CanvasToSVG(canvas, string(font), int(scaleX), int(scaleY))
	if len(svg) == 0 {
		return ast.BlockSlice{ast.CreateParaNode(noSVGErrMsg()...)}
	}
	return ast.BlockSlice{&ast.BLOBNode{
		Description: ParseDescription(m),
		Syntax:      meta.ValueSyntaxSVG,
		Blob:        svg,
	}}
}

// ParseDrawBlock parses the content of an eval verbatim node into an SVG image BLOB.
func ParseDrawBlock(vn *ast.VerbatimNode) ast.BlockNode {
	font := defaultFont
	if val, found := vn.Attrs.Get("font"); found {
		font = val
	}
	scaleX := getScale(vn.Attrs, "x-scale", defaultScaleX)
	scaleY := getScale(vn.Attrs, "y-scale", defaultScaleY)

	canvas, err := aasvg.NewCanvas(vn.Content)
	if err != nil {
		return ast.CreateParaNode(canvasErrMsg(err)...)
	}
	if scaleX < 1 || 1000000 < scaleX {
		scaleX = defaultScaleX
	}
	if scaleY < 1 || 1000000 < scaleY {
		scaleY = defaultScaleY
	}
	svg := aasvg.CanvasToSVG(canvas, font, scaleX, scaleY)
	if len(svg) == 0 {
		return ast.CreateParaNode(noSVGErrMsg()...)
	}
	return &ast.BLOBNode{
		Description: nil, // TODO: look for attribute "summary" / "title"
		Syntax:      meta.ValueSyntaxSVG,
		Blob:        svg,
	}
}

func getScale(a attrs.Attributes, key string, defVal int) int {
	if val, found := a.Get(key); found {
		if n, err := strconv.Atoi(val); err == nil && 0 < n && n < 100000 {
			return n
		}
	}
	return defVal
}

func canvasErrMsg(err error) ast.InlineSlice {
	return ast.InlineSlice{&ast.TextNode{Text: "Error: " + err.Error()}}
}

func noSVGErrMsg() ast.InlineSlice {
	return ast.InlineSlice{&ast.TextNode{Text: "NO IMAGE"}}
}
