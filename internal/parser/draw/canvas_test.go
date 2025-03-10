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
	"reflect"
	"strings"
	"testing"
)

func TestNewCanvas(t *testing.T) {
	t.Parallel()
	data := []struct {
		input     []string
		strings   []string
		texts     []string
		points    [][]point
		allPoints bool
	}{
		// 0 Small box
		{
			[]string{
				"+-+",
				"| |",
				"+-+",
			},
			[]string{"Path{[(0,0) (1,0) (2,0) (2,1) (2,2) (1,2) (0,2) (0,1)]}"},
			[]string{""},
			[][]point{{{x: 0, y: 0}, {x: 2, y: 0}, {x: 2, y: 2}, {x: 0, y: 2}}},
			false,
		},

		// 1 Tight box
		{
			[]string{
				"++",
				"++",
			},
			[]string{"Path{[(0,0) (1,0) (1,1) (0,1)]}"},
			[]string{""},
			[][]point{
				{
					{x: 0, y: 0}, {x: 1, y: 0}, {x: 1, y: 1}, {x: 0, y: 1},
				},
			},
			false,
		},

		// 2 Indented box
		{
			[]string{
				"",
				" +-+",
				" | |",
				" +-+",
			},
			[]string{"Path{[(1,1) (2,1) (3,1) (3,2) (3,3) (2,3) (1,3) (1,2)]}"},
			[]string{""},
			[][]point{{{x: 1, y: 1}, {x: 3, y: 1}, {x: 3, y: 3}, {x: 1, y: 3}}},
			false,
		},

		// 3 Free flow text
		{
			[]string{
				"",
				" foo bar ",
				"b  baz   bee",
			},
			[]string{"Text{(1,1) \"foo bar\"}", "Text{(0,2) \"b  baz\"}", "Text{(9,2) \"bee\"}"},
			[]string{"foo bar", "b  baz", "bee"},
			[][]point{
				{{x: 1, y: 1}, {x: 7, y: 1}},
				{{x: 0, y: 2}, {x: 5, y: 2}},
				{{x: 9, y: 2}, {x: 11, y: 2}},
			},
			false,
		},

		// 4 Text in a box
		{
			[]string{
				"+--+",
				"|Hi|",
				"+--+",
			},
			[]string{"Path{[(0,0) (1,0) (2,0) (3,0) (3,1) (3,2) (2,2) (1,2) (0,2) (0,1)]}", "Text{(1,1) \"Hi\"}"},
			[]string{"", "Hi"},
			[][]point{
				{{x: 0, y: 0}, {x: 3, y: 0}, {x: 3, y: 2}, {x: 0, y: 2}},
				{{x: 1, y: 1}, {x: 2, y: 1}},
			},
			false,
		},

		// 5 Concave pieces
		{
			[]string{
				"    +----+",
				"    |    |",
				"+---+    +----+",
				"|             |",
				"+-------------+",
				"", // 5
				"+----+",
				"|    |",
				"|    +---+",
				"|        |",
				"|    +---+", // 10
				"|    |",
				"+----+",
				"",
				"    +----+",
				"    |    |", // 15
				"+---+    |",
				"|        |",
				"+---+    |",
				"    |    |",
				"    +----+",
			},
			[]string{
				"Path{[(4,0) (5,0) (6,0) (7,0) (8,0) (9,0) (9,1) (9,2) (10,2) (11,2) (12,2) (13,2) (14,2) (14,3) (14,4) (13,4) (12,4) (11,4) (10,4) (9,4) (8,4) (7,4) (6,4) (5,4) (4,4) (3,4) (2,4) (1,4) (0,4) (0,3) (0,2) (1,2) (2,2) (3,2) (4,2) (4,1)]}",
				"Path{[(0,6) (1,6) (2,6) (3,6) (4,6) (5,6) (5,7) (5,8) (6,8) (7,8) (8,8) (9,8) (9,9) (9,10) (8,10) (7,10) (6,10) (5,10) (5,11) (5,12) (4,12) (3,12) (2,12) (1,12) (0,12) (0,11) (0,10) (0,9) (0,8) (0,7)]}",
				"Path{[(4,14) (5,14) (6,14) (7,14) (8,14) (9,14) (9,15) (9,16) (9,17) (9,18) (9,19) (9,20) (8,20) (7,20) (6,20) (5,20) (4,20) (4,19) (4,18) (3,18) (2,18) (1,18) (0,18) (0,17) (0,16) (1,16) (2,16) (3,16) (4,16) (4,15)]}",
			},
			[]string{"", "", ""},
			[][]point{
				{
					{x: 4, y: 0}, {x: 9, y: 0}, {x: 9, y: 2}, {x: 14, y: 2},
					{x: 14, y: 4}, {x: 0, y: 4}, {x: 0, y: 2}, {x: 4, y: 2},
				},
				{
					{x: 0, y: 6}, {x: 5, y: 6}, {x: 5, y: 8}, {x: 9, y: 8},
					{x: 9, y: 10}, {x: 5, y: 10}, {x: 5, y: 12}, {x: 0, y: 12},
				},
				{
					{x: 4, y: 14}, {x: 9, y: 14}, {x: 9, y: 20}, {x: 4, y: 20},
					{x: 4, y: 18}, {x: 0, y: 18}, {x: 0, y: 16}, {x: 4, y: 16},
				},
			},
			false,
		},

		// 6 Inner boxes
		{
			[]string{
				"+-----+",
				"|     |",
				"| +-+ |",
				"| | | |",
				"| +-+ |",
				"|     |",
				"+-----+",
			},
			[]string{
				"Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (5,0) (6,0) (6,1) (6,2) (6,3) (6,4) (6,5) (6,6) (5,6) (4,6) (3,6) (2,6) (1,6) (0,6) (0,5) (0,4) (0,3) (0,2) (0,1)]}",
				"Path{[(2,2) (3,2) (4,2) (4,3) (4,4) (3,4) (2,4) (2,3)]}",
			},
			[]string{"", ""},
			[][]point{
				{{x: 0, y: 0}, {x: 6, y: 0}, {x: 6, y: 6}, {x: 0, y: 6}},
				{{x: 2, y: 2}, {x: 4, y: 2}, {x: 4, y: 4}, {x: 2, y: 4}},
			},
			false,
		},

		// 7 Real world diagram example
		{
			[]string{
				//         1         2         3
				"      +------+",
				"      |Editor|-------------+--------+",
				"      +------+             |        |",
				"          |                |        v",
				"          v                |   +--------+",
				"      +------+             |   |Document|", // 5
				"      |Window|             |   +--------+",
				"      +------+             |",
				"         |                 |",
				"   +-----+-------+         |",
				"   |             |         |", // 10
				"   v             v         |",
				"+------+     +------+      |",
				"|Window|     |Window|      |",
				"+------+     +------+      |",
				"                |          |", // 15
				"                v          |",
				"              +----+       |",
				"              |View|       |",
				"              +----+       |",
				"                |          |", // 20
				"                v          |",
				"            +--------+     |",
				"            |Document|<----+",
				"            +--------+",
			},
			[]string{
				"Path{[(6,0) (7,0) (8,0) (9,0) (10,0) (11,0) (12,0) (13,0) (13,1) (13,2) (12,2) (11,2) (10,2) (9,2) (8,2) (7,2) (6,2) (6,1)]}",
				"Path{[(14,1) (15,1) (16,1) (17,1) (18,1) (19,1) (20,1) (21,1) (22,1) (23,1) (24,1) (25,1) (26,1) (27,1) (28,1) (29,1) (30,1) (31,1) (32,1) (33,1) (34,1) (35,1) (36,1) (36,2) (36,3)]}",
				"Path{[(14,1) (15,1) (16,1) (17,1) (18,1) (19,1) (20,1) (21,1) (22,1) (23,1) (24,1) (25,1) (26,1) (27,1) (27,2) (27,3) (27,4) (27,5) (27,6) (27,7) (27,8) (27,9) (27,10) (27,11) (27,12) (27,13) (27,14) (27,15) (27,16) (27,17) (27,18) (27,19) (27,20) (27,21) (27,22) (27,23) (26,23) (25,23) (24,23) (23,23) (22,23)]}",
				"Path{[(10,3) (10,4)]}",
				"Path{[(31,4) (32,4) (33,4) (34,4) (35,4) (36,4) (37,4) (38,4) (39,4) (40,4) (40,5) (40,6) (39,6) (38,6) (37,6) (36,6) (35,6) (34,6) (33,6) (32,6) (31,6) (31,5)]}",
				"Path{[(6,5) (7,5) (8,5) (9,5) (10,5) (11,5) (12,5) (13,5) (13,6) (13,7) (12,7) (11,7) (10,7) (9,7) (8,7) (7,7) (6,7) (6,6)]}",
				"Path{[(9,8) (9,9)]}",
				"Path{[(9,9) (8,9) (7,9) (6,9) (5,9) (4,9) (3,9) (3,10) (3,11)]}",
				"Path{[(9,9) (10,9) (11,9) (12,9) (13,9) (14,9) (15,9) (16,9) (17,9) (17,10) (17,11)]}",
				"Path{[(0,12) (1,12) (2,12) (3,12) (4,12) (5,12) (6,12) (7,12) (7,13) (7,14) (6,14) (5,14) (4,14) (3,14) (2,14) (1,14) (0,14) (0,13)]}",
				"Path{[(13,12) (14,12) (15,12) (16,12) (17,12) (18,12) (19,12) (20,12) (20,13) (20,14) (19,14) (18,14) (17,14) (16,14) (15,14) (14,14) (13,14) (13,13)]}",
				"Path{[(16,15) (16,16)]}",
				"Path{[(14,17) (15,17) (16,17) (17,17) (18,17) (19,17) (19,18) (19,19) (18,19) (17,19) (16,19) (15,19) (14,19) (14,18)]}",
				"Path{[(16,20) (16,21)]}",
				"Path{[(12,22) (13,22) (14,22) (15,22) (16,22) (17,22) (18,22) (19,22) (20,22) (21,22) (21,23) (21,24) (20,24) (19,24) (18,24) (17,24) (16,24) (15,24) (14,24) (13,24) (12,24) (12,23)]}",
				"Text{(7,1) \"Editor\"}",
				"Text{(32,5) \"Document\"}",
				"Text{(7,6) \"Window\"}",
				"Text{(1,13) \"Window\"}",
				"Text{(14,13) \"Window\"}",
				"Text{(15,18) \"View\"}",
				"Text{(13,23) \"Document\"}",
			},
			[]string{
				"", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
				"Editor", "Document", "Window", "Window", "Window", "View", "Document",
			},
			[][]point{
				{{x: 6, y: 0}, {x: 13, y: 0}, {x: 13, y: 2}, {x: 6, y: 2}},
				{{x: 14, y: 1}, {x: 36, y: 1}, {x: 36, y: 3, hint: 3}},
				{{x: 14, y: 1}, {x: 27, y: 1}, {x: 27, y: 23}, {x: 22, y: 23, hint: 3}},
				{{x: 10, y: 3}, {x: 10, y: 4, hint: 3}},
				{{x: 31, y: 4}, {x: 40, y: 4}, {x: 40, y: 6}, {x: 31, y: 6}},
				{{x: 6, y: 5}, {x: 13, y: 5}, {x: 13, y: 7}, {x: 6, y: 7}},
				{{x: 9, y: 8}, {x: 9, y: 9}},
				{{x: 9, y: 9}, {x: 3, y: 9}, {x: 3, y: 11, hint: 3}},
				{{x: 9, y: 9}, {x: 17, y: 9}, {x: 17, y: 11, hint: 3}},
				{{x: 0, y: 12}, {x: 7, y: 12}, {x: 7, y: 14}, {x: 0, y: 14}},
				{{x: 13, y: 12}, {x: 20, y: 12}, {x: 20, y: 14}, {x: 13, y: 14}},
				{{x: 16, y: 15}, {x: 16, y: 16, hint: 3}},
				{{x: 14, y: 17}, {x: 19, y: 17}, {x: 19, y: 19}, {x: 14, y: 19}},
				{{x: 16, y: 20}, {x: 16, y: 21, hint: 3}},
				{{x: 12, y: 22}, {x: 21, y: 22}, {x: 21, y: 24}, {x: 12, y: 24}},
				{{x: 7, y: 1}, {x: 12, y: 1}},
				{{x: 32, y: 5}, {x: 39, y: 5}},
				{{x: 7, y: 6}, {x: 12, y: 6}},
				{{x: 1, y: 13}, {x: 6, y: 13}},
				{{x: 14, y: 13}, {x: 19, y: 13}},
				{{x: 15, y: 18}, {x: 18, y: 18}},
				{{x: 13, y: 23}, {x: 20, y: 23}},
			},
			false,
		},

		// 8 Interwined lines.
		{
			[]string{
				"             +-----+-------+",
				"             |     |       |",
				"             |     |       |",
				"        +----+-----+----   |",
				"--------+----+-----+-------+---+",
				"        |    |     |       |   |",
				"        |    |     |       |   |     |   |",
				"        |    |     |       |   |     |   |",
				"        |    |     |       |   |     |   |",
				"--------+----+-----+-------+---+-----+---+--+",
				"        |    |     |       |   |     |   |  |",
				"        |    |     |       |   |     |   |  |",
				"        |   -+-----+-------+---+-----+   |  |",
				"        |    |     |       |   |     |   |  |",
				"        |    |     |       |   +-----+---+--+",
				"             |     |       |         |   |",
				"             |     |       |         |   |",
				"     --------+-----+-------+---------+---+-----",
				"             |     |       |         |   |",
				"             +-----+-------+---------+---+",
			},
			// TODO(dhobsd): it's a tad overwhelming.
			nil,
			nil,
			nil,
			false,
		},

		// 9 Indented box
		{
			[]string{
				"",
				" +-+",
				" | |",
				" +-+",
			},
			[]string{"Path{[(1,1) (2,1) (3,1) (3,2) (3,3) (2,3) (1,3) (1,2)]}"},
			[]string{""},
			[][]point{{{x: 1, y: 1}, {x: 3, y: 1}, {x: 3, y: 3}, {x: 1, y: 3}}},
			false,
		},

		// 10 Diagonal lines with arrows
		{
			[]string{
				"^          ^",
				" \\        /",
				"  \\      /",
				"   \\    /",
				"    v  v",
			},
			[]string{"Path{[(0,0) (1,1) (2,2) (3,3) (4,4)]}", "Path{[(11,0) (10,1) (9,2) (8,3) (7,4)]}"},
			[]string{"", ""},
			[][]point{
				{{x: 0, y: 0, hint: 2}, {x: 4, y: 4, hint: 3}},
				{{x: 11, y: 0, hint: 2}, {x: 7, y: 4, hint: 3}},
			},
			false,
		},

		// 11 Diagonal lines forming an object
		{
			[]string{
				"   .-----.",
				"  /       \\",
				" /         \\",
				"+           +",
				"|           |",
				"|           |",
				"+           +",
				" \\         /",
				"  \\       /",
				"   '-----'",
			},
			[]string{"Path{[(3,0) (4,0) (5,0) (6,0) (7,0) (8,0) (9,0) (10,1) (11,2) (12,3) (12,4) (12,5) (12,6) (11,7) (10,8) (9,9) (8,9) (7,9) (6,9) (5,9) (4,9) (3,9) (2,8) (1,7) (0,6) (0,5) (0,4) (0,3) (1,2) (2,1)]}"},
			[]string{""},
			[][]point{{
				{x: 3, y: 0},
				{x: 9, y: 0},
				{x: 12, y: 3},
				{x: 12, y: 6},
				{x: 9, y: 9},
				{x: 3, y: 9},
				{x: 0, y: 6},
				{x: 0, y: 3},
			}},
			false,
		},

		// 12 A2S logo
		{
			[]string{
				".-------------------------.",
				"|                         |",
				"| .---.-. .-----. .-----. |",
				"| | .-. | +-->  | |  <--+ |",
				"| | '-' | |  <--+ +-->  | |",
				"| '---'-' '-----' '-----' |",
				"|  ascii     2      svg   |",
				"|                         |",
				"'-------------------------'",
			},
			[]string{
				"Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (5,0) (6,0) (7,0) (8,0) (9,0) (10,0) (11,0) (12,0) (13,0) (14,0) (15,0) (16,0) (17,0) (18,0) (19,0) (20,0) (21,0) (22,0) (23,0) (24,0) (25,0) (26,0) (26,1) (26,2) (26,3) (26,4) (26,5) (26,6) (26,7) (26,8) (25,8) (24,8) (23,8) (22,8) (21,8) (20,8) (19,8) (18,8) (17,8) (16,8) (15,8) (14,8) (13,8) (12,8) (11,8) (10,8) (9,8) (8,8) (7,8) (6,8) (5,8) (4,8) (3,8) (2,8) (1,8) (0,8) (0,7) (0,6) (0,5) (0,4) (0,3) (0,2) (0,1)]}",
				"Path{[(2,2) (3,2) (4,2) (5,2) (6,2) (7,2) (8,2) (8,3) (8,4) (8,5) (7,5) (6,5) (5,5) (4,5) (3,5) (2,5) (2,4) (2,3)]}",
				"Path{[(2,2) (3,2) (4,2) (5,2) (6,2) (7,2) (8,2) (8,3) (8,4) (8,5) (7,5) (6,5) (6,4) (5,4) (4,4) (4,3) (5,3) (6,3)]}",
				"Path{[(10,2) (11,2) (12,2) (13,2) (14,2) (15,2) (16,2) (16,3) (16,4) (15,4) (14,4) (13,4)]}",
				"Path{[(10,2) (11,2) (12,2) (13,2) (14,2) (15,2) (16,2) (16,3) (16,4) (16,5) (15,5) (14,5) (13,5) (12,5) (11,5) (10,5) (10,4) (10,3)]}",
				"Path{[(18,2) (19,2) (20,2) (21,2) (22,2) (23,2) (24,2) (24,3) (23,3) (22,3) (21,3)]}",
				"Path{[(18,2) (19,2) (20,2) (21,2) (22,2) (23,2) (24,2) (24,3) (24,4) (24,5) (23,5) (22,5) (21,5) (20,5) (19,5) (18,5) (18,4) (19,4) (20,4) (21,4)]}",
				"Path{[(18,2) (19,2) (20,2) (21,2) (22,2) (23,2) (24,2) (24,3) (24,4) (24,5) (23,5) (22,5) (21,5) (20,5) (19,5) (18,5) (18,4) (18,3)]}",
				"Path{[(10,3) (11,3) (12,3) (13,3)]}",
				"Text{(3,6) \"ascii\"}",
				"Text{(13,6) \"2\"}",
				"Text{(20,6) \"svg\"}",
			},
			[]string{"", "", "", "", "", "", "", "", "", "ascii", "2", "svg"},
			[][]point{
				{{x: 0, y: 0}, {x: 26, y: 0}, {x: 26, y: 8}, {x: 0, y: 8}},
				{{x: 2, y: 2}, {x: 8, y: 2}, {x: 8, y: 5}, {x: 2, y: 5}},
				{{x: 2, y: 2}, {x: 8, y: 2}, {x: 8, y: 5}, {x: 6, y: 5}, {x: 6, y: 4}, {x: 4, y: 4}, {x: 4, y: 3}, {x: 6, y: 3}},
				{{x: 10, y: 2}, {x: 16, y: 2}, {x: 16, y: 4}, {x: 13, y: 4, hint: 3}},
				{{x: 10, y: 2}, {x: 16, y: 2}, {x: 16, y: 5}, {x: 10, y: 5}},
				{{x: 18, y: 2}, {x: 24, y: 2}, {x: 24, y: 3}, {x: 21, y: 3, hint: 3}},
				{{x: 18, y: 2}, {x: 24, y: 2}, {x: 24, y: 5}, {x: 18, y: 5}, {x: 18, y: 4}, {x: 21, y: 4, hint: 3}},
				{{x: 18, y: 2}, {x: 24, y: 2}, {x: 24, y: 5}, {x: 18, y: 5}},
				{{x: 10, y: 3}, {x: 13, y: 3, hint: 3}},
				{{x: 3, y: 6}, {x: 7, y: 6}},
				{{x: 13, y: 6}},
				{{x: 20, y: 6}, {x: 22, y: 6}},
			},
			false,
		},

		// 13 Ticks and dots in lines.
		{
			[]string{
				" ------x----->",
				"",
				" <-----*------",
			},
			[]string{"Path{[(1,0) (2,0) (3,0) (4,0) (5,0) (6,0) (7,0) (8,0) (9,0) (10,0) (11,0) (12,0) (13,0)]}", "Path{[(1,2) (2,2) (3,2) (4,2) (5,2) (6,2) (7,2) (8,2) (9,2) (10,2) (11,2) (12,2) (13,2)]}"},
			[]string{"", ""},
			[][]point{
				{
					{x: 1, y: 0, hint: 0},
					{x: 2, y: 0, hint: 0},
					{x: 3, y: 0, hint: 0},
					{x: 4, y: 0, hint: 0},
					{x: 5, y: 0, hint: 0},
					{x: 6, y: 0, hint: 0},
					{x: 7, y: 0, hint: 4},
					{x: 8, y: 0, hint: 0},
					{x: 9, y: 0, hint: 0},
					{x: 10, y: 0, hint: 0},
					{x: 11, y: 0, hint: 0},
					{x: 12, y: 0, hint: 0},
					{x: 13, y: 0, hint: 3},
				},
				{
					{x: 1, y: 2, hint: 2},
					{x: 2, y: 2, hint: 0},
					{x: 3, y: 2, hint: 0},
					{x: 4, y: 2, hint: 0},
					{x: 5, y: 2, hint: 0},
					{x: 6, y: 2, hint: 0},
					{x: 7, y: 2, hint: 5},
					{x: 8, y: 2, hint: 0},
					{x: 9, y: 2, hint: 0},
					{x: 10, y: 2, hint: 0},
					{x: 11, y: 2, hint: 0},
					{x: 12, y: 2, hint: 0},
					{x: 13, y: 2, hint: 0},
				},
			},
			true,
		},

		// 14 Multiple closed path on one object
		{
			[]string{
				"+-+-+",
				"| | |",
				"+-+-+",
			},
			[]string{
				"Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (4,1) (4,2) (3,2) (2,2) (1,2) (0,2) (0,1)]}",
				"Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (4,1) (4,2) (3,2) (2,2) (2,1)]}", // TODO (2,0)
			},
			[]string{"", ""},
			[][]point{
				{
					{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {3, 0, 0}, {4, 0, 0}, {4, 1, 0}, {4, 2, 0},
					{3, 2, 0}, {2, 2, 0}, {1, 2, 0}, {0, 2, 0}, {0, 1, 0},
				},
				{
					{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {3, 0, 0}, {4, 0, 0}, {4, 1, 0}, {4, 2, 0},
					{3, 2, 0}, {2, 2, 0}, {2, 1, 0}, // TODO: {2, 0, 0}
				},
			},
			true,
		},
	}
	for i, line := range data {
		c, err := newCanvas([]byte(strings.Join(line.input, "\n")))
		if err != nil {
			t.Fatalf("Test %d: error creating canvas: %s", i, err)
		}
		objs := c.objects()
		if line.strings != nil {
			if got := getStrings(objs); !reflect.DeepEqual(line.strings, got) {
				t.Errorf("%d: expected %q, but got %q", i, line.strings, got)
			}
		}
		if line.texts != nil {
			if got := getTexts(objs); !reflect.DeepEqual(line.texts, got) {
				t.Errorf("%d: expected %q, but got %q", i, line.texts, got)
			}
		}
		if line.points != nil {
			if line.allPoints == false {
				if got := getCorners(objs); !reflect.DeepEqual(line.points, got) {
					t.Errorf("%d: expected %q, but got %q", i, line.points, got)
				}
			} else {
				if got := getPoints(objs); !reflect.DeepEqual(line.points, got) {
					t.Errorf("%d: expected %q, but got %q", i, line.points, got)
				}
			}
		}
	}
}

func TestNewCanvasBroken(t *testing.T) {
	// These are the ones that do not give the desired result.
	t.Parallel()
	data := []struct {
		input   []string
		strings []string
		texts   []string
		corners [][]point
	}{
		// 0 URL
		{
			[]string{
				"github.com/foo/bar",
			},
			[]string{"Text{(0,0) \"github.com/foo/bar\"}"},
			[]string{"github.com/foo/bar"},
			[][]point{{{x: 0, y: 0}, {x: 17, y: 0}}},
		},

		// 1 Merged boxes
		{
			[]string{
				"+-+-+",
				"| | |",
				"+-+-+",
			},
			[]string{"Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (4,1) (4,2) (3,2) (2,2) (1,2) (0,2) (0,1)]}", "Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (4,1) (4,2) (3,2) (2,2) (2,1)]}"},
			[]string{"", ""},
			// TODO(dhobsd): BROKEN.
			[][]point{
				{{x: 0, y: 0}, {x: 4, y: 0}, {x: 4, y: 2}, {x: 0, y: 2}},
				{{x: 0, y: 0}, {x: 4, y: 0}, {x: 4, y: 2}, {x: 2, y: 2}, {x: 2, y: 1}},
			},
		},

		// 2 Adjacent boxes
		{
			// TODO(dhobsd): BROKEN. This one is hard, as it can be seen as 3 boxes
			// but that is not what is desired.
			[]string{
				"+-++-+",
				"| || |",
				"+-++-+",
			},
			[]string{
				"Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (5,0) (5,1) (5,2) (4,2) (3,2) (2,2) (1,2) (0,2) (0,1)]}",
				"Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (5,0) (5,1) (5,2) (4,2) (3,2) (2,2) (2,1)]}",
				"Path{[(0,0) (1,0) (2,0) (3,0) (4,0) (5,0) (5,1) (5,2) (4,2) (3,2) (3,1)]}",
			},
			[]string{"", "", ""},
			[][]point{
				{{x: 0, y: 0}, {x: 5, y: 0}, {x: 5, y: 2}, {x: 0, y: 2}},
				{{x: 0, y: 0}, {x: 5, y: 0}, {x: 5, y: 2}, {x: 2, y: 2}, {x: 2, y: 1}},
				{{x: 0, y: 0}, {x: 5, y: 0}, {x: 5, y: 2}, {x: 3, y: 2}, {x: 3, y: 1}},
			},
		},
	}
	for i, line := range data {
		c, err := newCanvas([]byte(strings.Join(line.input, "\n")))
		if err != nil {
			t.Fatalf("Test %d: error creating canvas: %s", i, err)
		}
		objs := c.objects()
		if line.strings != nil {
			if got := getStrings(objs); !reflect.DeepEqual(line.strings, got) {
				t.Errorf("%d: expected %q, but got %q", i, line.strings, got)
			}
		}
		if line.texts != nil {
			if got := getTexts(objs); !reflect.DeepEqual(line.texts, got) {
				t.Errorf("%d: expected %q, but got %q", i, line.texts, got)
			}
		}
		if line.corners != nil {
			if got := getCorners(objs); !reflect.DeepEqual(line.corners, got) {
				t.Errorf("%d: expected %q, but got %q", i, line.corners, got)
			}
		}
	}
}

func TestPointsToCorners(t *testing.T) {
	t.Parallel()
	data := []struct {
		in       []point
		expected []point
		closed   bool
	}{
		{
			[]point{{x: 0, y: 0}, {x: 1, y: 0}},
			[]point{{x: 0, y: 0}, {x: 1, y: 0}},
			false,
		},
		{
			[]point{{x: 0, y: 0}, {x: 1, y: 0}, {x: 2, y: 0}},
			[]point{{x: 0, y: 0}, {x: 2, y: 0}},
			false,
		},
		{
			[]point{{x: 0, y: 0}, {x: 1, y: 0}, {x: 1, y: 1}},
			[]point{{x: 0, y: 0}, {x: 1, y: 0}, {x: 1, y: 1}},
			false,
		},
		{
			[]point{
				{x: 0, y: 0}, {x: 1, y: 0}, {x: 2, y: 0}, {x: 2, y: 1}, {x: 2, y: 2},
				{x: 1, y: 2}, {x: 0, y: 2}, {x: 0, y: 1},
			},
			[]point{{x: 0, y: 0}, {x: 2, y: 0}, {x: 2, y: 2}, {x: 0, y: 2}},
			true,
		},
		{
			[]point{{x: 0, y: 0}, {x: 1, y: 0}, {x: 1, y: 1}, {x: 0, y: 1}},
			[]point{{x: 0, y: 0}, {x: 1, y: 0}, {x: 1, y: 1}, {x: 0, y: 1}},
			// TODO(dhobsd): Unexpected; broken.
			false,
		},
	}
	for i, line := range data {
		p, c := pointsToCorners(line.in)
		if !reflect.DeepEqual(line.expected, p) {
			t.Errorf("%d: expected %v, but got %v", i, line.expected, p)
		}
		if line.closed != c {
			t.Errorf("%d: expected close == %v, but got %v", i, line.closed, c)
		}
	}
}

func BenchmarkT(b *testing.B) {
	data := []string{
		"             +-----+-------+",
		"             |     |       |",
		"             |     |       |",
		"        +----+-----+----   |",
		"--------+----+-----+-------+---+",
		"        |    |     |       |   |",
		"        |    |     |       |   |     |   |",
		"        |    |     |       |   |     |   |",
		"        |    |     |       |   |     |   |",
		"--------+----+-----+-------+---+-----+---+--+",
		"        |    |     |       |   |     |   |  |",
		"        |    |     |       |   |     |   |  |",
		"        |   -+-----+-------+---+-----+   |  |",
		"        |    |     |       |   |     |   |  |",
		"        |    |     |       |   +-----+---+--+",
		"             |     |       |         |   |",
		"             |     |       |         |   |",
		"     --------+-----+-------+---------+---+-----",
		"             |     |       |         |   |",
		"             +-----+-------+---------+---+",
		"",
		"",
	}
	chunk := []byte(strings.Join(data, "\n"))
	input := make([]byte, 0, len(chunk)*b.N)
	for range b.N {
		input = append(input, chunk...)
	}
	expected := 30 * b.N
	b.ResetTimer()
	c, err := newCanvas(input)
	if err != nil {
		b.Fatalf("Error creating canvas: %s", err)
	}

	objs := c.objects()
	if len(objs) != expected {
		b.Fatalf("%d != %d", len(objs), expected)
	}
}

// Private details.

func getPoints(objs []*object) [][]point {
	out := [][]point{}
	for _, obj := range objs {
		out = append(out, obj.Points())
	}
	return out
}

func getTexts(objs []*object) []string {
	out := []string{}
	for _, obj := range objs {
		t := obj.Text()
		if !obj.isJustText() {
			out = append(out, "")
		} else if len(t) > 0 {
			out = append(out, string(t))
		} else {
			panic("failed")
		}
	}
	return out
}

func getStrings(objs []*object) []string {
	out := []string{}
	for _, obj := range objs {
		out = append(out, obj.String())
	}
	return out
}

func getCorners(objs []*object) [][]point {
	out := make([][]point, len(objs))
	for i, obj := range objs {
		out[i] = obj.Corners()
	}
	return out
}
