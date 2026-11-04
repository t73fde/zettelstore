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
	"sync/atomic"

	"t73f.de/r/zsc/domain/id"
)

type memberSet map[id.Zid]struct{}

// waiterSet maps the zid of an individually notified zettel to the channels of
// the callers that wait until that zettel is stored. The slice may be nil.
type waiterSet map[id.Zid][]chan<- error

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
	single waiterSet // individually notified zettel
	closed bool      // abort was called; new waiters are refused
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
		single:       make(waiterSet),
		fetchPending: true,
		onBulkDone:   onBulkDone,
	}
}

func (pq *pendingQueue) addZettel(zid id.Zid, done chan<- error) {
	if !zid.IsValid() {
		return
	}
	pq.mx.Lock()
	defer pq.mx.Unlock()

	if pq.closed {
		notify(done, errStopped) // do not let the caller wait for the timeout
		done = nil
	}
	delete(pq.bulk, zid)      // retrieved anew anyway; moves to the individual set
	waiters := pq.single[zid] // nil, if zid is not queued yet
	if done != nil {
		waiters = append(waiters, done)
	}
	pq.single[zid] = waiters // creates the entry, even if waiters is nil
}

// reload sets the bulk. Only the first call has an effect.
//
// Ownership of allZids is transferred to pendingQueue.
func (pq *pendingQueue) reload(allZids memberSet) {
	total := len(allZids)
	pq.mx.Lock()
	defer pq.mx.Unlock()
	if pq.bulkLoaded {
		return
	}
	for zid := range pq.single {
		delete(allZids, zid)
	}
	pq.bulk = allZids
	pq.total = total
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
	waiters  []chan<- error // to be notified when the job is stored or dropped
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
	if zid, waiters, ok := popAny(pq.single); ok {
		return pendingJob{zid: zid, action: pendingZettel, waiters: waiters}, res, false
	}
	if zid, _, ok := popAny(pq.bulk); ok {
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
// store. err is the reason, delivered to all waiters of the element.
func (pq *pendingQueue) dropped(bulk bool, waiters []chan<- error, err error) {
	pq.droppedCount.Add(1)
	notifyAll(waiters, err)
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

// abort notifies the waiters of all zettel not yet taken and refuses further
// waiters. Called when the instance is stopped or replaced. Elements already
// in the stages are completed by their dropped calls (with ErrIndexStopped).
func (pq *pendingQueue) abort(err error) {
	pq.mx.Lock()
	defer pq.mx.Unlock()
	pq.closed = true
	for zid, waiters := range pq.single {
		notifyAll(waiters, err)
		pq.single[zid] = nil
	}
}

// completeLocked reports true exactly once, when the bulk is complete.
func (pq *pendingQueue) completeLocked() (bulkResult, bool) {
	if !pq.bulkLoaded || pq.bulkDone || len(pq.bulk) > 0 || pq.outstanding > 0 {
		return bulkResult{}, false
	}
	pq.bulkDone = true
	return bulkResult{total: pq.total, failed: pq.failed}, true
}

func popAny[V any](m map[id.Zid]V) (id.Zid, V, bool) {
	for zid, v := range m {
		delete(m, zid)
		return zid, v, true
	}
	var zero V
	return id.Invalid, zero, false
}
