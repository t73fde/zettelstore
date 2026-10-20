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
	"runtime"
	"sync"
	"testing"

	"t73f.de/r/zsc/domain/id"
)

func zid(n int) id.Zid { return id.Zid(20260101000000 + n) }

func setOf(zids ...id.Zid) memberSet {
	ms := make(memberSet, len(zids))
	for _, z := range zids {
		ms[z] = struct{}{}
	}
	return ms
}

// drain takes jobs until the queue reports nothing to do.
func drain(pq *pendingQueue) []pendingJob {
	var jobs []pendingJob
	for {
		job := pq.getJob()
		if job.action == pendingNothing {
			return jobs
		}
		jobs = append(jobs, job)
	}
}

// zettelSet collects the zids of all zettel jobs and fails on duplicates.
func zettelSet(t *testing.T, jobs []pendingJob) memberSet {
	t.Helper()
	ms := make(memberSet, len(jobs))
	for _, job := range jobs {
		if job.action != pendingZettel {
			continue
		}
		if _, found := ms[job.zid]; found {
			t.Errorf("zid %v delivered more than once", job.zid)
		}
		ms[job.zid] = struct{}{}
	}
	return ms
}

func TestPendingEmpty(t *testing.T) {
	var pq pendingQueue
	if job := pq.getJob(); job.action != pendingNothing {
		t.Errorf("empty queue: got action %v", job.action)
	}
}

func TestPendingInvalidZid(t *testing.T) {
	var pq pendingQueue
	pq.addZettel(id.Invalid)
	if jobs := drain(&pq); len(jobs) != 0 {
		t.Errorf("invalid zid queued: %v", jobs)
	}
}

func TestPendingSingleDedup(t *testing.T) {
	var pq pendingQueue
	pq.addZettel(zid(1))
	pq.addZettel(zid(1))
	pq.addZettel(zid(1))
	jobs := drain(&pq)
	if len(jobs) != 1 {
		t.Fatalf("want 1 job, got %d", len(jobs))
	}
	if jobs[0].action != pendingZettel || jobs[0].zid != zid(1) || jobs[0].fromReload {
		t.Errorf("unexpected job: %+v", jobs[0])
	}
}

func TestPendingAllDelivered(t *testing.T) {
	var pq pendingQueue
	want := make(memberSet)
	for i := range 100 {
		pq.addZettel(zid(i))
		want[zid(i)] = struct{}{}
	}
	if got := zettelSet(t, drain(&pq)); !maps.Equal(got, want) {
		t.Errorf("delivered %d zids, want %d", len(got), len(want))
	}
}

func TestPendingReloadFirstGeneration(t *testing.T) {
	var pq pendingQueue
	all := setOf(zid(1), zid(2), zid(3))
	pq.reload(all)

	jobs := drain(&pq)
	if got := zettelSet(t, jobs); !maps.Equal(got, setOf(zid(1), zid(2), zid(3))) {
		t.Errorf("unexpected zids: %v", got)
	}
	for _, job := range jobs {
		if !job.fromReload || job.generation != 1 {
			t.Errorf("want fromReload and generation 1, got %+v", job)
		}
	}
	if len(all) != 3 {
		t.Errorf("caller's map was modified: %v", all)
	}
}

func TestPendingReloadDoesNotShareMap(t *testing.T) {
	var pq pendingQueue
	all := setOf(zid(1), zid(2))
	pq.reload(all)
	all[zid(99)] = struct{}{}
	delete(all, zid(1))

	if got := zettelSet(t, drain(&pq)); !maps.Equal(got, setOf(zid(1), zid(2))) {
		t.Errorf("queue was affected by caller's later changes: %v", got)
	}
}

func TestPendingSingleBeforeBulk(t *testing.T) {
	var pq pendingQueue
	pq.reload(setOf(zid(1), zid(2), zid(3)))
	pq.addZettel(zid(4))

	job := pq.getJob()
	if job.zid != zid(4) || job.fromReload {
		t.Errorf("single zettel must come first, got %+v", job)
	}
}

func TestPendingAddMovesFromBulk(t *testing.T) {
	var pq pendingQueue
	pq.reload(setOf(zid(1), zid(2)))
	pq.addZettel(zid(1))

	jobs := drain(&pq)
	if got := zettelSet(t, jobs); !maps.Equal(got, setOf(zid(1), zid(2))) {
		t.Fatalf("unexpected zids: %v", got)
	}
	for _, job := range jobs {
		switch job.zid {
		case zid(1):
			if job.fromReload {
				t.Error("zid 1 must be delivered as single zettel")
			}
		case zid(2):
			if !job.fromReload {
				t.Error("zid 2 must be delivered as part of the reload")
			}
		}
	}
}

func TestPendingReloadExcludesSingle(t *testing.T) {
	var pq pendingQueue
	pq.addZettel(zid(1))
	pq.reload(setOf(zid(1), zid(2)))

	jobs := drain(&pq)
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %d: %+v", len(jobs), jobs)
	}
	zettelSet(t, jobs) // fails on duplicates
	if jobs[0].zid != zid(1) || jobs[0].fromReload {
		t.Errorf("zid 1 must be first and single: %+v", jobs[0])
	}
}

func TestPendingResetClearsAndDeliversOnce(t *testing.T) {
	var pq pendingQueue
	pq.addZettel(zid(1))
	pq.reload(setOf(zid(2), zid(3)))
	pq.reset()

	jobs := drain(&pq)
	if len(jobs) != 1 || jobs[0].action != pendingReset {
		t.Fatalf("want exactly one reset job, got %+v", jobs)
	}
	if jobs[0].generation != 2 { // reload -> 1, reset -> 2
		t.Errorf("want generation 2, got %d", jobs[0].generation)
	}
}

func TestPendingResetBeforeZettel(t *testing.T) {
	var pq pendingQueue
	pq.reset()
	pq.addZettel(zid(5))

	jobs := drain(&pq)
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %+v", jobs)
	}
	if jobs[0].action != pendingReset {
		t.Errorf("reset must be first: %+v", jobs[0])
	}
	if jobs[1].action != pendingZettel || jobs[1].zid != zid(5) || jobs[1].generation != 1 {
		t.Errorf("unexpected second job: %+v", jobs[1])
	}
}

func TestPendingGeneration(t *testing.T) {
	var pq pendingQueue
	pq.addZettel(zid(1))
	if job := pq.getJob(); job.generation != 0 {
		t.Errorf("before any reset/reload: want 0, got %d", job.generation)
	}

	pq.reset()
	pq.reload(setOf(zid(2)))
	for _, job := range drain(&pq) {
		if job.generation != 1 {
			t.Errorf("reload after reset keeps generation 1, got %+v", job)
		}
	}

	pq.reset()
	pq.reset()
	if job := pq.getJob(); job.action != pendingReset || job.generation != 3 {
		t.Errorf("want reset job with generation 3, got %+v", job)
	}
}

func TestPendingConcurrent(t *testing.T) {
	const producers, perProducer = 8, 200

	var pq pendingQueue
	var mx sync.Mutex
	seen := make(map[id.Zid]int)
	collect := func(job pendingJob) {
		if job.action == pendingZettel {
			mx.Lock()
			seen[job.zid]++
			mx.Unlock()
		}
	}

	stop := make(chan struct{})
	var consumers sync.WaitGroup
	for range 4 {
		consumers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				job := pq.getJob()
				if job.action == pendingNothing {
					runtime.Gosched()
					continue
				}
				collect(job)
			}
		})
	}

	var wg sync.WaitGroup
	for p := range producers {
		wg.Go(func() {
			for i := range perProducer {
				pq.addZettel(zid(p*perProducer + i))
			}
		})
	}
	wg.Wait()
	close(stop)
	consumers.Wait()
	for _, job := range drain(&pq) {
		collect(job)
	}

	if len(seen) != producers*perProducer {
		t.Errorf("want %d distinct zids, got %d", producers*perProducer, len(seen))
	}
	for z, n := range seen {
		if n != 1 {
			t.Errorf("zid %v delivered %d times", z, n)
		}
	}
}
