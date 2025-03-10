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

package store_test

import (
	"slices"
	"testing"

	"zettelstore.de/z/internal/box/manager/store"
)

func equalWordList(exp, got []string) bool {
	if len(exp) != len(got) {
		return false
	}
	if len(got) == 0 {
		return len(exp) == 0
	}
	slices.Sort(got)
	for i, w := range exp {
		if w != got[i] {
			return false
		}
	}
	return true
}

func TestWordsWords(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		words store.WordSet
		exp   []string
	}{
		{nil, nil},
		{store.WordSet{}, nil},
		{store.WordSet{"a": 1, "b": 2}, []string{"a", "b"}},
	}
	for i, tc := range testcases {
		got := tc.words.Words()
		if !equalWordList(tc.exp, got) {
			t.Errorf("%d: %v.Words() == %v, but got %v", i, tc.words, tc.exp, got)
		}
	}
}

func TestWordsDiff(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		cur        store.WordSet
		old        []string
		expN, expR []string
	}{
		{nil, nil, nil, nil},
		{store.WordSet{}, []string{}, nil, nil},
		{store.WordSet{"a": 1}, []string{}, []string{"a"}, nil},
		{store.WordSet{"a": 1}, []string{"b"}, []string{"a"}, []string{"b"}},
		{store.WordSet{}, []string{"b"}, nil, []string{"b"}},
		{store.WordSet{"a": 1}, []string{"a"}, nil, nil},
	}
	for i, tc := range testcases {
		gotN, gotR := tc.cur.Diff(tc.old)
		if !equalWordList(tc.expN, gotN) {
			t.Errorf("%d: %v.Diff(%v)->new %v, but got %v", i, tc.cur, tc.old, tc.expN, gotN)
		}
		if !equalWordList(tc.expR, gotR) {
			t.Errorf("%d: %v.Diff(%v)->rem %v, but got %v", i, tc.cur, tc.old, tc.expR, gotR)
		}
	}
}
