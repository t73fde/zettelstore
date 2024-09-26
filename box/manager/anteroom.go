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

package manager

import (
	"sync"

	"zettelstore.de/z/zettel/id"
)

type arAction int

const (
	arNothing arAction = iota
	arReload
	arZettel
)

type anteroom struct {
	next    *anteroom
	waiting *id.Set
	curLoad int
	reload  bool
}

type anteroomQueue struct {
	mx      sync.Mutex
	first   *anteroom
	last    *anteroom
	maxLoad int
}

func newAnteroomQueue(maxLoad int) *anteroomQueue { return &anteroomQueue{maxLoad: maxLoad} }

func (ar *anteroomQueue) EnqueueZettel(zid id.Zid) {
	if !zid.IsValid() {
		return
	}
	ar.mx.Lock()
	defer ar.mx.Unlock()
	if ar.first == nil {
		ar.first = ar.makeAnteroom(zid)
		ar.last = ar.first
		return
	}
	for room := ar.first; room != nil; room = room.next {
		if room.reload {
			continue // Do not put zettel in reload room
		}
		if room.waiting.Contains(zid) {
			// Zettel is already waiting. Nothing to do.
			return
		}
	}
	if room := ar.last; !room.reload && (ar.maxLoad == 0 || room.curLoad < ar.maxLoad) {
		room.waiting.Add(zid)
		room.curLoad++
		return
	}
	room := ar.makeAnteroom(zid)
	ar.last.next = room
	ar.last = room
}

func (ar *anteroomQueue) makeAnteroom(zid id.Zid) *anteroom {
	if zid == id.Invalid {
		panic(zid)
	}
	waiting := id.NewSetCap(max(ar.maxLoad, 100), zid)
	return &anteroom{next: nil, waiting: waiting, curLoad: 1, reload: false}
}

func (ar *anteroomQueue) Reset() {
	ar.mx.Lock()
	defer ar.mx.Unlock()
	ar.first = &anteroom{next: nil, waiting: nil, curLoad: 0, reload: true}
	ar.last = ar.first
}

func (ar *anteroomQueue) Reload(allZids *id.Set) {
	ar.mx.Lock()
	defer ar.mx.Unlock()
	ar.deleteReloadedRooms()

	if !allZids.IsEmpty() {
		ar.first = &anteroom{next: ar.first, waiting: allZids, curLoad: allZids.Length(), reload: true}
		if ar.first.next == nil {
			ar.last = ar.first
		}
	} else {
		ar.first = nil
		ar.last = nil
	}
}

func (ar *anteroomQueue) deleteReloadedRooms() {
	room := ar.first
	for room != nil && room.reload {
		room = room.next
	}
	ar.first = room
	if room == nil {
		ar.last = nil
	}
}

func (ar *anteroomQueue) Dequeue() (arAction, id.Zid, bool) {
	ar.mx.Lock()
	defer ar.mx.Unlock()
	first := ar.first
	if first != nil {
		if first.waiting == nil && first.reload {
			ar.removeFirst()
			return arReload, id.Invalid, false
		}
		if zid, found := first.waiting.Pop(); found {
			if first.waiting.IsEmpty() {
				ar.removeFirst()
			}
			return arZettel, zid, first.reload
		}
		ar.removeFirst()
	}
	return arNothing, id.Invalid, false
}

func (ar *anteroomQueue) removeFirst() {
	ar.first = ar.first.next
	if ar.first == nil {
		ar.last = nil
	}
}
