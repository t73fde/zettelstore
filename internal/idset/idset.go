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

	"t73f.de/r/zero/roster"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

// ArraySet is a set of zettel identifier, stored as an sorted array.
type ArraySet struct {
	roster.Roster[id.Zid]
	// seq []id.Zid
}

// String returns a string representation of the set.
func (s ArraySet) String() string {
	return "{" + s.metaString() + "}"
}

// metaString returns a string representation of the set to be stored as metadata.
func (s ArraySet) metaString() string {
	return s.Roster.String()
}

// MetaValue returns a metadata value representation of the set.
func (s ArraySet) MetaValue() meta.Value { return meta.Value(s.metaString()) }

// New returns a new set of identifier with the given initial values.
func New() ArraySet { return ArraySet{Roster: roster.New[id.Zid]()} }

// IsEmpty returns true, if the set conains no element.
func (s ArraySet) IsEmpty() bool { return s.Roster.IsEmpty() }

// Count returns the number of elements in this set.
func (s ArraySet) Count() int { return s.Roster.Count() }

// Clone returns a copy of the given set.
func (s ArraySet) Clone() ArraySet {
	return ArraySet{s.Roster.Clone()}
}

// Add adds a zid to the set.
func (s *ArraySet) Add(zid id.Zid) {
	s.Roster.Insert(zid)
}

// Contains return true if the set is non-nil and the set contains the given Zettel identifier.
func (s ArraySet) Contains(zid id.Zid) bool {
	return s.Roster.Contains(zid)
}

// Intersection removes all elements from s that are not in o.
// Only s is modified, o is left unchanged.
func (s *ArraySet) Intersection(o ArraySet) {
	s.Roster.And(o.Roster)
}

// IUnion adds the elements of set other to s.
func (s *ArraySet) IUnion(other ArraySet) {
	s.Roster.Or(other.Roster)
}

// ISubstract removes all zettel identifier from 's' that are in the set 'other'.
func (s *ArraySet) ISubstract(other ArraySet) {
	s.Roster.AndNot(other.Roster)
}

// Diff returns the difference sets between the two sets: the first difference
// set is the set of elements that are in other, but not in s; the second
// difference set is the set of element that are in s but not in other.
//
// In other words: the first result is the set of elements from other that must
// be added to s; the second result is the set of elements that must be removed
// from s, so that s would have the same elemest as other.
func (s ArraySet) Diff(other ArraySet) (newS, remS ArraySet) {
	onlyS, onlyOther := s.Roster.Delta(other.Roster)
	return ArraySet{onlyOther}, ArraySet{onlyS}
}

// Remove the identifier from the set.
func (s *ArraySet) Remove(zid id.Zid) {
	s.Roster.Delete(zid)
}

// Values returns an iterator for each element of the set, in ascending order.
func (s ArraySet) Values() iter.Seq[id.Zid] {
	return s.Roster.Values()
}

// Pop return one arbitrary element of the set.
func (s *ArraySet) Pop() (id.Zid, bool) {
	return s.Roster.Pop()
}

// Grow ensures that n values can be inserted without further allocation.
// It does not insert n.
//
// Grow panics if n is negative or too large to allocate the memory
func (s *ArraySet) Grow(n int) {
	s.Roster.Grow(n)
}

// Shrink the amount of memory to store the set.
func (s *ArraySet) Shrink() {
	if s != nil {
		s.Roster.Shrink()
	}
}
