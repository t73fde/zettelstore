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
	"testing"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/idset"
	"zettelstore.de/z/internal/index"
)

// fakeFetcher satisfies index.Fetcher by embedding the (nil) interface.
// Only its identity is used; calling a method would panic.
type fakeFetcher struct {
	index.Fetcher
	name string
}

func zid(n int) id.Zid { return id.Zid(uint64(20260101000000) + uint64(n)) }

func zidSet(ns ...int) idset.ZidSet {
	var s idset.ZidSet
	for _, n := range ns {
		s.Insert(zid(n))
	}
	return s
}

// drain calls getJob until the queue reports pendingNothing.
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

// checkInvariants verifies the structural invariants of the batch list.
func checkInvariants(t *testing.T, pq *pendingQueue) {
	t.Helper()
	pq.mx.Lock()
	defer pq.mx.Unlock()
	if (pq.first == nil) != (pq.last == nil) {
		t.Fatalf("first/last inconsistent: first=%v last=%v", pq.first, pq.last)
	}
	var last *pendingBatch
	for b := pq.first; b != nil; b = b.next {
		last = b
		if len(b.members) == 0 {
			t.Error("empty batch in queue")
		}
		if b.isReload && b != pq.first {
			t.Error("reload batch not at the front")
		}
		if !b.isReload && pq.maxLoad > 0 && len(b.members) > pq.maxLoad {
			t.Errorf("batch has %d members, maxLoad is %d", len(b.members), pq.maxLoad)
		}
	}
	if last != pq.last {
		t.Error("last does not point to the final batch")
	}
}

func TestEmptyQueue(t *testing.T) {
	pq := &pendingQueue{}
	if job := pq.getJob(); job.action != pendingNothing {
		t.Errorf("expected pendingNothing, got %v", job.action)
	}
}

func TestInvalidInputIgnored(t *testing.T) {
	pq := &pendingQueue{}
	f := &fakeFetcher{name: "f"}
	pq.addZettel(f, id.Invalid)
	pq.addZettel(nil, zid(1))
	pq.reset(nil)
	pq.reload(nil, zidSet(1, 2))
	if jobs := drain(pq); len(jobs) != 0 {
		t.Errorf("expected no jobs, got %d", len(jobs))
	}
	checkInvariants(t, pq)
}

func TestDuplicateCollapsesNewerFetcherWins(t *testing.T) {
	pq := &pendingQueue{}
	f1, f2 := &fakeFetcher{name: "f1"}, &fakeFetcher{name: "f2"}
	pq.addZettel(f1, zid(1))
	pq.addZettel(f2, zid(1))
	jobs := drain(pq)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].fetcher != index.Fetcher(f2) || jobs[0].zid != zid(1) {
		t.Errorf("unexpected job %+v", jobs[0])
	}
}

func TestDuplicateAcrossBatches(t *testing.T) {
	pq := &pendingQueue{maxLoad: 1} // every zettel gets its own batch
	f1, f2 := &fakeFetcher{name: "f1"}, &fakeFetcher{name: "f2"}
	pq.addZettel(f1, zid(1))
	pq.addZettel(f1, zid(2))
	pq.addZettel(f2, zid(1)) // already waiting in the first batch
	checkInvariants(t, pq)
	got := map[id.Zid]index.Fetcher{}
	for _, job := range drain(pq) {
		got[job.zid] = job.fetcher
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 distinct zettel, got %d", len(got))
	}
	if got[zid(1)] != index.Fetcher(f2) || got[zid(2)] != index.Fetcher(f1) {
		t.Errorf("unexpected fetchers: %v", got)
	}
}

func TestMaxLoadOpensNewBatches(t *testing.T) {
	pq := &pendingQueue{maxLoad: 3}
	f := &fakeFetcher{name: "f"}
	for i := 1; i <= 10; i++ {
		pq.addZettel(f, zid(i))
	}
	checkInvariants(t, pq)
	batches := 0
	for b := pq.first; b != nil; b = b.next {
		batches++
	}
	if batches != 4 { // 3+3+3+1
		t.Errorf("expected 4 batches, got %d", batches)
	}
	seen := map[id.Zid]bool{}
	for _, job := range drain(pq) {
		seen[job.zid] = true
	}
	if len(seen) != 10 {
		t.Errorf("expected 10 distinct zettel, got %d", len(seen))
	}
	checkInvariants(t, pq)
}

func TestReloadBatchComesFirst(t *testing.T) {
	pq := &pendingQueue{}
	fz, fr := &fakeFetcher{name: "zettel"}, &fakeFetcher{name: "reload"}
	pq.addZettel(fz, zid(1))
	pq.reload(fr, zidSet(2, 3))
	pq.addZettel(fz, zid(4))
	checkInvariants(t, pq)

	jobs := drain(pq)
	if len(jobs) != 4 {
		t.Fatalf("expected 4 jobs, got %d", len(jobs))
	}
	for i, job := range jobs {
		wantReload := i < 2
		if job.fromReload != wantReload {
			t.Errorf("job %d: fromReload=%v, want %v", i, job.fromReload, wantReload)
		}
		if wantReload && job.fetcher != index.Fetcher(fr) {
			t.Errorf("job %d: wrong fetcher", i)
		}
		if !wantReload && job.fetcher != index.Fetcher(fz) {
			t.Errorf("job %d: wrong fetcher", i)
		}
	}
}

func TestAddZettelNeverEntersReloadBatch(t *testing.T) {
	pq := &pendingQueue{}
	fz, fr := &fakeFetcher{name: "zettel"}, &fakeFetcher{name: "reload"}
	pq.reload(fr, zidSet(1))
	pq.addZettel(fz, zid(2)) // last batch is a reload batch -> new batch
	checkInvariants(t, pq)
	for _, job := range drain(pq) {
		if job.zid == zid(2) && job.fromReload {
			t.Error("zettel 2 was put into the reload batch")
		}
	}
}

func TestReloadReplacesPreviousReloadKeepsZettel(t *testing.T) {
	pq := &pendingQueue{}
	fz, fr := &fakeFetcher{name: "zettel"}, &fakeFetcher{name: "reload"}
	pq.addZettel(fz, zid(9))
	pq.reload(fr, zidSet(1, 2))
	pq.reload(fr, zidSet(3))
	checkInvariants(t, pq)

	got := map[id.Zid]bool{}
	for _, job := range drain(pq) {
		got[job.zid] = job.fromReload
	}
	if len(got) != 2 || !got[zid(3)] || got[zid(9)] {
		t.Errorf("expected {3: reload, 9: zettel}, got %v", got)
	}
}

func TestReloadWithEmptySetClearsQueue(t *testing.T) {
	pq := &pendingQueue{}
	f := &fakeFetcher{name: "f"}
	pq.addZettel(f, zid(1))
	pq.reload(f, idset.ZidSet{})
	if jobs := drain(pq); len(jobs) != 0 {
		t.Errorf("expected empty queue, got %d jobs", len(jobs))
	}
	checkInvariants(t, pq)
}

func TestResetDeliveredOnceAndFirst(t *testing.T) {
	pq := &pendingQueue{}
	fz, fr := &fakeFetcher{name: "zettel"}, &fakeFetcher{name: "reset"}
	pq.addZettel(fz, zid(1))
	pq.reset(fr) // clears the queue

	job := pq.getJob()
	if job.action != pendingReload || job.fetcher != index.Fetcher(fr) {
		t.Fatalf("expected pendingReload with reset fetcher, got %+v", job)
	}
	if job = pq.getJob(); job.action != pendingNothing {
		t.Errorf("expected pendingNothing after reset was delivered, got %+v", job)
	}
}

func TestResetSurvivesFollowingReload(t *testing.T) {
	pq := &pendingQueue{}
	fr, fl := &fakeFetcher{name: "reset"}, &fakeFetcher{name: "reload"}
	pq.reset(fr)
	pq.reload(fl, zidSet(5))
	pq.reload(fl, idset.ZidSet{}) // even an empty reload keeps the reset signal

	jobs := drain(pq)
	if len(jobs) != 1 || jobs[0].action != pendingReload || jobs[0].fetcher != index.Fetcher(fr) {
		t.Errorf("expected exactly the reset signal, got %+v", jobs)
	}
}

func TestResetThenReloadOrder(t *testing.T) {
	pq := &pendingQueue{}
	fr, fl := &fakeFetcher{name: "reset"}, &fakeFetcher{name: "reload"}
	pq.reset(fr)
	pq.reload(fl, zidSet(5))

	jobs := drain(pq)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].action != pendingReload {
		t.Errorf("first job must be the reset signal, got %+v", jobs[0])
	}
	if jobs[1].action != pendingZettel || jobs[1].zid != zid(5) || !jobs[1].fromReload {
		t.Errorf("second job must be zettel 5 from reload, got %+v", jobs[1])
	}
}

// TestConcurrent is meant to run with -race. Producers add disjoint zettel
// while a consumer drains; afterwards every zettel must have been delivered.
func TestConcurrent(t *testing.T) {
	const producers, perProducer = 8, 500
	pq := &pendingQueue{maxLoad: 50}
	f := &fakeFetcher{name: "f"}

	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perProducer {
				pq.addZettel(f, zid(p*perProducer+i+1))
			}
		}()
	}

	done := make(chan struct{})
	seen := map[id.Zid]bool{}
	go func() {
		defer close(done)
		for {
			job := pq.getJob()
			if job.action == pendingZettel {
				seen[job.zid] = true
				continue
			}
			select {
			case <-stop(&wg):
				for _, job := range drain(pq) {
					seen[job.zid] = true
				}
				return
			default:
			}
		}
	}()
	<-done

	if len(seen) != producers*perProducer {
		t.Errorf("expected %d distinct zettel, got %d", producers*perProducer, len(seen))
	}
	checkInvariants(t, pq)
}

// stop returns a channel that is closed when wg is done.
func stop(wg *sync.WaitGroup) <-chan struct{} {
	ch := make(chan struct{})
	go func() { wg.Wait(); close(ch) }()
	return ch
}
