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

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/idset"
	"zettelstore.de/z/internal/index"
)

type pendingQueue struct {
	mx           sync.Mutex
	first        *pendingBatch
	last         *pendingBatch
	maxLoad      int
	resetFetcher index.Fetcher // non-nil: pendingReload to be delivered first
	generation   uint64        // number of accepted resets and reloads so far
}

type pendingBatch struct {
	next       *pendingBatch
	members    memberSet
	generation uint64 // >0: reload batch created in that generation; 0: individual zettel
}
type memberSet map[id.Zid]index.Fetcher

func (pb *pendingBatch) has(zid id.Zid) bool {
	_, found := pb.members[zid]
	return found
}
func (pb *pendingBatch) pop() (index.Fetcher, id.Zid) {
	for zid, fetcher := range pb.members {
		delete(pb.members, zid)
		return fetcher, zid
	}
	return nil, id.Invalid
}
func (pb *pendingBatch) isReload() bool { return pb.generation != 0 }

func (pq *pendingQueue) reset(fetcher index.Fetcher) {
	if fetcher == nil {
		return
	}
	pq.mx.Lock()
	defer pq.mx.Unlock()
	pq.generation++
	pq.first, pq.last = nil, nil
	pq.resetFetcher = fetcher
}

func (pq *pendingQueue) addZettel(fetcher index.Fetcher, zid id.Zid) {
	if !zid.IsValid() || fetcher == nil {
		return
	}
	pq.mx.Lock()
	defer pq.mx.Unlock()

	for batch := pq.first; batch != nil; batch = batch.next {
		if batch.isReload() {
			continue // Do not put zettel in reload batch
		}
		if batch.has(zid) {
			batch.members[zid] = fetcher // Zettel is already waiting; newer fetcher wins.
			return
		}
	}
	if batch := pq.last; batch != nil && !batch.isReload() && (pq.maxLoad == 0 || len(batch.members) < pq.maxLoad) {
		batch.members[zid] = fetcher
		return
	}
	batch := &pendingBatch{members: memberSet{zid: fetcher}} // generation 0: individual zettel
	if pq.last == nil {
		pq.first = batch
	} else {
		pq.last.next = batch
	}
	pq.last = batch
}

func (pq *pendingQueue) reload(fetcher index.Fetcher, allZids idset.ZidSet) {
	if fetcher == nil {
		return
	}
	var members memberSet
	if !allZids.IsEmpty() {
		members = make(memberSet, allZids.Count())
		for zid := range allZids.Values() {
			members[zid] = fetcher
		}
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
	fetcher index.Fetcher
	zid     id.Zid
	action  pendingAction

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
	if f := pq.resetFetcher; f != nil {
		pq.resetFetcher = nil
		return pendingJob{fetcher: f, action: pendingReset, generation: pq.generation}
	}

	for batch := pq.first; batch != nil; batch = pq.first {
		if len(batch.members) == 0 {
			pq.removeFirst()
			continue
		}
		fetcher, zid := batch.pop()
		if len(batch.members) == 0 {
			pq.removeFirst()
		}
		return pendingJob{fetcher: fetcher, zid: zid, action: pendingZettel, generation: batch.generation}
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
