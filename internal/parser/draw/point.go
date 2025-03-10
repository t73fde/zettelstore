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

import "fmt"

// A renderHint suggests ways the SVG renderer may appropriately represent this point.
type renderHint uint8

const (
	_             renderHint = iota
	roundedCorner            // the renderer should smooth corners on this path.
	startMarker              // this point should have an SVG marker-start attribute.
	endMarker                // this point should have an SVG marker-end attribute.
	tick                     // the renderer should mark a tick in the path at this point.
	dot                      // the renderer should insert a filled dot in the path at this point.
)

// A point is an X,Y coordinate in the diagram's grid. The grid represents (0, 0) as the top-left
// of the diagram. The point also provides hints to the renderer as to how it should be interpreted.
type point struct {
	x, y int
	hint renderHint
}

// String implements fmt.Stringer on Point.
func (p point) String() string { return fmt.Sprintf("(%d,%d)", p.x, p.y) }

// isHorizontal returns true if p1 and p2 are horizontally aligned.
func isHorizontal(p1, p2 point) bool {
	d := p1.x - p2.x
	return d <= 1 && d >= -1 && p1.y == p2.y
}

// isVertical returns true if p1 and p2 are vertically aligned.
func isVertical(p1, p2 point) bool {
	d := p1.y - p2.y
	return d <= 1 && d >= -1 && p1.x == p2.x
}

// The following functions return true when the diagonals are connected in various compass directions.
func isDiagonalSE(p1, p2 point) bool { return p1.x-p2.x == -1 && p1.y-p2.y == -1 }
func isDiagonalSW(p1, p2 point) bool { return p1.x-p2.x == 1 && p1.y-p2.y == -1 }
func isDiagonalNW(p1, p2 point) bool { return p1.x-p2.x == 1 && p1.y-p2.y == 1 }
func isDiagonalNE(p1, p2 point) bool { return p1.x-p2.x == -1 && p1.y-p2.y == 1 }
