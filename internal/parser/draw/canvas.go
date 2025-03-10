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
	"image"
	"slices"
	"unicode/utf8"
)

// newCanvas returns a new Canvas, initialized from the provided data. If tabWidth is set to a non-negative
// value, that value will be used to convert tabs to spaces within the grid. Creation of the Canvas
// can fail if the diagram contains invalid UTF-8 sequences.
func newCanvas(data []byte) (*canvas, error) {
	c := &canvas{}

	lines := bytes.Split(data, []byte("\n"))
	c.siz.Y = len(lines)

	// Diagrams will often not be padded to a uniform width. To overcome this, we scan over
	// each line and figure out which is the longest. This becomes the width of the canvas.
	for i, line := range lines {
		if ok := utf8.Valid(line); !ok {
			return nil, fmt.Errorf("invalid UTF-8 encoding on line %d", i)
		}
		if i1 := utf8.RuneCount(line); i1 > c.siz.X {
			c.siz.X = i1
		}
	}

	c.grid = make([]char, c.siz.X*c.siz.Y)
	c.visited = make([]bool, c.siz.X*c.siz.Y)
	for y, line := range lines {
		x := 0
		for len(line) > 0 {
			r, l := utf8.DecodeRune(line)
			c.grid[y*c.siz.X+x] = char(r)
			x++
			line = line[l:]
		}

		for ; x < c.siz.X; x++ {
			c.grid[y*c.siz.X+x] = ' '
		}
	}

	c.findObjects()
	return c, nil
}

// canvas is the parsed source data.
type canvas struct {
	// (0,0) is top left.
	grid           []char
	visited        []bool
	objs           objects
	siz            image.Point
	hasStartMarker bool
	hasEndMarker   bool
}

// String provides a view into the underlying grid.
func (c *canvas) String() string { return fmt.Sprintf("%+v", c.grid) }

// objects returns all the objects found in the underlying grid.
func (c *canvas) objects() objects { return c.objs }

// size returns the visual dimensions of the Canvas.
func (c *canvas) size() image.Point { return c.siz }

// findObjects finds all objects (lines, polygons, and text) within the underlying grid.
func (c *canvas) findObjects() {
	c.findPaths()
	c.findTexts()
	slices.SortFunc(c.objs, compare)
}

// findPaths by starting with a point that wasn't yet visited, beginning at the top
// left of the grid.
func (c *canvas) findPaths() {
	for y := range c.siz.Y {
		p := point{y: y}
		for x := range c.siz.X {
			p.x = x
			if c.isVisited(p) {
				continue
			}
			ch := c.at(p)
			if !ch.isPathStart() {
				continue
			}

			// Found the start of a one or multiple connected paths. Traverse all
			// connecting points. This will generate multiple objects if multiple
			// paths (either open or closed) are found.
			c.visit(p)
			objs := c.scanPath([]point{p})
			for _, obj := range objs {
				// For all points in all objects found, mark the points as visited.
				for _, p := range obj.Points() {
					c.visit(p)
				}
			}
			c.objs = append(c.objs, objs...)
		}
	}
}

// findTexts with a second pass through the grid attempts to identify any text within the grid.
func (c *canvas) findTexts() {
	for y := range c.siz.Y {
		p := point{}
		p.y = y
		for x := range c.siz.X {
			p.x = x
			if c.isVisited(p) {
				continue
			}
			ch := c.at(p)
			if !ch.isTextStart() {
				continue
			}

			// scanText will return nil if the text at this area is simply
			// setting options on a container object.
			obj := c.scanText(p)
			if obj == nil {
				continue
			}
			for _, p := range obj.Points() {
				c.visit(p)
			}
			c.objs = append(c.objs, obj)
		}
	}
}

// scanPath tries to complete a total path (for lines or polygons) starting with some partial path.
// It recurses when it finds multiple unvisited outgoing paths.
func (c *canvas) scanPath(points []point) objects {
	cur := points[len(points)-1]
	next := c.next(cur)

	// If there are no points that can progress traversal of the path, finalize the one we're
	// working on, and return it. This is the terminal condition in the passive flow.
	if len(next) == 0 {
		if len(points) == 1 {
			// Discard 'path' of 1 point. Do not mark point as visited.
			c.unvisit(cur)
			return nil
		}

		// TODO(dhobsd): Determine if path is sharing the line with another path. If so,
		// we may want to join the objects such that we don't get weird rendering artifacts.
		o := &object{points: points}
		o.seal(c)
		return objects{o}
	}

	// If we have hit a point that can create a closed path, create an object and close
	// the path. Additionally, recurse to other progress directions in case e.g. an open
	// path spawns from this point. Paths are always closed vertically.
	if cur.x == points[0].x && cur.y == points[0].y+1 {
		o := &object{points: points}
		o.seal(c)
		r := objects{o}
		return append(r, c.scanPath([]point{cur})...)
	}

	// We scan depth-first instead of breadth-first, making it possible to find a
	// closed path.
	var objs objects
	for _, n := range next {
		if c.isVisited(n) {
			continue
		}
		c.visit(n)
		p2 := make([]point, len(points)+1)
		copy(p2, points)
		p2[len(p2)-1] = n
		objs = append(objs, c.scanPath(p2)...)
	}
	return objs
}

// The next returns the points that can be used to make progress, scanning (in order) horizontal
// progress to the left or right, vertical progress above or below, or diagonal progress to NW,
// NE, SW, and SE. It skips any points already visited, and returns all of the possible progress
// points.
func (c *canvas) next(pos point) []point {
	// Our caller must have called c.visit prior to calling this function.
	if !c.isVisited(pos) {
		panic(fmt.Errorf("internal error; revisiting %s", pos))
	}

	var out []point

	nextHorizontal := func(p point) {
		if !c.isVisited(p) && c.at(p).canHorizontal() {
			out = append(out, p)
		}
	}
	nextVertical := func(p point) {
		if !c.isVisited(p) && c.at(p).canVertical() {
			out = append(out, p)
		}
	}
	nextDiagonal := func(from, to point) {
		if !c.isVisited(to) && c.at(to).canDiagonalFrom(c.at(from)) {
			out = append(out, to)
		}
	}

	ch := c.at(pos)
	if ch.canHorizontal() {
		if c.canLeft(pos) {
			n := pos
			n.x--
			nextHorizontal(n)
		}
		if c.canRight(pos) {
			n := pos
			n.x++
			nextHorizontal(n)
		}
	}
	if ch.canVertical() {
		if c.canUp(pos) {
			n := pos
			n.y--
			nextVertical(n)
		}
		if c.canDown(pos) {
			n := pos
			n.y++
			nextVertical(n)
		}
	}
	if c.canDiagonal(pos) {
		if c.canUp(pos) {
			if c.canLeft(pos) {
				n := pos
				n.x--
				n.y--
				nextDiagonal(pos, n)
			}
			if c.canRight(pos) {
				n := pos
				n.x++
				n.y--
				nextDiagonal(pos, n)
			}
		}
		if c.canDown(pos) {
			if c.canLeft(pos) {
				n := pos
				n.x--
				n.y++
				nextDiagonal(pos, n)
			}
			if c.canRight(pos) {
				n := pos
				n.x++
				n.y++
				nextDiagonal(pos, n)
			}
		}
	}

	return out
}

// scanText extracts a line of text.
func (c *canvas) scanText(start point) *object {
	obj := &object{points: []point{start}, isText: true}
	whiteSpaceStreak := 0
	cur := start

	for c.canRight(cur) {
		cur.x++
		if c.isVisited(cur) {
			// If the point is already visited, we hit a polygon or a line.
			break
		}
		ch := c.at(cur)
		if !ch.isTextCont() {
			break
		}
		if ch.isSpace() {
			whiteSpaceStreak++
			// Stop when we see 3 consecutive whitespace points.
			if whiteSpaceStreak > 2 {
				break
			}
		} else {
			whiteSpaceStreak = 0
		}
		obj.points = append(obj.points, cur)
	}

	// Trim the right side of the text object.
	for len(obj.points) != 0 && c.at(obj.points[len(obj.points)-1]).isSpace() {
		obj.points = obj.points[:len(obj.points)-1]
	}

	obj.seal(c)
	return obj
}

func (c *canvas) at(p point) char {
	return c.grid[p.y*c.siz.X+p.x]
}

func (c *canvas) isVisited(p point) bool {
	return c.visited[p.y*c.siz.X+p.x]
}

func (c *canvas) visit(p point) {
	// TODO(dhobsd): Change code to ensure that visit() is called once and only
	// once per point.
	c.visited[p.y*c.siz.X+p.x] = true
}

func (c *canvas) unvisit(p point) {
	o := p.y*c.siz.X + p.x
	if !c.visited[o] {
		panic(fmt.Errorf("internal error: point %+v never visited", p))
	}
	c.visited[o] = false
}

func (*canvas) canLeft(p point) bool    { return p.x > 0 }
func (c *canvas) canRight(p point) bool { return p.x < c.siz.X-1 }
func (*canvas) canUp(p point) bool      { return p.y > 0 }
func (c *canvas) canDown(p point) bool  { return p.y < c.siz.Y-1 }

func (c *canvas) canDiagonal(p point) bool {
	return (c.canLeft(p) || c.canRight(p)) && (c.canUp(p) || c.canDown(p))
}
