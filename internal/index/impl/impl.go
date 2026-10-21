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
	logger    *slog.Logger
	config    config.Config
	retriever index.Retriever

	lifecycleMx sync.Mutex   // serializes Start and Stop; may be held while waiting for pipelines
	mx          sync.RWMutex // protects the fields below; never held while waiting for a pipeline
	running     bool
	active      *instance
	building    *instance // nil, if no rebuild is in progress
	resets      uint64
	lastBuild   buildInfo
}

type buildInfo struct {
	duration time.Duration
	zettel   int
	failed   int
}

// New creates a new index object.
func New(config config.Config) *Index {
	return &Index{
		logger:    kernel.Main.GetLogger(kernel.IndexService),
		config:    config,
		retriever: nil,
	}
}

// RegisterRetriever allows to set a retriever to be used by the Index.
func (idx *Index) RegisterRetriever(retriever index.Retriever) {
	if idx.retriever != nil {
		panic("Index.Retriever already set")
	}
	idx.retriever = retriever
}
func (idx *Index) getRetriever() index.Retriever { return idx.retriever }

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
	idx.logger.Debug("Started")
}

// Stop the search index.
func (idx *Index) Stop() {
	idx.lifecycleMx.Lock()
	defer idx.lifecycleMx.Unlock()

	idx.mx.Lock()
	if !idx.running {
		idx.mx.Unlock()
		return
	}
	idx.running = false
	toStop := make([]*instance, 0, 2)
	if idx.active != nil {
		toStop = append(toStop, idx.active)
	}
	if idx.building != nil {
		toStop = append(toStop, idx.building)
	}
	idx.active, idx.building = nil, nil
	idx.mx.Unlock()

	idx.logger.Debug("Stop")
	for _, in := range toStop {
		in.stop()
	}
}

// Enqueue a new zettel to update the search index.
func (idx *Index) Enqueue(reason index.Reason, zid id.Zid) {
	switch reason {
	case index.OnReady:
		idx.logger.Info("OnReady")
	case index.OnReset:
		idx.logger.Info("OnReset")
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
		idx.logger.Error("Reset without retriever")
		return
	}

	idx.mx.Lock()
	if !idx.running {
		idx.mx.Unlock()
		return
	}
	idx.resets++
	in := newInstance(idx, idx.resets, idx.hintsLocked())

	var discard *instance
	if idx.active == nil || !idx.active.ready.Load() {
		// No complete index to keep serving: restart the initial build in place.
		discard = idx.active
		idx.active = in
	} else {
		discard = idx.building // an unfinished rebuild is aborted
		idx.building = in
	}
	in.start(idx.config, idx.getRetriever)
	idx.mx.Unlock()

	if discard != nil {
		go discard.stop()
	}
}

// hintsLocked derives the dimension of a new instance from the active one.
func (idx *Index) hintsLocked() sizeHints {
	h := sizeHints{words: defaultWordsHint}
	if a := idx.active; a != nil {
		h.words = max(h.words, withMargin(a.dict.numWords()))
		h.zettel = withMargin(a.store.count())
	}
	return h
}
func withMargin(n int) int { return n + n/8 }

// promote is called, exactly once per instance, when its bulk load is
// complete. It makes the instance the active one.
func (idx *Index) promote(in *instance, res bulkResult) {
	idx.mx.Lock()
	var old *instance
	switch {
	case idx.active == in:
		// Initial build is complete; the instance already serves.
	case idx.building == in:
		old = idx.active
		idx.active, idx.building = in, nil
	default:
		idx.mx.Unlock()
		return // superseded by a newer reset, or stopped
	}
	duration := time.Since(in.started)
	idx.lastBuild = buildInfo{duration: duration, zettel: res.total, failed: res.failed}
	in.ready.Store(true)
	idx.mx.Unlock()

	idx.logger.Info("Index complete",
		"instance", in.id, "duration", duration, "zettel", res.total, "failed", res.failed)
	if old != nil {
		go old.stop() // never wait here: called from a goroutine of in
	}
}

// ReadStats populates st with index statistics.
func (idx *Index) ReadStats(st *index.Stats) {
	idx.mx.RLock()
	defer idx.mx.RUnlock()

	st.NumResets = idx.resets
	st.Building = idx.building != nil
	st.BuildDuration = idx.lastBuild.duration
	st.BuildFailed = idx.lastBuild.failed

	a := idx.active
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
	st.Ready = a.ready.Load()
}

// isReady reports whether the initial build is complete.
func (idx *Index) isReady() bool {
	idx.mx.RLock()
	defer idx.mx.RUnlock()
	return idx.active != nil && idx.active.ready.Load()
}
