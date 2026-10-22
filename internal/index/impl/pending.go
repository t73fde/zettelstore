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
	"sync/atomic"

	"t73f.de/r/zsc/domain/id"
)

type memberSet map[id.Zid]struct{}

// bulkResult summarizes a completed bulk load.
type bulkResult struct {
	total  int // number of zettel known when the bulk was fetched
	failed int // number of bulk zettel that could not be processed
}

// pendingQueue is a set of zettel to be synchronized with the index of one
// pipeline instance. The content is retrieved when a job is taken, not when
// it is queued.
//
// A queue starts with a fetch job that loads the identifier of all zettel
// (the bulk). The bulk is complete when it is loaded, no bulk zettel is left
// and every bulk job that was handed out has been finished.
type pendingQueue struct {
	mx     sync.Mutex
	single memberSet // individually notified zettel
	bulk   memberSet // zettel of the initial load

	fetchPending bool // the fetch job is still to be delivered
	bulkLoaded   bool // reload was called
	bulkDone     bool // completion was already signalled
	outstanding  int  // bulk jobs handed out but not yet finished
	total        int
	failed       int

	onBulkDone func(bulkResult) // called once, without holding mx

	droppedCount atomic.Uint64 // all elements that left the pipeline unprocessed
}

func newPendingQueue(onBulkDone func(bulkResult)) *pendingQueue {
	return &pendingQueue{
		single:       make(memberSet),
		fetchPending: true,
		onBulkDone:   onBulkDone,
	}
}

func (pq *pendingQueue) addZettel(zid id.Zid) {
	if !zid.IsValid() {
		return
	}
	pq.mx.Lock()
	defer pq.mx.Unlock()

	delete(pq.bulk, zid) // retrieved anew anyway; moves to the individual set
	pq.single[zid] = struct{}{}
}

// reload sets the bulk. Only the first call has an effect.
func (pq *pendingQueue) reload(allZids memberSet) {
	bulk := maps.Clone(allZids) // do not share the caller's map

	pq.mx.Lock()
	defer pq.mx.Unlock()
	if pq.bulkLoaded {
		return
	}
	for zid := range pq.single {
		delete(bulk, zid)
	}
	pq.bulk = bulk
	pq.total = len(allZids)
	pq.bulkLoaded = true
	pq.fetchPending = false
}

// retryFetch requests the fetch job again after a failed FetchZids.
func (pq *pendingQueue) retryFetch() {
	pq.mx.Lock()
	defer pq.mx.Unlock()
	if !pq.bulkLoaded {
		pq.fetchPending = true
	}
}

type pendingJob struct {
	zid      id.Zid
	action   pendingAction
	fromBulk bool
}

type pendingAction uint8

const (
	pendingNothing pendingAction = iota
	pendingZettel
	pendingFetch
)

func (pq *pendingQueue) getJob() pendingJob {
	job, res, done := pq.takeJob()
	if done {
		pq.onBulkDone(res)
	}
	return job
}

func (pq *pendingQueue) takeJob() (job pendingJob, res bulkResult, done bool) {
	pq.mx.Lock()
	defer pq.mx.Unlock()

	if pq.fetchPending {
		pq.fetchPending = false
		return pendingJob{action: pendingFetch}, res, false
	}
	if zid, ok := popAny(pq.single); ok {
		return pendingJob{zid: zid, action: pendingZettel}, res, false
	}
	if zid, ok := popAny(pq.bulk); ok {
		pq.outstanding++
		return pendingJob{zid: zid, action: pendingZettel, fromBulk: true}, res, false
	}
	// Nothing to hand out: the bulk may have been drained by addZettel.
	res, done = pq.completeLocked()
	return pendingJob{}, res, done
}

// finishBulk marks a bulk job as processed: by the store step, or earlier if
// the element left the pipeline (failed == true).
func (pq *pendingQueue) finishBulk(failed bool) {
	res, done := pq.markFinished(failed)
	if done {
		pq.onBulkDone(res)
	}
}

// dropped accounts for an element that left the pipeline without reaching the
// store. Only bulk elements are counted.
func (pq *pendingQueue) dropped(bulk bool) {
	pq.droppedCount.Add(1)
	if bulk {
		pq.finishBulk(true)
	}
}
func (pq *pendingQueue) droppedTotal() uint64 { return pq.droppedCount.Load() }

func (pq *pendingQueue) markFinished(failed bool) (bulkResult, bool) {
	pq.mx.Lock()
	defer pq.mx.Unlock()

	pq.outstanding--
	if failed {
		pq.failed++
	}
	return pq.completeLocked()
}

// completeLocked reports true exactly once, when the bulk is complete.
func (pq *pendingQueue) completeLocked() (bulkResult, bool) {
	if !pq.bulkLoaded || pq.bulkDone || len(pq.bulk) > 0 || pq.outstanding > 0 {
		return bulkResult{}, false
	}
	pq.bulkDone = true
	return bulkResult{total: pq.total, failed: pq.failed}, true
}

func popAny(ms memberSet) (id.Zid, bool) {
	for zid := range ms {
		delete(ms, zid)
		return zid, true
	}
	return id.Invalid, false
}
