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

	"t73f.de/r/zero/bitset"
	"t73f.de/r/zero/roster"
	"t73f.de/r/zero/vocab"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/index"
)

const defaultWordsHint = 2000 // TODO: configurable

// sizeHints dimension a new instance.
type sizeHints struct {
	words  int // expected number of distinct words
	zettel int // expected number of zettel
	urls   int // expected number of URLs
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
	in.store = makeStore(in.stats, hints, in.dict.inverse)
	in.pending = newPendingQueue(func(res bulkResult) { idx.promote(in, res) })
	in.onFail = idx.instanceFailed
	return in
}

// start spawns the services. The first thing they do is fetching all zettel.
func (in *instance) start(cfg config.Config, retriever index.Retriever) {
	in.logger.Debug("Start")
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
		logger:    in.logger.With("service", "retrieve"),
		retriever: retriever,
		pending:   in.pending,
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
	in.logger.Debug("Stopping")
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
	in.logger.Debug("Stopped")
}

// enqueue schedules the zettel for (re-) indexing.
func (in *instance) enqueue(zid id.Zid) {
	in.pending.addZettel(zid)
	select {
	case in.wake <- struct{}{}:
	default:
	}
}

// ----- query related methods -----------------------------------------------

// findZettel returns the dense identifier and the published storeZettel. The
// storeZettel is nil, if the zettel does not exist; the dzid is valid, if found.
func (in *instance) findZettel(zid id.Zid) (denseZid, *storeZettel, bool) {
	if dzid, found := in.dict.Find(zid); found {
		return dzid, in.store.getZettel(dzid), true
	}
	return 0, nil, false
}

// searchEqual returns the identifier of all zettel that match word exactly.
func (in *instance) searchEqual(word string) roster.Roster[id.Zid] {
	wid := in.dict.LookupString(word)           // vocab.InvalidWordID, if s is not a known word
	dzid, hasZid := in.dict.lookupZidText(word) // dense identifier, if s is the text of a known zid
	return in.dict.zidRoster(in.store.matchEqual(word, wid, hasZid, dzid))
}

// searchPrefix returns the identifier of all zettel that have the given prefix.
func (in *instance) searchPrefix(prefix string) roster.Roster[id.Zid] {
	// The snapshot of the words is taken before the store is read, as in searchEqual.
	words := in.dict.snapshot().WordsWithPrefixString(prefix)

	var zidMatches bitset.BitSet[denseZid]
	if pattern, ok := parseZidFragment(prefix); ok {
		in.dict.matchZids(&zidMatches, pattern, len(prefix), zidHasPrefix)
	}
	return in.dict.zidBitset(in.store.matchPrefix(prefix, words, &zidMatches, in.dict.numZids()))
}

// searchSuffix returns the identifier of all zettel that have the given suffix.
func (in *instance) searchSuffix(suffix string) roster.Roster[id.Zid] {
	// The snapshot of the words is taken before the store is read, as in searchEqual.
	words := in.dict.snapshot().WordsWithSuffixString(suffix)

	var zidMatches bitset.BitSet[denseZid]
	if pattern, ok := parseZidFragment(suffix); ok {
		in.dict.matchZids(&zidMatches, pattern, len(suffix), zidHasSuffix)
	}
	return in.dict.zidBitset(in.store.matchSuffix(suffix, words, &zidMatches, in.dict.numZids()))
}

// searchContains returns the identifier of all zettel that contain s.
func (in *instance) searchContains(s string) roster.Roster[id.Zid] {
	// The snapshot of the words is taken before the store is read, as in searchEqual.
	words := in.dict.snapshot().WordsContainingString(s)

	var zidMatches bitset.BitSet[denseZid]
	if pattern, ok := parseZidFragment(s); ok {
		in.dict.matchZids(&zidMatches, pattern, len(s), zidContains)
	}
	return in.dict.zidBitset(in.store.matchContains(s, words, &zidMatches, in.dict.numZids()))
}

// Enrich adds the computed metadata (relations) of m.Zid to m. It does nothing,
// if the zettel is not stored.
func (in *instance) enrich(m *meta.Meta) {
	dzid, found := in.dict.Find(m.Zid)
	if !found {
		in.store.stats.enrichMissed.Add(1)
		m.Set(meta.KeyIndexState, meta.ValueIndexStateUnindexed)
		return
	}

	updated := false
	known := in.store.enrich(dzid, func(slot enrichSlot, keyID vocab.WordID, dzids roster.Roster[denseZid]) {
		zids := in.dict.zidRoster(dzids)
		if zids.IsEmpty() { // no dense identifier could be decoded
			return
		}
		var key string
		switch slot {
		case slotDead:
			key = meta.KeyDead
		case slotBackward:
			key = meta.KeyBackward
		case slotForward:
			key = meta.KeyForward
		case slotBack:
			key = meta.KeyBack
		case slotMetaBackward:
			key = in.dict.GetString(keyID)
		}
		m.Set(key, zidsToValue(zids))
		updated = true
	})

	switch {
	case !known:
		in.store.stats.enrichMissed.Add(1)
		m.Set(meta.KeyIndexState, meta.ValueIndexStateUnindexed)
	case updated:
		in.store.stats.enrichUpdates.Add(1)
	}
}

func zidsToValue(s roster.Roster[id.Zid]) meta.Value {
	buf := make([]byte, 0, s.Count()*(id.LengthZid+1))
	for zid := range s.Values() {
		if len(buf) > 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, zid.Bytes()...)
	}
	return meta.Value(buf)
}
