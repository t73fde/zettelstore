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

// Package mapstore stored the index in main memory via a Go map.
package mapstore

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/maps"
	"zettelstore.de/z/box"
	"zettelstore.de/z/box/manager/store"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

type zettelData struct {
	meta      *meta.Meta // a local copy of the metadata, without computed keys
	dead      *id.Set    // set of dead references in this zettel
	forward   *id.Set    // set of forward references in this zettel
	backward  *id.Set    // set of zettel that reference with zettel
	otherRefs map[string]bidiRefs
	words     []string // list of words of this zettel
	urls      []string // list of urls of this zettel
}

type bidiRefs struct {
	forward  *id.Set
	backward *id.Set
}

func (zd *zettelData) optimize() {
	zd.dead.Optimize()
	zd.forward.Optimize()
	zd.backward.Optimize()
	for _, bidi := range zd.otherRefs {
		bidi.forward.Optimize()
		bidi.backward.Optimize()
	}
}

type mapStore struct {
	mx     sync.RWMutex
	intern map[string]string // map to intern strings
	idx    map[id.Zid]*zettelData
	dead   map[id.Zid]*id.Set // map dead refs where they occur
	words  stringRefs
	urls   stringRefs

	// Stats
	mxStats sync.Mutex
	updates uint64
}
type stringRefs map[string]*id.Set

// New returns a new memory-based index store.
func New() store.Store {
	return &mapStore{
		intern: make(map[string]string, 1024),
		idx:    make(map[id.Zid]*zettelData),
		dead:   make(map[id.Zid]*id.Set),
		words:  make(stringRefs),
		urls:   make(stringRefs),
	}
}

func (ms *mapStore) GetMeta(_ context.Context, zid id.Zid) (*meta.Meta, error) {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	if zi, found := ms.idx[zid]; found && zi.meta != nil {
		// zi.meta is nil, if zettel was referenced, but is not indexed yet.
		return zi.meta.Clone(), nil
	}
	return nil, box.ErrZettelNotFound{Zid: zid}
}

func (ms *mapStore) Enrich(_ context.Context, m *meta.Meta) {
	if ms.doEnrich(m) {
		ms.mxStats.Lock()
		ms.updates++
		ms.mxStats.Unlock()
	}
}

func (ms *mapStore) doEnrich(m *meta.Meta) bool {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	zi, ok := ms.idx[m.Zid]
	if !ok {
		return false
	}
	var updated bool
	if !zi.dead.IsEmpty() {
		m.Set(api.KeyDead, zi.dead.MetaString())
		updated = true
	}
	back := removeOtherMetaRefs(m, zi.backward.Clone())
	if !zi.backward.IsEmpty() {
		m.Set(api.KeyBackward, zi.backward.MetaString())
		updated = true
	}
	if !zi.forward.IsEmpty() {
		m.Set(api.KeyForward, zi.forward.MetaString())
		back.ISubstract(zi.forward)
		updated = true
	}
	for k, refs := range zi.otherRefs {
		if !refs.backward.IsEmpty() {
			m.Set(k, refs.backward.MetaString())
			back.ISubstract(refs.backward)
			updated = true
		}
	}
	if !back.IsEmpty() {
		m.Set(api.KeyBack, back.MetaString())
		updated = true
	}
	return updated
}

// SearchEqual returns all zettel that contains the given exact word.
// The word must be normalized through Unicode NKFD, trimmed and not empty.
func (ms *mapStore) SearchEqual(word string) *id.Set {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	result := id.NewSet()
	if refs, ok := ms.words[word]; ok {
		result = result.IUnion(refs)
	}
	if refs, ok := ms.urls[word]; ok {
		result = result.IUnion(refs)
	}
	zid, err := id.Parse(word)
	if err != nil {
		return result
	}
	zi, ok := ms.idx[zid]
	if !ok {
		return result
	}

	return addBackwardZids(result, zid, zi)
}

// SearchPrefix returns all zettel that have a word with the given prefix.
// The prefix must be normalized through Unicode NKFD, trimmed and not empty.
func (ms *mapStore) SearchPrefix(prefix string) *id.Set {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	result := ms.selectWithPred(prefix, strings.HasPrefix)
	l := len(prefix)
	if l > 14 {
		return result
	}
	maxZid, err := id.Parse(prefix + "99999999999999"[:14-l])
	if err != nil {
		return result
	}
	var minZid id.Zid
	if l < 14 && prefix == "0000000000000"[:l] {
		minZid = id.Zid(1)
	} else {
		minZid, err = id.Parse(prefix + "00000000000000"[:14-l])
		if err != nil {
			return result
		}
	}
	for zid, zi := range ms.idx {
		if minZid <= zid && zid <= maxZid {
			result = addBackwardZids(result, zid, zi)
		}
	}
	return result
}

// SearchSuffix returns all zettel that have a word with the given suffix.
// The suffix must be normalized through Unicode NKFD, trimmed and not empty.
func (ms *mapStore) SearchSuffix(suffix string) *id.Set {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	result := ms.selectWithPred(suffix, strings.HasSuffix)
	l := len(suffix)
	if l > 14 {
		return result
	}
	val, err := id.ParseUint(suffix)
	if err != nil {
		return result
	}
	modulo := uint64(1)
	for range l {
		modulo *= 10
	}
	for zid, zi := range ms.idx {
		if uint64(zid)%modulo == val {
			result = addBackwardZids(result, zid, zi)
		}
	}
	return result
}

// SearchContains returns all zettel that contains the given string.
// The string must be normalized through Unicode NKFD, trimmed and not empty.
func (ms *mapStore) SearchContains(s string) *id.Set {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	result := ms.selectWithPred(s, strings.Contains)
	if len(s) > 14 {
		return result
	}
	if _, err := id.ParseUint(s); err != nil {
		return result
	}
	for zid, zi := range ms.idx {
		if strings.Contains(zid.String(), s) {
			result = addBackwardZids(result, zid, zi)
		}
	}
	return result
}

func (ms *mapStore) selectWithPred(s string, pred func(string, string) bool) *id.Set {
	// Must only be called if ms.mx is read-locked!
	result := id.NewSet()
	for word, refs := range ms.words {
		if !pred(word, s) {
			continue
		}
		result.IUnion(refs)
	}
	for u, refs := range ms.urls {
		if !pred(u, s) {
			continue
		}
		result.IUnion(refs)
	}
	return result
}

func addBackwardZids(result *id.Set, zid id.Zid, zi *zettelData) *id.Set {
	// Must only be called if ms.mx is read-locked!
	result = result.Add(zid)
	result = result.IUnion(zi.backward)
	for _, mref := range zi.otherRefs {
		result = result.IUnion(mref.backward)
	}
	return result
}

func removeOtherMetaRefs(m *meta.Meta, back *id.Set) *id.Set {
	for _, p := range m.PairsRest() {
		switch meta.Type(p.Key) {
		case meta.TypeID:
			if zid, err := id.Parse(p.Value); err == nil {
				back = back.Remove(zid)
			}
		case meta.TypeIDSet:
			for _, val := range meta.ListFromValue(p.Value) {
				if zid, err := id.Parse(val); err == nil {
					back = back.Remove(zid)
				}
			}
		}
	}
	return back
}

func (ms *mapStore) UpdateReferences(_ context.Context, zidx *store.ZettelIndex) *id.Set {
	ms.mx.Lock()
	defer ms.mx.Unlock()
	m := ms.makeMeta(zidx)
	zi, ziExist := ms.idx[zidx.Zid]
	if !ziExist || zi == nil {
		zi = &zettelData{}
		ziExist = false
	}

	// Is this zettel an old dead reference mentioned in other zettel?
	var toCheck *id.Set
	if refs, ok := ms.dead[zidx.Zid]; ok {
		// These must be checked later again
		toCheck = refs
		delete(ms.dead, zidx.Zid)
	}

	zi.meta = m
	ms.updateDeadReferences(zidx, zi)
	ids := ms.updateForwardBackwardReferences(zidx, zi)
	toCheck = toCheck.IUnion(ids)
	ids = ms.updateMetadataReferences(zidx, zi)
	toCheck = toCheck.IUnion(ids)
	zi.words = updateStrings(zidx.Zid, ms.words, zi.words, zidx.GetWords())
	zi.urls = updateStrings(zidx.Zid, ms.urls, zi.urls, zidx.GetUrls())

	// Check if zi must be inserted into ms.idx
	if !ziExist {
		ms.idx[zidx.Zid] = zi
	}
	zi.optimize()
	return toCheck
}

var internableKeys = map[string]bool{
	api.KeyRole:      true,
	api.KeySyntax:    true,
	api.KeyFolgeRole: true,
	api.KeyLang:      true,
	api.KeyReadOnly:  true,
}

func isInternableValue(key string) bool {
	if internableKeys[key] {
		return true
	}
	return strings.HasSuffix(key, meta.SuffixKeyRole)
}

func (ms *mapStore) internString(s string) string {
	if is, found := ms.intern[s]; found {
		return is
	}
	ms.intern[s] = s
	return s
}

func (ms *mapStore) makeMeta(zidx *store.ZettelIndex) *meta.Meta {
	origM := zidx.GetMeta()
	copyM := meta.New(origM.Zid)
	for _, p := range origM.Pairs() {
		key := ms.internString(p.Key)
		if isInternableValue(key) {
			copyM.Set(key, ms.internString(p.Value))
		} else if key == api.KeyBoxNumber || !meta.IsComputed(key) {
			copyM.Set(key, p.Value)
		}
	}
	return copyM
}

func (ms *mapStore) updateDeadReferences(zidx *store.ZettelIndex, zi *zettelData) {
	// Must only be called if ms.mx is write-locked!
	drefs := zidx.GetDeadRefs()
	newRefs, remRefs := zi.dead.Diff(drefs)
	zi.dead = drefs
	remRefs.ForEach(func(ref id.Zid) {
		ms.dead[ref] = ms.dead[ref].Remove(zidx.Zid)
	})
	newRefs.ForEach(func(ref id.Zid) {
		ms.dead[ref] = ms.dead[ref].Add(zidx.Zid)
	})
}

func (ms *mapStore) updateForwardBackwardReferences(zidx *store.ZettelIndex, zi *zettelData) *id.Set {
	// Must only be called if ms.mx is write-locked!
	brefs := zidx.GetBackRefs()
	newRefs, remRefs := zi.forward.Diff(brefs)
	zi.forward = brefs

	var toCheck *id.Set
	remRefs.ForEach(func(ref id.Zid) {
		bzi := ms.getOrCreateEntry(ref)
		bzi.backward = bzi.backward.Remove(zidx.Zid)
		if bzi.meta == nil {
			toCheck = toCheck.Add(ref)
		}
	})
	newRefs.ForEach(func(ref id.Zid) {
		bzi := ms.getOrCreateEntry(ref)
		bzi.backward = bzi.backward.Add(zidx.Zid)
		if bzi.meta == nil {
			toCheck = toCheck.Add(ref)
		}
	})
	return toCheck
}

func (ms *mapStore) updateMetadataReferences(zidx *store.ZettelIndex, zi *zettelData) *id.Set {
	// Must only be called if ms.mx is write-locked!
	inverseRefs := zidx.GetInverseRefs()
	for key, mr := range zi.otherRefs {
		if _, ok := inverseRefs[key]; ok {
			continue
		}
		ms.removeInverseMeta(zidx.Zid, key, mr.forward)
	}
	if zi.otherRefs == nil {
		zi.otherRefs = make(map[string]bidiRefs)
	}
	var toCheck *id.Set
	for key, mrefs := range inverseRefs {
		mr := zi.otherRefs[key]
		newRefs, remRefs := mr.forward.Diff(mrefs)
		mr.forward = mrefs
		zi.otherRefs[key] = mr

		newRefs.ForEach(func(ref id.Zid) {
			bzi := ms.getOrCreateEntry(ref)
			if bzi.otherRefs == nil {
				bzi.otherRefs = make(map[string]bidiRefs)
			}
			bmr := bzi.otherRefs[key]
			bmr.backward = bmr.backward.Add(zidx.Zid)
			bzi.otherRefs[key] = bmr
			if bzi.meta == nil {
				toCheck = toCheck.Add(ref)
			}
		})

		ms.removeInverseMeta(zidx.Zid, key, remRefs)
	}
	return toCheck
}

func updateStrings(zid id.Zid, srefs stringRefs, prev []string, next store.WordSet) []string {
	newWords, removeWords := next.Diff(prev)
	for _, word := range newWords {
		srefs[word] = srefs[word].Add(zid)
	}
	for _, word := range removeWords {
		refs, ok := srefs[word]
		if !ok {
			continue
		}
		refs = refs.Remove(zid)
		if refs.IsEmpty() {
			delete(srefs, word)
			continue
		}
		srefs[word] = refs
	}
	return next.Words()
}

func (ms *mapStore) getOrCreateEntry(zid id.Zid) *zettelData {
	// Must only be called if ms.mx is write-locked!
	if zi, ok := ms.idx[zid]; ok {
		return zi
	}
	zi := &zettelData{}
	ms.idx[zid] = zi
	return zi
}

func (ms *mapStore) RenameZettel(_ context.Context, curZid, newZid id.Zid) *id.Set {
	ms.mx.Lock()
	defer ms.mx.Unlock()

	curZi, curFound := ms.idx[curZid]
	_, newFound := ms.idx[newZid]
	if !curFound || newFound {
		return nil
	}
	newZi := &zettelData{
		meta:      copyMeta(curZi.meta, newZid),
		dead:      ms.copyDeadReferences(curZi.dead),
		forward:   ms.copyForward(curZi.forward, newZid),
		backward:  nil, // will be done through tocheck
		otherRefs: nil, // TODO: check if this will be done through toCheck
		words:     copyStrings(ms.words, curZi.words, newZid),
		urls:      copyStrings(ms.urls, curZi.urls, newZid),
	}

	ms.idx[newZid] = newZi
	toCheck := ms.doDeleteZettel(curZid)
	toCheck = toCheck.IUnion(ms.dead[newZid])
	delete(ms.dead, newZid)
	toCheck = toCheck.Add(newZid) // should update otherRefs
	return toCheck
}
func copyMeta(m *meta.Meta, newZid id.Zid) *meta.Meta {
	result := m.Clone()
	result.Zid = newZid
	return result
}

func (ms *mapStore) copyDeadReferences(curDead *id.Set) *id.Set {
	// Must only be called if ms.mx is write-locked!
	curDead.ForEach(func(ref id.Zid) {
		ms.dead[ref] = ms.dead[ref].Add(ref)
	})
	return curDead.Clone()
}
func (ms *mapStore) copyForward(curForward *id.Set, newZid id.Zid) *id.Set {
	// Must only be called if ms.mx is write-locked!
	curForward.ForEach(func(ref id.Zid) {
		if fzi, found := ms.idx[ref]; found {
			fzi.backward = fzi.backward.Add(newZid)
		}

	})
	return curForward.Clone()
}

func copyStrings(msStringMap stringRefs, curStrings []string, newZid id.Zid) []string {
	// Must only be called if ms.mx is write-locked!
	if l := len(curStrings); l > 0 {
		result := make([]string, l)
		for i, s := range curStrings {
			result[i] = s
			msStringMap[s] = msStringMap[s].Add(newZid)
		}
		return result
	}
	return nil
}

func (ms *mapStore) DeleteZettel(_ context.Context, zid id.Zid) *id.Set {
	ms.mx.Lock()
	defer ms.mx.Unlock()
	return ms.doDeleteZettel(zid)
}

func (ms *mapStore) doDeleteZettel(zid id.Zid) *id.Set {
	// Must only be called if ms.mx is write-locked!
	zi, ok := ms.idx[zid]
	if !ok {
		return nil
	}

	ms.deleteDeadSources(zid, zi)
	toCheck := ms.deleteForwardBackward(zid, zi)
	for key, mrefs := range zi.otherRefs {
		ms.removeInverseMeta(zid, key, mrefs.forward)
	}
	deleteStrings(ms.words, zi.words, zid)
	deleteStrings(ms.urls, zi.urls, zid)
	delete(ms.idx, zid)
	return toCheck
}

func (ms *mapStore) deleteDeadSources(zid id.Zid, zi *zettelData) {
	// Must only be called if ms.mx is write-locked!
	zi.dead.ForEach(func(ref id.Zid) {
		if drefs, ok := ms.dead[ref]; ok {
			if drefs = drefs.Remove(zid); drefs.IsEmpty() {
				delete(ms.dead, ref)
			} else {
				ms.dead[ref] = drefs
			}
		}
	})
}

func (ms *mapStore) deleteForwardBackward(zid id.Zid, zi *zettelData) *id.Set {
	// Must only be called if ms.mx is write-locked!
	zi.forward.ForEach(func(ref id.Zid) {
		if fzi, ok := ms.idx[ref]; ok {
			fzi.backward = fzi.backward.Remove(zid)
		}
	})

	var toCheck *id.Set
	zi.backward.ForEach(func(ref id.Zid) {
		if bzi, ok := ms.idx[ref]; ok {
			bzi.forward = bzi.forward.Remove(zid)
			toCheck = toCheck.Add(ref)
		}
	})
	return toCheck
}

func (ms *mapStore) removeInverseMeta(zid id.Zid, key string, forward *id.Set) {
	// Must only be called if ms.mx is write-locked!
	forward.ForEach(func(ref id.Zid) {
		bzi, ok := ms.idx[ref]
		if !ok || bzi.otherRefs == nil {
			return
		}
		bmr, ok := bzi.otherRefs[key]
		if !ok {
			return
		}
		bmr.backward = bmr.backward.Remove(zid)
		if !bmr.backward.IsEmpty() || !bmr.forward.IsEmpty() {
			bzi.otherRefs[key] = bmr
		} else {
			delete(bzi.otherRefs, key)
			if len(bzi.otherRefs) == 0 {
				bzi.otherRefs = nil
			}
		}
	})
}

func deleteStrings(msStringMap stringRefs, curStrings []string, zid id.Zid) {
	// Must only be called if ms.mx is write-locked!
	for _, word := range curStrings {
		refs, ok := msStringMap[word]
		if !ok {
			continue
		}
		refs = refs.Remove(zid)
		if refs.IsEmpty() {
			delete(msStringMap, word)
			continue
		}
		msStringMap[word] = refs
	}
}

func (ms *mapStore) Optimize() {
	ms.mx.Lock()
	defer ms.mx.Unlock()

	// No need to optimize ms.idx: is already done via ms.UpdateReferences
	for _, dead := range ms.dead {
		dead.Optimize()
	}
	for _, s := range ms.words {
		s.Optimize()
	}
	for _, s := range ms.urls {
		s.Optimize()
	}
}

func (ms *mapStore) ReadStats(st *store.Stats) {
	ms.mx.RLock()
	st.Zettel = len(ms.idx)
	st.Words = uint64(len(ms.words))
	st.Urls = uint64(len(ms.urls))
	ms.mx.RUnlock()
	ms.mxStats.Lock()
	st.Updates = ms.updates
	ms.mxStats.Unlock()
}

func (ms *mapStore) Dump(w io.Writer) {
	ms.mx.RLock()
	defer ms.mx.RUnlock()

	io.WriteString(w, "=== Dump\n")
	ms.dumpIndex(w)
	ms.dumpDead(w)
	dumpStringRefs(w, "Words", "", "", ms.words)
	dumpStringRefs(w, "URLs", "[[", "]]", ms.urls)
}

func (ms *mapStore) dumpIndex(w io.Writer) {
	if len(ms.idx) == 0 {
		return
	}
	io.WriteString(w, "==== Zettel Index\n")
	zids := make(id.Slice, 0, len(ms.idx))
	for id := range ms.idx {
		zids = append(zids, id)
	}
	zids.Sort()
	for _, id := range zids {
		fmt.Fprintln(w, "=====", id)
		zi := ms.idx[id]
		if !zi.dead.IsEmpty() {
			fmt.Fprintln(w, "* Dead:", zi.dead)
		}
		dumpSet(w, "* Forward:", zi.forward)
		dumpSet(w, "* Backward:", zi.backward)

		otherRefs := make([]string, 0, len(zi.otherRefs))
		for k := range zi.otherRefs {
			otherRefs = append(otherRefs, k)
		}
		slices.Sort(otherRefs)
		for _, k := range otherRefs {
			fmt.Fprintln(w, "* Meta", k)
			dumpSet(w, "** Forward:", zi.otherRefs[k].forward)
			dumpSet(w, "** Backward:", zi.otherRefs[k].backward)
		}
		dumpStrings(w, "* Words", "", "", zi.words)
		dumpStrings(w, "* URLs", "[[", "]]", zi.urls)
	}
}

func (ms *mapStore) dumpDead(w io.Writer) {
	if len(ms.dead) == 0 {
		return
	}
	fmt.Fprintf(w, "==== Dead References\n")
	zids := make(id.Slice, 0, len(ms.dead))
	for id := range ms.dead {
		zids = append(zids, id)
	}
	zids.Sort()
	for _, id := range zids {
		fmt.Fprintln(w, ";", id)
		fmt.Fprintln(w, ":", ms.dead[id])
	}
}

func dumpSet(w io.Writer, prefix string, s *id.Set) {
	if !s.IsEmpty() {
		io.WriteString(w, prefix)
		s.ForEach(func(zid id.Zid) {
			io.WriteString(w, " ")
			w.Write(zid.Bytes())
		})
		fmt.Fprintln(w)
	}
}
func dumpStrings(w io.Writer, title, preString, postString string, slice []string) {
	if len(slice) > 0 {
		sl := make([]string, len(slice))
		copy(sl, slice)
		slices.Sort(sl)
		fmt.Fprintln(w, title)
		for _, s := range sl {
			fmt.Fprintf(w, "** %s%s%s\n", preString, s, postString)
		}
	}
}

func dumpStringRefs(w io.Writer, title, preString, postString string, srefs stringRefs) {
	if len(srefs) == 0 {
		return
	}
	fmt.Fprintln(w, "====", title)
	for _, s := range maps.Keys(srefs) {
		fmt.Fprintf(w, "; %s%s%s\n", preString, s, postString)
		fmt.Fprintln(w, ":", srefs[s])
	}
}
