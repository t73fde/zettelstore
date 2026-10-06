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

package idset_test

import (
	"slices"
	"testing"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/idset"
)

// SafeSorted returns the set as a new sorted slice of zettel identifier.
func safeSorted(s *idset.ArraySet) []id.Zid {
	if s == nil {
		return nil
	}
	result := make([]id.Zid, 0, s.Count())
	for zid := range s.Values() {
		result = append(result, zid)
	}
	return result
}

func makeNew() *idset.ArraySet {
	tmp := idset.New()
	return &tmp
}

func collect(sl []id.Zid) *idset.ArraySet {
	result := idset.New()
	if len(sl) > 0 {
		result.Grow(len(sl))
		for _, zid := range sl {
			result.Insert(zid)
		}
	}
	return &result
}

func TestSetContains(t *testing.T) {
	testcases := []id.Zid{2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22}
	var e idset.ArraySet
	for _, tc := range testcases {
		if e.Contains(tc) {
			t.Errorf("nil set contains %v", tc)
		}
	}
	s := idset.New()
	data := slices.Clone(testcases)
	slices.Reverse(data)
	for _, zid := range data {
		s.Insert(zid)
	}
	for _, tc := range testcases {
		if !s.Contains(tc) {
			t.Errorf("set does not contain %v", tc)
		}
	}
	notFounds := []id.Zid{0, 1, 3, 5, 23}
	for _, zid := range notFounds {
		if s.Contains(zid) {
			t.Errorf("set does contain %v", zid)

		}
	}
}

func TestSetAdd(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		s1, s2 *idset.ArraySet
		exp    []id.Zid
	}{
		{makeNew(), makeNew(), nil},
		{collect([]id.Zid{1}), makeNew(), []id.Zid{1}},
		{collect([]id.Zid{1}), collect([]id.Zid{2}), []id.Zid{1, 2}},
		{collect([]id.Zid{1}), collect([]id.Zid{1}), []id.Zid{1}},
	}
	for i, tc := range testcases {
		sl1 := safeSorted(tc.s1)
		sl2 := safeSorted(tc.s2)
		tmp := tc.s1.Clone()
		tmp.Or(*tc.s2)
		got := safeSorted(&tmp)
		if !slices.Equal(got, tc.exp) {
			t.Errorf("%d: %v.Add(%v) should be %v, but got %v", i, sl1, sl2, tc.exp, got)
		}
	}
}

func TestSetSafeSorted(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		set *idset.ArraySet
		exp []id.Zid
	}{
		{nil, nil},
		{makeNew(), nil},
		{collect([]id.Zid{9, 4, 6, 1, 7}), []id.Zid{1, 4, 6, 7, 9}},
	}
	for i, tc := range testcases {
		got := safeSorted(tc.set)
		if !slices.Equal(got, tc.exp) {
			t.Errorf("%d: %v.SafeSorted() should be %v, but got %v", i, tc.set, tc.exp, got)
		}
	}
}

func TestSetIntersection(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		s1, s2 *idset.ArraySet
		exp    []id.Zid
	}{
		{makeNew(), makeNew(), nil},
		{collect([]id.Zid{1}), makeNew(), nil},
		{makeNew(), collect([]id.Zid{1}), nil},
		{collect([]id.Zid{1}), collect([]id.Zid{2}), nil},
		{collect([]id.Zid{2}), collect([]id.Zid{1}), nil},
		{collect([]id.Zid{1}), collect([]id.Zid{1}), []id.Zid{1}},
	}
	for i, tc := range testcases {
		sl1 := safeSorted(tc.s1)
		sl2 := safeSorted(tc.s2)
		r := tc.s1.Clone()
		r.And(*tc.s2)
		got := safeSorted(&r)
		if !slices.Equal(got, tc.exp) {
			t.Errorf("%d: %v.IntersectOrSet(%v) should be %v, but got %v", i, sl1, sl2, tc.exp, got)
		}
	}
}

func TestSetIUnion(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		s1, s2 *idset.ArraySet
		exp    *idset.ArraySet
	}{
		{makeNew(), makeNew(), makeNew()},
		{collect([]id.Zid{1}), makeNew(), collect([]id.Zid{1})},
		{makeNew(), collect([]id.Zid{1}), collect([]id.Zid{1})},
		{collect([]id.Zid{1}), collect([]id.Zid{2}), collect([]id.Zid{1, 2})},
		{collect([]id.Zid{2}), collect([]id.Zid{1}), collect([]id.Zid{2, 1})},
		{collect([]id.Zid{1}), collect([]id.Zid{1}), collect([]id.Zid{1})},
		{collect([]id.Zid{1, 2, 3}), collect([]id.Zid{2, 3, 4}), collect([]id.Zid{1, 2, 3, 4})},
	}
	for i, tc := range testcases {
		s1 := tc.s1.Clone()
		sl1 := safeSorted(&s1)
		sl2 := safeSorted(tc.s2)
		s1.Or(*tc.s2)
		got := safeSorted(&s1)
		if !slices.Equal(got, safeSorted(tc.exp)) {
			t.Errorf("%d: %v.IUnion(%v) should be %v, but got %v", i, sl1, sl2, tc.exp, got)
		}
	}
}

func TestSetISubtract(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		s1, s2 *idset.ArraySet
		exp    []id.Zid
	}{
		{makeNew(), makeNew(), nil},
		{collect([]id.Zid{1}), makeNew(), []id.Zid{1}},
		{makeNew(), collect([]id.Zid{1}), nil},
		{collect([]id.Zid{1}), collect([]id.Zid{2}), []id.Zid{1}},
		{collect([]id.Zid{2}), collect([]id.Zid{1}), []id.Zid{2}},
		{collect([]id.Zid{1}), collect([]id.Zid{1}), nil},
		{collect([]id.Zid{1, 2, 3}), collect([]id.Zid{1}), []id.Zid{2, 3}},
		{collect([]id.Zid{1, 2, 3}), collect([]id.Zid{2}), []id.Zid{1, 3}},
		{collect([]id.Zid{1, 2, 3}), collect([]id.Zid{3}), []id.Zid{1, 2}},
		{collect([]id.Zid{1, 2, 3}), collect([]id.Zid{1, 2}), []id.Zid{3}},
		{collect([]id.Zid{1, 2, 3}), collect([]id.Zid{1, 3}), []id.Zid{2}},
		{collect([]id.Zid{1, 2, 3}), collect([]id.Zid{2, 3}), []id.Zid{1}},
	}
	for i, tc := range testcases {
		s1 := tc.s1.Clone()
		sl1 := safeSorted(&s1)
		sl2 := safeSorted(tc.s2)
		s1.AndNot(*tc.s2)
		got := safeSorted(&s1)
		if !slices.Equal(got, tc.exp) {
			t.Errorf("%d: %v.ISubstract(%v) should be %v, but got %v", i, sl1, sl2, tc.exp, got)
		}
	}
}

func TestSetDiff(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		in1, in2   *idset.ArraySet
		exp1, exp2 *idset.ArraySet
	}{
		{collect([]id.Zid{}), collect([]id.Zid{}), collect([]id.Zid{}), collect([]id.Zid{})},
		{collect([]id.Zid{1}), collect([]id.Zid{}), nil, collect([]id.Zid{1})},
		{collect([]id.Zid{}), collect([]id.Zid{1}), collect([]id.Zid{1}), nil},
		{collect([]id.Zid{1}), collect([]id.Zid{1}), nil, nil},
		{collect([]id.Zid{1, 2}), collect([]id.Zid{1}), nil, collect([]id.Zid{2})},
		{collect([]id.Zid{1}), collect([]id.Zid{1, 2}), collect([]id.Zid{2}), nil},
		{collect([]id.Zid{1, 2}), collect([]id.Zid{1, 3}), collect([]id.Zid{3}), collect([]id.Zid{2})},
		{collect([]id.Zid{1, 2, 3}), collect([]id.Zid{2, 3, 4}), collect([]id.Zid{4}), collect([]id.Zid{1})},
		{collect([]id.Zid{2, 3, 4}), collect([]id.Zid{1, 2, 3}), collect([]id.Zid{1}), collect([]id.Zid{4})},
	}
	for i, tc := range testcases {
		gotO, gotN := tc.in1.Delta(*tc.in2)
		if exp := safeSorted(tc.exp1); !slices.Equal(exp, safeSorted(&gotN)) {
			t.Errorf("%d: expected %v, but got: %v", i, tc.exp1, gotN)
		}
		if exp := safeSorted(tc.exp2); !slices.Equal(exp, safeSorted(&gotO)) {
			t.Errorf("%d: expected %v, but got: %v", i, tc.exp2, gotO)
		}
	}
}

func TestSetRemove(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		s1, s2 *idset.ArraySet
		exp    []id.Zid
	}{
		{makeNew(), makeNew(), nil},
		{collect([]id.Zid{1}), makeNew(), []id.Zid{1}},
		{collect([]id.Zid{1}), collect([]id.Zid{2}), []id.Zid{1}},
		{collect([]id.Zid{1}), collect([]id.Zid{1}), []id.Zid{}},
	}
	for i, tc := range testcases {
		sl1 := safeSorted(tc.s1)
		sl2 := safeSorted(tc.s2)
		newS1 := collect(sl1)
		newS1.AndNot(*tc.s2)
		got := safeSorted(newS1)
		if !slices.Equal(got, tc.exp) {
			t.Errorf("%d: %v.Remove(%v) should be %v, but got %v", i, sl1, sl2, tc.exp, got)
		}
	}
}

func BenchmarkSet(b *testing.B) {
	s := idset.New()
	s.Grow(b.N)
	for i := range b.N {
		s.Insert(id.Zid(i))
	}
}
