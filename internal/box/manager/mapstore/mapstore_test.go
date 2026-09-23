//-----------------------------------------------------------------------------
// Copyright (c) 2026-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2026-present Detlef Stern
//-----------------------------------------------------------------------------

package mapstore

import (
	"slices"
	"testing"

	"zettelstore.de/z/internal/box/manager/store"
)

func equalWordList(exp, got []string) bool {
	slices.Sort(got)
	return slices.Equal(exp, got)
}

func TestWordsDiff(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		cur        store.WordSet
		old        []string
		expN, expR []string
	}{
		{nil, nil, nil, nil},
		{nil, []string{}, nil, nil},
		{store.WordSet{}, nil, nil, nil},
		{store.WordSet{}, []string{}, nil, nil},
		{store.WordSet{"a": struct{}{}}, []string{}, []string{"a"}, nil},
		{store.WordSet{"a": struct{}{}}, []string{"b"}, []string{"a"}, []string{"b"}},
		{store.WordSet{}, []string{"b"}, nil, []string{"b"}},
		{store.WordSet{"a": struct{}{}}, []string{"a"}, nil, nil},
	}
	for i, tc := range testcases {
		gotN, gotR := diffWordSet(tc.cur, tc.old)
		if !equalWordList(tc.expN, gotN) {
			t.Errorf("%d: diffWordSet(%v, %v)->new %v, but got %v", i, tc.cur, tc.old, tc.expN, gotN)
		}
		if !equalWordList(tc.expR, gotR) {
			t.Errorf("%d: diffWordSet(%v, %v)->rem %v, but got %v", i, tc.cur, tc.old, tc.expR, gotR)
		}
	}
}
