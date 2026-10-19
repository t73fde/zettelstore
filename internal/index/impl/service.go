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
	"time"

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
	idx.mx.Lock()
	defer idx.mx.Unlock()
	if idx.notify != nil {
		return // already running
	}

	idx.stats = &stats{}
	idx.store = makeStore(idx.stats, 2000) // TODO: configurable wordsHint for voc

	ctx, cancel := context.WithCancelCause(context.Background())
	idx.pl = &pipeline{
		ctx:    ctx,
		cancel: cancel,
		logger: idx.logger.With("service", "pipeline"),
	}

	idx.pending = &pendingQueue{maxLoad: 1000} // TODO: maxLoad could be configurable?
	idx.notify = make(chan notifyData, 10)     // TODO: make 10 configurable?

	storeCh := make(chan storeData, 4) // TODO: make configurable?
	parseCh := make(chan parseData, 4) // TODO: make configurable?
	wakeCh := make(chan struct{}, 1)   // collector signals retrieve service that new data was placed in pending.

	storeSrv := storeService{logger: idx.logger.With("service", "store"), store: idx.store}
	parseSrv := parseService{logger: idx.logger.With("service", "parse"), config: idx.config}
	retrieveSrv := retrieveService{logger: idx.logger.With("service", "retrieve"), getRetriever: idx.getRetriever}
	collectSrv := collectService{logger: idx.logger.With("service", "collector")}

	idx.pl.spawn("IndexStore", func() { storeSrv.service(ctx, storeCh) })
	idx.pl.spawn("IndexParser", func() { parseSrv.service(ctx, parseCh, storeCh) })
	idx.pl.spawn("IndexRetriever", func() { retrieveSrv.service(ctx, wakeCh, idx.pending, parseCh) })
	idx.pl.spawn("IndexCollector", func() { collectSrv.service(ctx, idx.notify, idx.pending, wakeCh) })

	idx.logger.Debug("Started")
}

// Stop the search index.
func (idx *Index) Stop() {
	idx.mx.Lock()
	defer idx.mx.Unlock()
	if idx.notify == nil {
		return // not running
	}
	idx.logger.Debug("Stop")
	close(idx.notify)
	idx.pl.cancel(nil) // interrupts blocking GetZettel / FetchZids
	idx.pl.wg.Wait()
	idx.notify, idx.pending, idx.pl = nil, nil, nil
	idx.stats, idx.store = nil, nil
}

// collectService receives update notifications and adds them to an unlimited
// queue of pending jobs.
type collectService struct {
	logger *slog.Logger
}

func (csrv *collectService) service(ctx context.Context, notify <-chan notifyData, pending *pendingQueue, wake chan<- struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
loop:
	for {
		select {
		case d, ok := <-notify:
			if !ok {
				break loop
			}
			switch d.reason {
			case index.OnReady:
				csrv.logger.Info("OnReady")
				continue
			case index.OnReset:
				csrv.logger.Info("OnReset")
				pending.reset()
			case index.OnZettel:
				csrv.logger.Info("OnZettel", "zid", d.zid)
				pending.addZettel(d.zid)
			default:
				csrv.logger.Error("Unknown notification reason", "reason", d.reason, "zid", d.zid)
				continue
			}
			select {
			case wake <- struct{}{}:
			default:
			}

		case <-ticker.C:
			csrv.logger.Info("OnTick")
			select {
			case wake <- struct{}{}:
			default:
			}

		case <-ctx.Done():
			break loop
		}
	}
	csrv.logger.Debug("Good Buy")
	close(wake)
}

// ----- retrieve service ----------

// retrieveService reads from the pending queue and tries to retrieve zettel.
type retrieveService struct {
	logger       *slog.Logger
	getRetriever func() index.Retriever
}

func (rsrv *retrieveService) service(ctx context.Context, wake <-chan struct{}, pending *pendingQueue, out chan<- parseData) {
	for range wake {
		rsrv.logger.Info("WakeUp")
		retriever := rsrv.getRetriever()
	loop:
		for ctx.Err() == nil {
			switch job := pending.getJob(); job.action {
			case pendingNothing:
				rsrv.logger.Info("Nothing")
				break loop

			case pendingZettel:
				rsrv.logger.Info("Zettel", "zid", job.zid, "generation", job.generation)
				ctx = box.NoEnrichContext(ctx) // TODO: does not seem to work always. Many zettel are enriched
				zettel, err := retriever.GetZettel(ctx, job.zid)
				if err != nil {
					if ctx.Err() != nil {
						break loop // cancelled by Stop: not an error, not a deletion
					}
					if _, notFound := errors.AsType[box.ErrZettelNotFound](err); notFound {
						if !send(ctx, out, parseData{zid: job.zid, deleted: true}) {
							break loop
						}
					} else {
						rsrv.logger.Error("GetZettel", "zid", job.zid, "error", err)
					}
					continue
				}
				if !send(ctx, out, parseData{zid: job.zid, zettel: zettel}) {
					break loop
				}

			case pendingReset:
				rsrv.logger.Info("Reload", "generation", job.generation)
				zids, err := retriever.FetchZids(ctx)
				if err != nil {
					if ctx.Err() != nil {
						break loop // cancelled by Stop: not an error
					}
					rsrv.logger.Error("FetchZids", "error", err)
					continue
				}
				pending.reload(zids)
			}
		}
	}
	rsrv.logger.Info("Byebye")
	close(out)
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

func (psrv *parseService) service(ctx context.Context, in <-chan parseData, out chan<- storeData) {
	for zd := range recvSeq(ctx, in) {
		if zd.deleted {
			psrv.logger.Info("Deleted", "zid", zd.zid, "generation", zd.generation)
			if !send(ctx, out, storeData{zid: zd.zid, zettel: nil, generation: zd.generation}) {
				break
			}
			continue
		}
		psrv.logger.Info("Update", "zid", zd.zid, "generation", zd.generation)
		ztl := parser.ParseZettel(ctx, zd.zettel, "", psrv.config)
		if !send(ctx, out, storeData{zid: zd.zid, zettel: ztl, generation: zd.generation}) {
			break
		}
	}
	psrv.logger.Info("NotTwoPi")
	close(out)
}

// ----- store service ----------

type storeData struct {
	zid        id.Zid // for use when zettel is deleted
	zettel     *zettel.ParsedZettel
	generation uint64
}

type storeService struct {
	logger *slog.Logger
	store  *store
}

func (ssrv *storeService) service(ctx context.Context, in <-chan storeData) {
	for sd := range recvSeq(ctx, in) {
		if sd.zettel == nil {
			ssrv.logger.Info("Deleted", "zid", sd.zid, "generation", sd.generation)
			ssrv.store.removeZettel(sd.zid, sd.generation)
			continue
		}
		ssrv.logger.Info("Update", "zid", sd.zid, "generation", sd.generation)
		if sd.zid != sd.zettel.Zid {
			ssrv.logger.Error("Differing Zids: queue (%s) vs parsed zettel (%s)", sd.zid.String(), sd.zettel.Zid.String())
			sd.zettel.Zid = sd.zid
		}
		ssrv.store.updateZettel(sd.zettel, sd.generation)
	}

	ssrv.logger.Info("Finalforget")
}
