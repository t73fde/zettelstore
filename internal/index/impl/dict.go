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

type denseZid uint32

// dictionary maps zettel identifier to dense identifier and words to word
// identifier. Identifier are never given back; a dense identifier may
// therefore refer to a zettel that does not (yet) exist. The store decides
// about existence. All methods are safe for concurrent use.
type dictionary struct {
	mx    sync.RWMutex
	dense *dense.Dense[id.Zid, denseZid]
	voc   *vocab.Vocabulary

	// inverse maps metadata keys to their inverse. It is read-only after
	// construction. Assumes that there will be no future inverse keys.
	inverse map[vocab.WordID]vocab.WordID
}

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
	return &dictionary{
		dense:   &dense.Dense[id.Zid, denseZid]{},
		voc:     voc,
		inverse: inverse,
	}
}

func (d *dictionary) Encode(zid id.Zid) denseZid {
	d.mx.Lock()
	defer d.mx.Unlock()
	return d.dense.Encode(zid)
}

func (d *dictionary) Decode(dzid denseZid) (id.Zid, bool) {
	d.mx.RLock()
	defer d.mx.RUnlock()
	return d.dense.Decode(dzid)
}

func (d *dictionary) AddBytes(b []byte) vocab.WordID {
	d.mx.Lock()
	defer d.mx.Unlock()
	return d.voc.AddBytes(b)
}

func (d *dictionary) AddString(s string) vocab.WordID {
	d.mx.Lock()
	defer d.mx.Unlock()
	return d.voc.AddString(s)
}

func (d *dictionary) numWords() int {
	d.mx.RLock()
	defer d.mx.RUnlock()
	return d.voc.Count()
}

func (d *dictionary) readStats(st *dictStats) {
	d.mx.RLock()
	defer d.mx.RUnlock()
	st.NumWords = d.voc.Count()
	st.Vocabulary = d.voc.StoreLen()
	st.WordCount = d.voc.WordCount()
}

type dictStats struct {
	NumWords   int
	Vocabulary int
	WordCount  map[uint16]int
}
