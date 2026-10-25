// -----------------------------------------------------------------------------
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
// -----------------------------------------------------------------------------

// Package impl implements the index.Indexer interface.
package impl

import (
	"log/slog"
	"sync"
	"time"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/index"
	"zettelstore.de/z/internal/kernel"
)

var _ index.Indexer = (*Index)(nil)
var _ index.Enqueuer = (*Index)(nil)
var _ index.Searcher = (*Index)(nil)

// Index serves searches from an active instance. A reset builds a new
// instance in the background; it replaces the active one when its bulk load
// is complete. Until the very first build is complete, the active instance
// serves partial results.
type Index struct {
	logger      *slog.Logger
	config      config.Config
	retriever   index.Retriever
	stopService func(cause error)

	lifecycleMx sync.Mutex   // serializes Start and Stop; may be held while waiting for pipelines
	mx          sync.RWMutex // protects the fields below; never held while waiting for a pipeline
	running     bool
	active      *instance
	building    *instance // nil, if no rebuild is in progress
	resets      uint64
	lastBuild   buildInfo

	failure        error // set when the active instance failed; cleared by Start
	rebuildFailure error // set when a rebuild failed; cleared by the next complete build
}

type buildInfo struct {
	duration time.Duration
	zettel   int
	failed   int
}

// New creates a new index object.
func New(config config.Config, stopService func(error)) *Index {
	return &Index{
		logger:      kernel.Main.GetLogger(kernel.IndexService),
		config:      config,
		retriever:   nil,
		stopService: stopService,
	}
}

// RegisterRetriever allows to set a retriever to be used by the Index.
func (idx *Index) RegisterRetriever(retriever index.Retriever) {
	if idx.retriever != nil {
		panic("Index.Retriever already set")
	}
	idx.retriever = retriever
}

// Start the search index. Indexing begins with the first OnReset.
func (idx *Index) Start() {
	idx.lifecycleMx.Lock()
	defer idx.lifecycleMx.Unlock()

	idx.mx.Lock()
	defer idx.mx.Unlock()
	if idx.running {
		return
	}
	idx.running = true
	idx.failure, idx.rebuildFailure = nil, nil

	idx.logger.Debug("Started")
}

// Stop the search index.
func (idx *Index) Stop() {
	idx.lifecycleMx.Lock()
	defer idx.lifecycleMx.Unlock()

	toStop, wasRunning := idx.detachAll()
	if !wasRunning {
		return
	}
	idx.logger.Debug("Stop")
	for _, in := range toStop {
		in.stop() // bounded by stopTimeout
	}
}

// detachAll marks the index as stopped and removes all instances from it.
// The caller stops the returned instances.
func (idx *Index) detachAll() (instances []*instance, wasRunning bool) {
	idx.mx.Lock()
	defer idx.mx.Unlock()

	if !idx.running {
		return nil, false
	}
	idx.running = false
	if idx.active != nil {
		instances = append(instances, idx.active)
	}
	if idx.building != nil {
		instances = append(instances, idx.building)
	}
	idx.active, idx.building = nil, nil
	return instances, true
}

type failureAction uint8

const (
	failIgnore       failureAction = iota // instance was already replaced or stopped
	failDropBuilding                      // rebuild failed, active index is kept
	failStopService                       // active index failed
)

// classifyFailure decides what to do about a failed instance. It detaches a
// failed building instance.
func (idx *Index) classifyFailure(in *instance, cause error) failureAction {
	idx.mx.Lock()
	defer idx.mx.Unlock()

	switch in {
	case idx.building:
		idx.building = nil
		idx.rebuildFailure = cause
		return failDropBuilding
	case idx.active:
		idx.failure = cause
		// Until the kernel has stopped the service, results of a failed
		// index are flagged as incomplete.
		in.ready.Store(false)
		return failStopService
	default:
		return failIgnore
	}
}

// instanceFailed is called in its own goroutine when a service of in gave up
// or the store step panicked.
func (idx *Index) instanceFailed(in *instance, cause error) {
	switch idx.classifyFailure(in, cause) {
	case failDropBuilding:
		idx.logger.Error("Rebuild failed, active index is kept", "instance", in.id, "cause", cause)
		in.stop() // pipeline is already cancelled; waits for its goroutines
	case failStopService:
		idx.logger.Error("Index failed, stopping index service", "instance", in.id, "cause", cause)
		if idx.stopService != nil {
			idx.stopService(cause)
		}
	}
}

// Enqueue a new zettel to update the search index.
func (idx *Index) Enqueue(reason index.Reason, zid id.Zid) {
	switch reason {
	case index.OnReady:
		idx.logger.Debug("OnReady")
	case index.OnReset:
		idx.logger.Debug("OnReset")
		idx.reset()
	case index.OnZettel:
		idx.logger.Debug("OnZettel", "zid", zid)
		idx.enqueueZettel(zid)
	default:
		idx.logger.Error("Unknown notification reason", "reason", reason, "zid", zid)
	}
}

// enqueueZettel hands the zettel to every instance that must reflect the
// change: the active one serves searches, the building one becomes the active
// one. Holding the read lock excludes a concurrent swap, so no update is lost.
func (idx *Index) enqueueZettel(zid id.Zid) {
	idx.mx.RLock()
	defer idx.mx.RUnlock()
	if idx.active != nil {
		idx.active.enqueue(zid)
	}
	if idx.building != nil {
		idx.building.enqueue(zid)
	}
}

// reset starts indexing all zettel in a new instance.
func (idx *Index) reset() {
	if idx.retriever == nil {
		panic("Index.Retriever not registered")
	}
	in, discard := idx.installNew()
	if in == nil {
		return // not running
	}
	if discard != nil {
		go discard.stop()
	}
}
func (idx *Index) installNew() (in, discard *instance) {
	idx.mx.Lock()
	defer idx.mx.Unlock()
	if !idx.running {
		return nil, nil
	}
	idx.resets++
	in = newInstance(idx, idx.resets, idx.hintsLocked())
	if idx.active == nil || !idx.active.ready.Load() {
		discard, idx.active = idx.active, in
	} else {
		discard, idx.building = idx.building, in
	}
	in.start(idx.config, idx.retriever)
	return in, discard
}

// hintsLocked derives the dimension of a new instance from the active one.
func (idx *Index) hintsLocked() sizeHints {
	h := sizeHints{words: defaultWordsHint}
	if a := idx.active; a != nil {
		h.words = max(h.words, withMargin(a.dict.numWords()))
		h.zettel = withMargin(a.dict.numZids())
		h.urls = withMargin(a.store.urlCount())
	}
	return h
}
func withMargin(n int) int { return n + n/8 }

// promote is called, exactly once per instance, when its bulk load is
// complete. It makes the instance the active one.
func (idx *Index) promote(in *instance, res bulkResult) {
	old, duration, ok := idx.swapIn(in, res)
	if !ok {
		return // superseded by a newer reset, or stopped
	}
	if res.failed > 0 {
		idx.logger.Error("Index complete",
			"instance", in.id, "duration", duration, "zettel", res.total, "failed", res.failed)

	} else {
		idx.logger.Info("Index complete",
			"instance", in.id, "duration", duration, "zettel", res.total)
	}
	if old != nil {
		go old.stop() // never wait here: called from a goroutine of in
	}
}

// swapIn makes in the active instance and returns the replaced one.
func (idx *Index) swapIn(in *instance, res bulkResult) (old *instance, duration time.Duration, ok bool) {
	idx.mx.Lock()
	defer idx.mx.Unlock()

	switch in {
	case idx.active:
		// Initial build is complete; the instance already serves.
	case idx.building:
		old = idx.active
		idx.active, idx.building = in, nil
	default:
		return nil, 0, false
	}
	duration = time.Since(in.started)
	idx.lastBuild = buildInfo{duration: duration, zettel: res.total, failed: res.failed}
	idx.rebuildFailure = nil
	in.ready.Store(true)
	return old, duration, true
}

// ReadStats populates st with index statistics.
func (idx *Index) ReadStats(st *index.Stats) {
	idx.mx.RLock()
	st.NumRebuild = idx.resets
	st.Building = idx.building != nil
	st.BuildDuration = idx.lastBuild.duration
	st.BuildFailed = idx.lastBuild.failed
	if idx.failure != nil {
		st.Failure = idx.failure.Error()
	}
	if idx.rebuildFailure != nil {
		st.RebuildFailure = idx.rebuildFailure.Error()
	}
	a := idx.active
	idx.mx.RUnlock()

	if a == nil {
		return // no instance yet: remaining fields stay zero
	}
	var ds dictStats
	a.dict.readStats(&ds)
	st.NumWords = ds.NumWords
	st.Vocabulary = ds.Vocabulary
	st.WordCount = ds.WordCount
	st.NumZettel = a.store.count()
	st.UpdatedZettel, st.RemovedZettel = a.stats.read()
	st.DroppedZettel = a.pending.droppedTotal()
	st.Ready = a.ready.Load()
}

// isReady reports whether the initial build is complete.
func (idx *Index) isReady() bool {
	idx.mx.RLock()
	defer idx.mx.RUnlock()
	return idx.active != nil && idx.active.ready.Load()
}

// activeInstance returns the active instance, if it is ready. Otherwise return nil.
func (idx *Index) activeInstance() *instance {
	idx.mx.RLock()
	defer idx.mx.RUnlock()
	if a := idx.active; a != nil && a.ready.Load() {
		return a
	}
	return nil
}
