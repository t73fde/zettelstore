//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// This file was originally created by the ASCIIToSVG contributors under an MIT
// license, but later changed to fulfil the needs of Zettelstore. The following
// statements affects the original code as found on
// https://github.com/asciitosvg/asciitosvg (Commit:
// ca82a5ce41e2190a05e07af6e8b3ea4e3256a283, 2020-11-20):
//
// Copyright 2012 - 2018 The ASCIIToSVG Contributors
// All rights reserved.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package draw

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"zettelstore.de/z/strfun"
)

// canvasToSVG renders the supplied asciitosvg.Canvas to SVG, based on the supplied options.
func canvasToSVG(c *canvas, font string, scaleX, scaleY int) []byte {
	if len(c.objects()) == 0 {
		return nil
	}
	if font == "" {
		font = "monospace"
	}

	var b bytes.Buffer
	fmt.Fprintf(&b,
		`<svg class="zs-draw" width="%d" height="%d" version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">`,
		(c.size().X+1)*scaleX, (c.size().Y+1)*scaleY)
	writeMarkerDefs(&b, c, scaleX, scaleY)

	// 3 passes, first closed paths, then open paths, then text.
	writeClosedPaths(&b, c, scaleX, scaleY)
	writeOpenPaths(&b, c, scaleX, scaleY)
	writeTexts(&b, c, escape(font), scaleX, scaleY)

	io.WriteString(&b, "</svg>")
	return b.Bytes()
}

const (
	nameStartMarker = "iPointer"
	nameEndMarker   = "Pointer"
)

func writeMarkerDefs(w io.Writer, c *canvas, scaleX, scaleY int) {
	const markerTag = `<marker id="%s" viewBox="0 0 10 10" refX="5" refY="5" markerUnits="strokeWidth" markerWidth="%g" markerHeight="%g" orient="auto"><path d="%s" /></marker>`
	x := float64(scaleX) / 2
	y := float64(scaleY) / 2
	if c.hasStartMarker {
		fmt.Fprintf(w, markerTag, nameStartMarker, x, y, "M 10 0 L 10 10 L 0 5 z")
	}
	if c.hasEndMarker {
		fmt.Fprintf(w, markerTag, nameEndMarker, x, y, "M 0 0 L 10 5 L 0 10 z")
	}
}

const pathTag = `<path id="%s%d" %sd="%s" />`

func writeClosedPaths(w io.Writer, c *canvas, scaleX, scaleY int) {
	first := true
	for i, obj := range c.objects() {
		if !obj.isClosedPath() {
			continue
		}
		if first {
			io.WriteString(w, `<g id="closed" stroke="#000" stroke-width="2" fill="none">`)
			first = false
		}
		opts := ""
		if obj.IsDashed() {
			opts = `stroke-dasharray="5 5" `
		}

		fmt.Fprintf(w, pathTag, "closed", i, opts, flatten(obj.Points(), scaleX, scaleY)+"Z")
	}
	if !first {
		io.WriteString(w, "</g>")
	}
}

func writeOpenPaths(w io.Writer, c *canvas, scaleX, scaleY int) {
	const optStartMarker = `marker-start="url(#` + nameStartMarker + `)" `
	const optEndMarker = `marker-end="url(#` + nameEndMarker + `)" `

	first := true
	for i, obj := range c.objects() {
		if !obj.isOpenPath() {
			continue
		}
		if first {
			io.WriteString(w, `<g id="lines" stroke="#000" stroke-width="2" fill="none">`)
			first = false
		}
		points := obj.Points()
		for _, p := range points {
			switch p.hint {
			case dot:
				sp := scale(p, scaleX, scaleY)
				fmt.Fprintf(w, `<circle cx="%g" cy="%g" r="3" fill="#000" />`, sp.X, sp.Y)
			case tick:
				sp := scale(p, scaleX, scaleY)
				const tickLine = `<line x1="%g" y1="%g" x2="%g" y2="%g" stroke-width="2" />`
				fmt.Fprintf(w, tickLine, sp.X-4, sp.Y-4, sp.X+4, sp.Y+4)
				fmt.Fprintf(w, tickLine, sp.X+4, sp.Y-4, sp.X-4, sp.Y+4)
			}
		}

		opts := ""
		if obj.IsDashed() {
			opts += `stroke-dasharray="5 5" `
		}
		if points[0].hint == startMarker {
			opts += optStartMarker
		}
		if points[len(points)-1].hint == endMarker {
			opts += optEndMarker
		}
		fmt.Fprintf(w, pathTag, "open", i, opts, flatten(points, scaleX, scaleY))
	}
	if !first {
		io.WriteString(w, "</g>")
	}
}

func writeTexts(w io.Writer, c *canvas, font string, scaleX, scaleY int) {
	fontSize := float64(scaleY) * 0.75
	deltaX := float64(scaleX) / 4
	deltaY := float64(scaleY) / 4
	first := true
	for i, obj := range c.objects() {
		if !obj.isJustText() {
			continue
		}
		if first {
			fmt.Fprintf(w, `<g id="text" stroke="none" style="font-family:%s;font-size:%gpx">`, font, fontSize)
			first = false
		}

		text := string(obj.Text())
		sp := scale(obj.Points()[0], scaleX, scaleY)
		fmt.Fprintf(w,
			`<text id="obj%d" x="%g" y="%g">%s</text>`,
			i, sp.X-deltaX, sp.Y+deltaY, escape(text))
	}
	if !first {
		io.WriteString(w, "</g>")
	}
}

func escape(s string) string {
	var sb strings.Builder
	strfun.XMLEscape(&sb, s)
	return sb.String()
}

type scaledPoint struct {
	X    float64
	Y    float64
	Hint renderHint
}

func scale(p point, scaleX, scaleY int) scaledPoint {
	return scaledPoint{
		X:    (float64(p.x) + .5) * float64(scaleX),
		Y:    (float64(p.y) + .5) * float64(scaleY),
		Hint: p.hint,
	}
}

func flatten(points []point, scaleX, scaleY int) string {
	var result strings.Builder

	// Scaled start point, and previous point (which is always initially the start point).
	sp := scale(points[0], scaleX, scaleY)
	pp := sp

	for i, cp := range points {
		p := scale(cp, scaleX, scaleY)

		// Our start point is represented by a single moveto command (unless the start point
		// is a rounded corner) as the shape will be closed with the Z command automatically
		// if we have a closed polygon. If our start point is a rounded corner, we have to go
		// ahead and draw that curve.
		if i == 0 {
			if cp.hint == roundedCorner {
				fmt.Fprintf(&result, "M %g %g Q %g %g %g %g ", p.X, p.Y+10, p.X, p.Y, p.X+10, p.Y)
				continue
			}

			fmt.Fprintf(&result, "M %g %g ", p.X, p.Y)
			continue
		}

		// If this point has a rounded corner, we need to calculate the curve. This algorithm
		// only works when the shapes are drawn in a clockwise manner.
		if cp.hint == roundedCorner {
			// The control point is always the original corner.
			cx := p.X
			cy := p.Y

			sx, sy, ex, ey := 0., 0., 0., 0.

			// We need to know the next point to determine which way to turn.
			var np scaledPoint
			if i == len(points)-1 {
				np = sp
			} else {
				np = scale(points[i+1], scaleX, scaleY)
			}

			if pp.X == p.X {
				// If we're on the same vertical axis, our starting X coordinate is
				// the same as the control point coordinate
				sx = p.X

				// Offset start point from control point in the proper direction.
				if pp.Y < p.Y {
					sy = p.Y - 10
				} else {
					sy = p.Y + 10
				}

				ey = p.Y
				// Offset endpoint from control point in the proper direction.
				if np.X < p.X {
					ex = p.X - 10
				} else {
					ex = p.X + 10
				}
			} else if pp.Y == p.Y {
				// Horizontal decisions mirror vertical's above.
				sy = p.Y
				if pp.X < p.X {
					sx = p.X - 10
				} else {
					sx = p.X + 10
				}
				ex = p.X
				if np.Y <= p.Y {
					ey = p.Y - 10
				} else {
					ey = p.Y + 10
				}
			}

			fmt.Fprintf(&result, "L %g %g Q %g %g %g %g ", sx, sy, cx, cy, ex, ey)
		} else {
			// Just draw a straight line.
			fmt.Fprintf(&result, "L %g %g ", p.X, p.Y)
		}

		pp = p
	}
	return result.String()
}
