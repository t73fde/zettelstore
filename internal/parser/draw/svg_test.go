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
	"strings"
	"testing"
)

func TestCanvasToSVG(t *testing.T) {
	t.Parallel()
	data := []struct {
		input  []string
		length int
	}{
		// 0 Box with dashed corners and text
		{
			[]string{
				"+--.",
				"|Hi:",
				"+--+",
			},
			482,
		},

		// 2 Ticks and dots in lines.
		{
			[]string{
				" ------x----->",
				"",
				" <-----*------",
			},
			1084,
		},

		// 3 Just text
		{
			[]string{
				" foo",
			},
			261,
		},
	}
	for i, line := range data {
		canvas, err := newCanvas([]byte(strings.Join(line.input, "\n")))
		if err != nil {
			t.Fatalf("Error creating canvas: %s", err)
		}
		actual := string(canvasToSVG(canvas, "", 9, 16))
		// TODO(dhobsd): Use golden file? Worth postponing once output is actually
		// nice.
		if line.length != len(actual) {
			t.Errorf("%d: expected length %d, but got %d\n%q", i, line.length, len(actual), actual)
		}
	}
}
