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
	"maps"
	"sync"

	"t73f.de/r/zsc/domain/id"
)

type memberSet map[id.Zid]struct{}

// pendingQueue is a set of zettel to be synchronized with the index.
// The content is retrieved when a job is taken, not when it is queued.
type pendingQueue struct {
	mx           sync.Mutex
	single       memberSet // individually notified zettel
	bulk         memberSet // zettel of the current reload
	resetPending bool      // a reset job is to be delivered first
	generation   uint64    // number of accepted resets
}

func (pq *pendingQueue) reset() {
	pq.mx.Lock()
	defer pq.mx.Unlock()

	pq.generation++
	clear(pq.single)
	clear(pq.bulk)
	pq.resetPending = true
}

func (pq *pendingQueue) addZettel(zid id.Zid) {
	if !zid.IsValid() {
		return
	}
	pq.mx.Lock()
	defer pq.mx.Unlock()

	delete(pq.bulk, zid) // retrieved anew anyway; moves to the individual set
	if pq.single == nil {
		pq.single = make(memberSet)
	}
	pq.single[zid] = struct{}{}
}

func (pq *pendingQueue) reload(allZids memberSet) {
	bulk := maps.Clone(allZids) // do not share the caller's map

	pq.mx.Lock()
	defer pq.mx.Unlock()
	if pq.generation == 0 {
		pq.generation = 1 // reload without preceding reset: first generation
	}
	for zid := range pq.single {
		delete(bulk, zid)
	}
	pq.bulk = bulk
}

type pendingJob struct {
	zid        id.Zid
	generation uint64 // current generation when the job was taken
	action     pendingAction
	fromReload bool
}

type pendingAction uint8

const (
	pendingNothing pendingAction = iota
	pendingZettel
	pendingReset
)

func (pq *pendingQueue) getJob() pendingJob {
	pq.mx.Lock()
	defer pq.mx.Unlock()

	if pq.resetPending {
		pq.resetPending = false
		return pendingJob{action: pendingReset, generation: pq.generation}
	}
	if zid, ok := popAny(pq.single); ok {
		return pendingJob{zid: zid, generation: pq.generation, action: pendingZettel}
	}
	if zid, ok := popAny(pq.bulk); ok {
		return pendingJob{zid: zid, generation: pq.generation, action: pendingZettel, fromReload: true}
	}
	return pendingJob{}
}

func popAny(ms memberSet) (id.Zid, bool) {
	for zid := range ms {
		delete(ms, zid)
		return zid, true
	}
	return id.Invalid, false
}
