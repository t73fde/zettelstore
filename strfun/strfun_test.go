//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package strfun_test

import (
	"testing"

	"zettelstore.de/z/strfun"
)

func TestLength(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		in  string
		exp int
	}{
		{"", 0},
		{"äbc", 3},
	}
	for i, tc := range testcases {
		got := strfun.Length(tc.in)
		if got != tc.exp {
			t.Errorf("%d/%q: expected %v, got %v", i, tc.in, tc.exp, got)
		}
	}
}

func TestJustifyLeft(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		in  string
		ml  int
		exp string
	}{
		{"", 0, ""},
		{"äbc", 0, ""},
		{"äbc", 1, "\u2025"},
		{"äbc", 2, "ä\u2025"},
		{"äbc", 3, "äbc"},
		{"äbc", 4, "äbc:"},
	}
	for i, tc := range testcases {
		got := strfun.JustifyLeft(tc.in, tc.ml, ':')
		if got != tc.exp {
			t.Errorf("%d/%q/%d: expected %q, got %q", i, tc.in, tc.ml, tc.exp, got)
		}
	}
}

func TestSplitLines(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		in  string
		exp []string
	}{
		{"", nil},
		{"\n", nil},
		{"a", []string{"a"}},
		{"a\n", []string{"a"}},
		{"a\n\n", []string{"a"}},
		{"a\n\nb", []string{"a", "b"}},
	}
	for i, tc := range testcases {
		got := strfun.SplitLines(tc.in)
		if !compareStringslice(tc.exp, got) {
			t.Errorf("%d/%q: expected %q, got %q", i, tc.in, tc.exp, got)
		}
	}
}

func compareStringslice(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i, s := range s1 {
		if s != s2[i] {
			return false
		}
	}
	return true
}
