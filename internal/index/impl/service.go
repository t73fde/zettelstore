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
	"errors"
	"log/slog"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/index"
	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/zettel"
)

// Implements the background service(s).

// Start the search index.
func (idx *Index) Start() {
	idx.lifecycleMx.Lock()
	defer idx.lifecycleMx.Unlock()

	idx.mx.Lock()
	defer idx.mx.Unlock()
	if idx.wake != nil {
		return // already running
	}
	idx.wake = make(chan struct{}, 1)

	idx.stats = &stats{}
	idx.dict = makeDictionary(2000) // TODO: configurable wordsHint
	idx.store = makeStore(idx.stats, idx.dict)

	ctx, cancel := context.WithCancelCause(context.Background())
	idx.pl = &pipeline{
		ctx:    ctx,
		cancel: cancel,
		logger: idx.logger.With("service", "pipeline"),
	}

	idx.pending = &pendingQueue{}

	storeCh := make(chan storeData, 4)     // TODO: make configurable?
	analyzeCh := make(chan analyzeData, 4) // TODO: make configurable?
	parseCh := make(chan parseData, 4)     // TODO: make configurable?

	storeSrv := storeService{logger: idx.logger.With("service", "store"), store: idx.store}
	analyzeSrv := analyzeService{logger: idx.logger.With("service", "analyse"), analyser: makeAnalyzer(idx.dict)}
	parseSrv := parseService{logger: idx.logger.With("service", "parse"), config: idx.config}
	retrieveSrv := retrieveService{logger: idx.logger.With("service", "retrieve"), getRetriever: idx.getRetriever}

	idx.pl.spawn("IndexStore", func() { storeSrv.service(ctx, storeCh) })
	idx.pl.spawn("IndexAnalyzer", func() { analyzeSrv.service(ctx, analyzeCh, storeCh) })
	idx.pl.spawn("IndexParser", func() { parseSrv.service(ctx, parseCh, analyzeCh) })
	idx.pl.spawn("IndexRetriever", func() { retrieveSrv.service(ctx, idx.wake, idx.pending, parseCh) })

	idx.logger.Debug("Started")
}

// Stop the search index.
func (idx *Index) Stop() {
	idx.lifecycleMx.Lock()
	defer idx.lifecycleMx.Unlock()

	idx.mx.RLock()
	pl := idx.pl
	idx.mx.RUnlock()
	if pl == nil {
		return // not running
	}

	idx.logger.Debug("Stop")
	pl.cancel(nil) // interrupts blocking GetZettel / FetchZids
	idx.pl.wg.Wait()

	idx.mx.Lock()
	close(idx.wake)
	idx.wake, idx.pending, idx.pl = nil, nil, nil
	idx.store, idx.dict, idx.stats = nil, nil, nil
	idx.mx.Unlock()
}

// ----- retrieve service ----------

// retrieveService reads from the pending queue and tries to retrieve zettel.
type retrieveService struct {
	logger       *slog.Logger
	getRetriever func() index.Retriever
}

func (rsrv *retrieveService) service(
	ctx context.Context,
	wake <-chan struct{},
	pending *pendingQueue,
	out chan<- parseData,
) {
	getCtx := box.NoEnrichContext(ctx) // TODO: does not seem to work always. Many zettel are enriched

	for {
	loop:
		for {
			if ctx.Err() != nil {
				break loop // cancelled by Stop
			}
			switch job := pending.getJob(); job.action {
			case pendingNothing:
				rsrv.logger.Info("Nothing")
				break loop

			case pendingZettel:
				rsrv.logger.Info("Zettel", "zid", job.zid, "generation", job.generation, "reload", job.fromReload)
				retriever := rsrv.getRetriever()
				zettel, err := retriever.GetZettel(getCtx, job.zid)
				if err != nil {
					if getCtx.Err() != nil {
						break loop // cancelled by Stop: not an error, not a deletion
					}
					if _, notFound := errors.AsType[box.ErrZettelNotFound](err); notFound {
						if !send(getCtx, out, parseData{zid: job.zid, generation: job.generation, deleted: true}) {
							break loop
						}
					} else {
						rsrv.logger.Error("GetZettel", "zid", job.zid, "error", err)
					}
					continue
				}
				if !send(getCtx, out, parseData{zid: job.zid, zettel: zettel, generation: job.generation}) {
					break loop
				}

			case pendingReset:
				rsrv.logger.Info("Reset", "generation", job.generation)
				retriever := rsrv.getRetriever()
				zids, err := retriever.FetchZids(ctx)
				if err != nil {
					if ctx.Err() != nil {
						break loop // cancelled by Stop: not an error
					}
					rsrv.logger.Error("FetchZids", "error", err)
					continue
				}
				pending.reload(zids)

			default:
				rsrv.logger.Error("Unknown job action", "action", job.action)
			}

		}
		select {
		case <-wake:
		case <-ctx.Done():
			rsrv.logger.Info("Byebye")
			close(out)
			return
		}
	}
}

// ----- parse service ----------

type parseData struct {
	zid        id.Zid
	zettel     zettel.Zettel
	generation uint64
	deleted    bool
}

type parseService struct {
	logger *slog.Logger
	config config.Config
}

func (psrv *parseService) service(ctx context.Context, in <-chan parseData, out chan<- analyzeData) {
	for zd := range recvSeq(ctx, in) {
		if zd.deleted {
			psrv.logger.Info("Deleted", "zid", zd.zid, "generation", zd.generation)
			if !send(ctx, out, analyzeData{zid: zd.zid, zettel: nil, generation: zd.generation}) {
				break
			}
			continue
		}
		psrv.logger.Info("Update", "zid", zd.zid, "generation", zd.generation)
		ztl := parser.ParseZettel(ctx, zd.zettel, "", psrv.config)
		if !send(ctx, out, analyzeData{zid: zd.zid, zettel: ztl, generation: zd.generation}) {
			break
		}
	}
	psrv.logger.Info("NotTwoPi")
	close(out)
}

// ----- analyze service ----------

type analyzeData struct {
	zid        id.Zid // for use when zettel is deleted
	zettel     *zettel.ParsedZettel
	generation uint64
}

type analyzeService struct {
	logger   *slog.Logger
	analyser *analyzer
}

func (asrv *analyzeService) service(ctx context.Context, in <-chan analyzeData, out chan<- storeData) {
	for ad := range recvSeq(ctx, in) {
		var sd storeData
		if ad.zettel == nil {
			asrv.logger.Info("Deleted", "zid", ad.zid, "generation", ad.generation)
			asrv.analyser.processDeleted(&ad, &sd)
			if !send(ctx, out, sd) {
				break
			}
			continue
		}
		asrv.logger.Info("Update", "zid", ad.zid, "generation", ad.generation)
		if ad.zid != ad.zettel.Zid {
			asrv.logger.Error("Differing Zids", "queue", ad.zid, "parsed", ad.zettel.Zid)
			ad.zettel.Zid = ad.zid
		}
		asrv.analyser.analyze(&ad, &sd)
		if !send(ctx, out, sd) {
			break
		}
	}

	asrv.logger.Info("SchlussAL")
	close(out)
}

// ----- store service ----------

type storeData struct {
	zid        id.Zid   // for use when zettel is deleted
	dzid       denseZid // for use when zettel is deleted
	aztl       *analyzedZettel
	generation uint64
}

type storeService struct {
	logger *slog.Logger
	store  *store
}

func (ssrv *storeService) service(ctx context.Context, in <-chan storeData) {
	for sd := range recvSeq(ctx, in) {
		if sd.aztl == nil {
			ssrv.logger.Info("Deleted", "zid", sd.zid, "dzid", sd.dzid, "generation", sd.generation)
			ssrv.store.removeZettel(sd.dzid, sd.generation)
			continue
		}
		ssrv.logger.Info("Update", "zid", sd.zid, "dzid", sd.dzid, "generation", sd.generation)
		ssrv.store.updateZettel(sd.aztl, sd.generation)
	}

	ssrv.logger.Info("Finalforget")
}
