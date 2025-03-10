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
	"cmp"
	"fmt"
)

// object represents one of an open path, a closed path, or text.
type object struct {
	// points always starts with the top most, then left most point, proceeding to the right.
	points   []point
	text     []rune
	corners  []point
	isText   bool
	isClosed bool
	isDashed bool
}

// Points returns all the points occupied by this Object. Every object has at least one point,
// and all points are both in-order and contiguous.
func (o *object) Points() []point { return o.points }

// Corners returns all the corners (change of direction) along the path.
func (o *object) Corners() []point { return o.corners }

// IsClosed is true if the object is composed of a closed path.
func (o *object) IsClosed() bool { return o.isClosed }

// IsDashed is true if this object is a path object, and lines should be drawn dashed.
func (o *object) IsDashed() bool { return o.isDashed }

// Text returns the text associated with this object if textual, and nil otherwise.
func (o *object) Text() []rune {
	return o.text
}

func (o *object) isOpenPath() bool   { return !o.isClosed && !o.isText }
func (o *object) isClosedPath() bool { return o.isClosed && !o.isText }
func (o *object) isJustText() bool   { return o.isText }

func (o *object) String() string {
	if o.isJustText() {
		return fmt.Sprintf("Text{%s %q}", o.points[0], string(o.text))
	}
	return fmt.Sprintf("Path{%v}", o.points)
}

// seal finalizes the object, setting its text, its corners, and its various rendering hints.
func (o *object) seal(c *canvas) {
	if c.at(o.points[0]).isArrow() {
		o.points[0].hint = startMarker
		c.hasStartMarker = true
	}

	if c.at(o.points[len(o.points)-1]).isArrow() {
		o.points[len(o.points)-1].hint = endMarker
		c.hasEndMarker = true
	}

	o.corners, o.isClosed = pointsToCorners(o.points)
	o.text = make([]rune, len(o.points))

	for i, p := range o.points {
		ch := c.at(p)
		if !o.isJustText() {
			if ch.isTick() {
				o.points[i].hint = tick
			} else if ch.isDot() {
				o.points[i].hint = dot
			}
			if ch.isDashed() {
				o.isDashed = true
			}

			for _, corner := range o.corners {
				if corner.x == p.x && corner.y == p.y && c.at(p).isRoundedCorner() {
					o.points[i].hint = roundedCorner
				}
			}
		}
		o.text[i] = rune(ch)
	}
}

// objects implements a sortable collection of Object interfaces.
type objects []*object

func compare(l, r *object) int {
	// TODO(dhobsd): This doesn't catch every z-index case we could possibly want. We should
	// support z-indexing of objects through an a2s tag.
	lt := l.isJustText()
	rt := r.isJustText()
	if lt != rt {
		return 1
	}
	lp := l.Points()[0]
	rp := r.Points()[0]
	if lp.y != rp.y {
		return cmp.Compare(lp.y, rp.y)
	}
	return cmp.Compare(lp.x, rp.x)
}

const (
	dirNone = iota // No directionality
	dirH           // Horizontal
	dirV           // Vertical
	dirSE          // South-East
	dirSW          // South-West
	dirNW          // North-West
	dirNE          // North-East
)

// pointsToCorners returns all the corners (points at which there is a change of directionality) for
// a path. It additionally returns a truth value indicating whether the points supplied indicate a
// closed path.
func pointsToCorners(points []point) ([]point, bool) {
	l := len(points)
	// A path containing fewer than 3 points can neither be closed, nor change direction.
	if l < 3 {
		return points, false
	}
	out := []point{points[0]}

	dir := dirNone
	if isHorizontal(points[0], points[1]) {
		dir = dirH
	} else if isVertical(points[0], points[1]) {
		dir = dirV
	} else if isDiagonalSE(points[0], points[1]) {
		dir = dirSE
	} else if isDiagonalSW(points[0], points[1]) {
		dir = dirSW
	} else if isDiagonalNW(points[0], points[1]) {
		dir = dirNW
	} else if isDiagonalNE(points[0], points[1]) {
		dir = dirNE
	} else {
		panic(fmt.Errorf("discontiguous points: %+v", points))
	}

	cornerFunc := func(idx, newDir int) {
		if dir != newDir {
			out = append(out, points[idx-1])
			dir = newDir
		}
	}

	// Starting from the third point, check to see if the directionality between points P and
	// P-1 has changed.
	for i := 2; i < l; i++ {
		if isHorizontal(points[i-1], points[i]) {
			cornerFunc(i, dirH)
		} else if isVertical(points[i-1], points[i]) {
			cornerFunc(i, dirV)
		} else if isDiagonalSE(points[i-1], points[i]) {
			cornerFunc(i, dirSE)
		} else if isDiagonalSW(points[i-1], points[i]) {
			cornerFunc(i, dirSW)
		} else if isDiagonalNW(points[i-1], points[i]) {
			cornerFunc(i, dirNW)
		} else if isDiagonalNE(points[i-1], points[i]) {
			cornerFunc(i, dirNE)
		} else {
			panic(fmt.Errorf("discontiguous points: %+v", points))
		}
	}

	// Check if the points indicate a closed path. If not, append the last point.
	last := points[l-1]
	closed := true
	closedFunc := func(newDir int) {
		if dir != newDir {
			closed = false
			out = append(out, last)
		}
	}
	if isHorizontal(points[0], last) {
		closedFunc(dirH)
	} else if isVertical(points[0], last) {
		closedFunc(dirV)
	} else if isDiagonalNE(last, points[0]) {
		closedFunc(dirNE)
	} else {
		// Note: we'll always find any closed polygon from its top-left-most point. If it
		// is closed, it must be closed in the north-easterly direction, thus we don't test
		// for any other types of polygone closure.
		closed = false
		out = append(out, last)
	}

	return out, closed
}
