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

// Package idset implements sets of zettel identifier.
package idset

import (
	"iter"
	"slices"
	"strings"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

// ArraySet is a set of zettel identifier, stored as an sorted array.
type ArraySet struct {
	seq []id.Zid
}

// String returns a string representation of the set.
func (s ArraySet) String() string {
	return "{" + s.metaString() + "}"
}

// metaString returns a string representation of the set to be stored as metadata.
func (s ArraySet) metaString() string {
	if len(s.seq) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, zid := range s.seq {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.Write(zid.Bytes())
	}
	return sb.String()
}

// MetaValue returns a metadata value representation of the set.
func (s ArraySet) MetaValue() meta.Value { return meta.Value(s.metaString()) }

// New returns a new set of identifier with the given initial values.
func New(zids ...id.Zid) *ArraySet {
	switch l := len(zids); l {
	case 0:
		return &ArraySet{seq: nil}
	case 1:
		return &ArraySet{seq: []id.Zid{zids[0]}}
	default:
		result := ArraySet{seq: make([]id.Zid, 0, l)}
		result.addSlice(zids)
		return &result
	}
}

// NewCap returns a new set of identifier with the given capacity and initial values.
func NewCap(c int, zids ...id.Zid) *ArraySet {
	result := ArraySet{seq: make([]id.Zid, 0, max(c, len(zids)))}
	result.addSlice(zids)
	return &result
}

// IsEmpty returns true, if the set conains no element.
func (s ArraySet) IsEmpty() bool { return len(s.seq) == 0 }

// Count returns the number of elements in this set.
func (s ArraySet) Count() int { return len(s.seq) }

// Clone returns a copy of the given set.
func (s *ArraySet) Clone() *ArraySet {
	if s == nil {
		return nil
	}
	return &ArraySet{seq: slices.Clone(s.seq)}
}

// Add adds a Add to the set.
func (s *ArraySet) Add(zid id.Zid) *ArraySet {
	if s == nil {
		return New(zid)
	}
	s.add(zid)
	return s
}

// Contains return true if the set is non-nil and the set contains the given Zettel identifier.
func (s ArraySet) Contains(zid id.Zid) bool {
	_, found := slices.BinarySearch(s.seq, zid)
	return found
}

// Intersection removes all elements from s that are not in o.
// Only s is modified, o is left unchanged.
func (s *ArraySet) Intersection(o ArraySet) {
	topos, spos, opos := 0, 0, 0
	for spos < len(s.seq) && opos < len(o.seq) {
		sz, oz := s.seq[spos], o.seq[opos]
		if sz < oz {
			spos++
			continue
		}
		if sz > oz {
			opos++
			continue
		}
		s.seq[topos] = sz
		topos++
		spos++
		opos++
	}
	s.seq = s.seq[:topos]
}

// IUnion adds the elements of set other to s.
func (s *ArraySet) IUnion(other *ArraySet) *ArraySet {
	if other == nil || len(other.seq) == 0 {
		return s
	}
	// TODO: if other is large enough (and s is not too small) -> optimize by swapping and/or loop through both
	return s.addSlice(other.seq)
}

// ISubstract removes all zettel identifier from 's' that are in the set 'other'.
func (s *ArraySet) ISubstract(other *ArraySet) {
	if s == nil || len(s.seq) == 0 || other == nil || len(other.seq) == 0 {
		return
	}
	topos, spos, opos := 0, 0, 0
	for spos < len(s.seq) && opos < len(other.seq) {
		sz, oz := s.seq[spos], other.seq[opos]
		if sz < oz {
			s.seq[topos] = sz
			topos++
			spos++
			continue
		}
		if sz == oz {
			spos++
		}
		opos++
	}
	for spos < len(s.seq) {
		s.seq[topos] = s.seq[spos]
		topos++
		spos++
	}
	s.seq = s.seq[:topos]
}

// Diff returns the difference sets between the two sets: the first difference
// set is the set of elements that are in other, but not in s; the second
// difference set is the set of element that are in s but not in other.
//
// in other words: the first result is the set of elements from other that must
// be added to s; the second result is the set of elements that must be removed
// from s, so that s would have the same elemest as other.
func (s *ArraySet) Diff(other *ArraySet) (newS, remS *ArraySet) {
	if s == nil || len(s.seq) == 0 {
		return other.Clone(), nil
	}
	if other == nil || len(other.seq) == 0 {
		return nil, s.Clone()
	}
	seqS, seqO := s.seq, other.seq
	var newRefs, remRefs []id.Zid
	npos, opos := 0, 0
	for npos < len(seqO) && opos < len(seqS) {
		rn, ro := seqO[npos], seqS[opos]
		if rn == ro {
			npos++
			opos++
			continue
		}
		if rn < ro {
			newRefs = append(newRefs, rn)
			npos++
			continue
		}
		remRefs = append(remRefs, ro)
		opos++
	}
	if npos < len(seqO) {
		newRefs = append(newRefs, seqO[npos:]...)
	}
	if opos < len(seqS) {
		remRefs = append(remRefs, seqS[opos:]...)
	}
	return newFromSlice(newRefs), newFromSlice(remRefs)
}

// Remove the identifier from the set.
func (s *ArraySet) Remove(zid id.Zid) *ArraySet {
	if s == nil || len(s.seq) == 0 {
		return nil
	}
	if pos, found := slices.BinarySearch(s.seq, zid); found {
		copy(s.seq[pos:], s.seq[pos+1:])
		s.seq = s.seq[:len(s.seq)-1]
	}
	if len(s.seq) == 0 {
		return nil
	}
	return s
}

// Equal returns true if the other set is equal to the given set.
func (s *ArraySet) Equal(other *ArraySet) bool {
	if s == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
	return slices.Equal(s.seq, other.seq)
}

// Values returns an iterator for each element of the set, in ascending order.
func (s *ArraySet) Values() iter.Seq[id.Zid] {
	if s == nil {
		return slices.Values([]id.Zid{})
	}
	return slices.Values(s.seq)
}

// Pop return one arbitrary element of the set.
func (s *ArraySet) Pop() (id.Zid, bool) {
	if s != nil {
		if l := len(s.seq); l > 0 {
			zid := s.seq[l-1]
			s.seq = s.seq[:l-1]
			return zid, true
		}
	}
	return id.Invalid, false
}

// Shrink the amount of memory to store the set.
func (s *ArraySet) Shrink() {
	if s != nil && cap(s.seq) > len(s.seq) {
		s.seq = slices.Clone(s.seq)
	}
}

// ----- unchecked base operations

func newFromSlice(seq []id.Zid) *ArraySet {
	if l := len(seq); l == 0 {
		return nil
	}
	return &ArraySet{seq: seq}
}

func (s *ArraySet) add(zid id.Zid) {
	if pos, found := slices.BinarySearch(s.seq, zid); !found {
		s.seq = slices.Insert(s.seq, pos, zid)
	}
}

// addSlice adds all identifier of the given slice to the set.
func (s *ArraySet) addSlice(sl []id.Zid) *ArraySet {
	if s == nil {
		return New(sl...)
	}
	s.seq = slices.Grow(s.seq, len(sl))
	for _, zid := range sl {
		s.add(zid)
	}
	return s
}
