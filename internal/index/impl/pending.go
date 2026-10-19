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

type pendingQueue struct {
	mx           sync.Mutex
	first        *pendingBatch
	last         *pendingBatch
	maxLoad      int
	resetPending bool   // a pendingReset is to be delivered first
	generation   uint64 // number of accepted resets and reloads so far
}

type pendingBatch struct {
	next       *pendingBatch
	members    memberSet
	generation uint64 // >0: reload batch created in that generation; 0: individual zettel
}
type memberSet map[id.Zid]struct{}

func (pb *pendingBatch) has(zid id.Zid) bool {
	_, found := pb.members[zid]
	return found
}
func (pb *pendingBatch) pop() id.Zid {
	for zid := range pb.members {
		delete(pb.members, zid)
		return zid
	}
	return id.Invalid
}
func (pb *pendingBatch) isReload() bool { return pb.generation != 0 }

func (pq *pendingQueue) reset() {
	pq.mx.Lock()
	defer pq.mx.Unlock()
	pq.generation++
	pq.first, pq.last = nil, nil
	pq.resetPending = true
}

func (pq *pendingQueue) addZettel(zid id.Zid) {
	if !zid.IsValid() {
		return
	}
	pq.mx.Lock()
	defer pq.mx.Unlock()

	for batch := pq.first; batch != nil; batch = batch.next {
		if batch.isReload() {
			continue // Do not put zettel in reload batch
		}
		if batch.has(zid) {
			return // Zettel is already waiting
		}
	}
	if batch := pq.last; batch != nil && !batch.isReload() && (pq.maxLoad == 0 || len(batch.members) < pq.maxLoad) {
		batch.members[zid] = struct{}{}
		return
	}
	batch := &pendingBatch{members: memberSet{zid: {}}} // generation 0: individual zettel
	if pq.last == nil {
		pq.first = batch
	} else {
		pq.last.next = batch
	}
	pq.last = batch
}

func (pq *pendingQueue) reload(allZids memberSet) {
	var members memberSet
	if len(allZids) > 0 {
		members = maps.Clone(memberSet(allZids)) // do not share the caller's map
	}

	pq.mx.Lock()
	defer pq.mx.Unlock()
	if pq.generation == 0 {
		pq.generation = 1 // reload without preceding reset: start the first generation
	}
	if members == nil {
		pq.first, pq.last = nil, nil
		return
	}
	batch := pq.first
	for batch != nil && batch.isReload() {
		batch = batch.next
	}
	pq.first = &pendingBatch{next: batch, members: members, generation: pq.generation}
	if batch == nil {
		pq.last = pq.first
	}
}

// ----- consuming side of pendingQueue --------------------------------------

// pendingJob is the result of getJob.
type pendingJob struct {
	zid    id.Zid
	action pendingAction

	// pendingReset: generation of the reset;
	// pendingZettel: generation of the reload batch, 0 for an individual zettel
	generation uint64
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

	// A pending reset is delivered first.
	if pq.resetPending {
		pq.resetPending = false
		return pendingJob{action: pendingReset, generation: pq.generation}
	}

	for batch := pq.first; batch != nil; batch = pq.first {
		if len(batch.members) == 0 {
			pq.removeFirst()
			continue
		}
		zid := batch.pop()
		if len(batch.members) == 0 {
			pq.removeFirst()
		}
		return pendingJob{zid: zid, action: pendingZettel, generation: batch.generation}
	}
	return pendingJob{action: pendingNothing}
}
func (pq *pendingQueue) removeFirst() {
	pq.first = pq.first.next
	if pq.first == nil {
		pq.last = nil
	}
}
func (j pendingJob) fromReload() bool { return j.action == pendingZettel && j.generation != 0 }
