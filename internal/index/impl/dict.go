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

package impl

import (
	"sync"

	"t73f.de/r/zero/dense"
	"t73f.de/r/zero/vocab"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

type dictionary struct {
	mx    sync.RWMutex
	dense *dense.Dense[id.Zid, denseZid]
	voc   *vocab.Vocabulary

	// inverse maps metadata keys to their inverse.
	// Assumes that there will be no future inverse keys.
	inverse map[vocab.WordID]vocab.WordID
}
type denseZid uint32

func makeDictionary(wordsHint int) *dictionary {
	voc := vocab.New(wordsHint)

	inverse := make(map[vocab.WordID]vocab.WordID, 8)
	for kd := range meta.KeyDescriptionSeq() {
		keyID := voc.AddString(kd.Name)
		if invKey := kd.Inverse; invKey != "" {
			invID := voc.AddString(invKey)
			inverse[keyID] = invID
			inverse[invID] = keyID
		}
	}
	result := &dictionary{
		dense:   &dense.Dense[id.Zid, denseZid]{},
		voc:     voc,
		inverse: inverse,
	}
	return result
}

func (d *dictionary) Lock()    { d.mx.Lock() }
func (d *dictionary) Unlock()  { d.mx.Unlock() }
func (d *dictionary) RLock()   { d.mx.RLock() }
func (d *dictionary) RUnlock() { d.mx.RUnlock() }

func (d *dictionary) Encode(zid id.Zid) denseZid          { return d.dense.Encode(zid) }
func (d *dictionary) Decode(dzid denseZid) (id.Zid, bool) { return d.dense.Decode(dzid) }
func (d *dictionary) RemoveValue(dzid denseZid)           { d.dense.RemoveValue(dzid) }
func (d *dictionary) SafeEncode(zid id.Zid) denseZid {
	d.RLock()
	defer d.RUnlock()
	return d.dense.Encode(zid)
}
func (d *dictionary) SafeDecode(dzid denseZid) (id.Zid, bool) {
	d.RLock()
	defer d.RUnlock()
	return d.dense.Decode(dzid)
}

func (d *dictionary) AddBytes(b []byte) vocab.WordID  { return d.voc.AddBytes(b) }
func (d *dictionary) AddString(s string) vocab.WordID { return d.voc.AddString(s) }

func (d *dictionary) readStats(st *dictStats) {
	st.NumZettel = d.dense.Count()
	st.NumWords = d.voc.Count()
	st.Vocabulary = d.voc.StoreLen()
	st.WordCount = d.voc.WordCount()
}

type dictStats struct {
	NumZettel  int
	NumWords   int
	Vocabulary int
	WordCount  map[uint16]int
}
