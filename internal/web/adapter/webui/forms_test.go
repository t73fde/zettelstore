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

package webui

import "testing"

func TestRemoveEmptyLines(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		in  string
		exp string
	}{
		{"", ""},
		{"a", "a"},
		{"\na", "a"},
		{"a\n", "a"},
		{"a\nb", "a\nb"},
		{"a\n\nb", "a\nb"},
		{"a\n \nb", "a\nb"},
	}
	for i, tc := range testcases {
		got := string(removeEmptyLines([]byte(tc.in)))
		if got != tc.exp {
			t.Errorf("%d/%q: expected=%q, got=%q", i, tc.in, tc.exp, got)
		}
	}
}
