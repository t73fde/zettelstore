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
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/index"
)

const defaultWordsHint = 2000 // TODO: configurable

// sizeHints dimension a new instance.
type sizeHints struct {
	words  int // expected number of distinct words
	zettel int // expected number of zettel
}

// instance is one complete indexing pipeline with its own dictionary, store
// and pending queue. An instance is never reused after it was stopped.
type instance struct {
	id      uint64
	logger  *slog.Logger
	pl      *pipeline
	pending *pendingQueue
	wake    chan struct{} // never closed, so that Enqueue cannot panic
	stats   *stats
	dict    *dictionary
	store   *store

	started time.Time
	ready   atomic.Bool // bulk is complete
	onFail  func(*instance, error)
}

func newInstance(idx *Index, instID uint64, hints sizeHints) *instance {
	in := &instance{
		id:      instID,
		logger:  idx.logger.With("instance", instID),
		wake:    make(chan struct{}, 1),
		stats:   &stats{},
		dict:    makeDictionary(hints),
		started: time.Now(),
	}
	in.store = makeStore(in.stats, hints.zettel)
	in.pending = newPendingQueue(func(res bulkResult) { idx.promote(in, res) })
	in.onFail = idx.instanceFailed
	return in
}

// start spawns the services. The first thing they do is fetching all zettel.
func (in *instance) start(cfg config.Config, getRetriever func() index.Retriever) {
	in.logger.Info("Start")
	ctx, cancel := context.WithCancelCause(context.Background())
	in.pl = &pipeline{
		ctx:      ctx,
		cancel:   cancel,
		logger:   in.logger.With("service", "pipeline"),
		onGiveUp: func(cause error) { in.onFail(in, cause) },
	}

	storeCh := make(chan storeData, 4)     // TODO: configurable?
	analyzeCh := make(chan analyzeData, 4) // TODO: configurable?
	parseCh := make(chan parseData, 4)     // TODO: configurable?

	storeSrv := storeService{
		logger:  in.logger.With("service", "store"),
		store:   in.store,
		pending: in.pending,
		fatal:   in.pl.giveUp,
	}
	analyzeSrv := analyzeService{
		logger:   in.logger.With("service", "analyse"),
		analyser: makeAnalyzer(in.dict),
		pending:  in.pending,
	}
	parseSrv := parseService{
		logger:  in.logger.With("service", "parse"),
		config:  cfg,
		pending: in.pending,
	}
	retrieveSrv := retrieveService{
		logger:       in.logger.With("service", "retrieve"),
		getRetriever: getRetriever,
		pending:      in.pending,
		store:        in.store,
	}

	in.pl.spawn("IndexStore", func() { storeSrv.service(ctx, storeCh) })
	in.pl.spawn("IndexAnalyzer", func() { analyzeSrv.service(ctx, analyzeCh, storeCh) })
	in.pl.spawn("IndexParser", func() { parseSrv.service(ctx, parseCh, analyzeCh) })
	in.pl.spawn("IndexRetriever", func() { retrieveSrv.service(ctx, in.wake, parseCh) })
}

const stopTimeout = 10 * time.Second

// stop cancels the services and waits for their termination. It must not be
// called while holding Index.mx.
func (in *instance) stop() {
	in.logger.Info("Stopping")
	in.pl.cancel(nil) // interrupts blocking GetZettel / FetchZids
	done := make(chan struct{})
	go func() {
		in.pl.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(stopTimeout):
		in.logger.Error("Services did not stop in time, abandoning them", "timeout", stopTimeout)
	}
	in.logger.Info("Stopped")
}

// enqueue schedules the zettel for (re-) indexing.
func (in *instance) enqueue(zid id.Zid) {
	in.pending.addZettel(zid)
	select {
	case in.wake <- struct{}{}:
	default:
	}
}
