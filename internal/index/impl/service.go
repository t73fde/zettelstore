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

// Implements the background service(s) of one instance.

const fetchRetryDelay = 5 * time.Second // pause after a failed FetchZids

// ----- retrieve service ----------

// retrieveService reads from the pending queue and tries to retrieve zettel.
type retrieveService struct {
	logger       *slog.Logger
	getRetriever func() index.Retriever
	pending      *pendingQueue
	store        *store
}

func (rsrv *retrieveService) service(ctx context.Context, wake <-chan struct{}, out chan<- parseData) {
	getCtx := box.NoEnrichContext(ctx) // TODO: does not seem to work always. Many zettel are enriched

	for {
		rsrv.process(ctx, getCtx, out) // also runs once after start: fetches the bulk
		select {
		case <-wake:
		case <-ctx.Done():
			rsrv.logger.Info("Stop")
			close(out)
			return
		}
	}
}

// process processes jobs until the queue is empty or ctx is done.
func (rsrv *retrieveService) process(ctx, getCtx context.Context, out chan<- parseData) {
	for ctx.Err() == nil {
		job := rsrv.pending.getJob()
		switch job.action {
		case pendingNothing:
			return
		case pendingFetch:
			rsrv.fetch(ctx)
		case pendingZettel:
			if !rsrv.retrieve(getCtx, job, out) {
				return
			}
		default:
			rsrv.logger.Error("Unknown job action", "action", job.action)
		}
	}
}

func (rsrv *retrieveService) fetch(ctx context.Context) {
	rsrv.logger.Info("Fetch")
	zids, err := rsrv.getRetriever().FetchZids(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return // cancelled by Stop: not an error
		}
		rsrv.logger.Error("FetchZids", "error", err)
		rsrv.pending.retryFetch()
		select {
		case <-time.After(fetchRetryDelay):
		case <-ctx.Done():
		}
		return
	}
	rsrv.store.grow(len(zids))
	rsrv.pending.reload(zids)
}

// retrieve returns false if the pipeline was cancelled.
func (rsrv *retrieveService) retrieve(ctx context.Context, job pendingJob, out chan<- parseData) bool {
	rsrv.logger.Info("Zettel", "zid", job.zid, "bulk", job.fromBulk)
	ztl, err := rsrv.getRetriever().GetZettel(ctx, job.zid)
	if err != nil {
		if ctx.Err() != nil {
			return false // cancelled by Stop: not an error, not a deletion
		}
		if _, notFound := errors.AsType[box.ErrZettelNotFound](err); notFound {
			return send(ctx, out, parseData{zid: job.zid, deleted: true, bulk: job.fromBulk})
		}
		rsrv.logger.Error("GetZettel", "zid", job.zid, "error", err)
		rsrv.pending.dropped(job.fromBulk)
		return true
	}
	return send(ctx, out, parseData{zid: job.zid, zettel: ztl, bulk: job.fromBulk})
}

// ----- parse service ----------

type parseData struct {
	zid     id.Zid
	zettel  zettel.Zettel
	deleted bool
	bulk    bool // element belongs to the bulk load
}

type parseService struct {
	logger  *slog.Logger
	config  config.Config
	pending *pendingQueue
}

func (psrv *parseService) service(ctx context.Context, in <-chan parseData, out chan<- analyzeData) {
	for pd := range recvSeq(ctx, in) {
		ad := analyzeData{zid: pd.zid, bulk: pd.bulk}
		if pd.deleted {
			psrv.logger.Info("Deleted", "zid", pd.zid)
		} else {
			psrv.logger.Info("Upsert", "zid", pd.zid)
			ok := runRecovered("IndexParser", func() {
				ad.zettel = parser.ParseZettel(ctx, pd.zettel, "", psrv.config)
			})
			if !ok { // element is dropped, the service continues
				psrv.pending.dropped(pd.bulk)
				continue
			}
		}
		if !send(ctx, out, ad) {
			break
		}
	}
	psrv.logger.Info("Stop")
	close(out)
}

// ----- analyze service ----------

type analyzeData struct {
	zid    id.Zid // for use when zettel is deleted
	zettel *zettel.ParsedZettel
	bulk   bool
}

type analyzeService struct {
	logger   *slog.Logger
	analyser *analyzer
	pending  *pendingQueue
}

func (asrv *analyzeService) service(ctx context.Context, in <-chan analyzeData, out chan<- storeData) {
	for ad := range recvSeq(ctx, in) {
		var sd storeData
		ok := runRecovered("IndexAnalyzer", func() {
			if ad.zettel == nil {
				asrv.logger.Info("Deleted", "zid", ad.zid)
				asrv.analyser.processDeleted(&ad, &sd)
				return
			}
			asrv.logger.Info("Upsert", "zid", ad.zid)
			if ad.zid != ad.zettel.Zid {
				asrv.logger.Error("Differing Zids", "queue", ad.zid, "parsed", ad.zettel.Zid)
				ad.zettel.Zid = ad.zid
			}
			asrv.analyser.analyze(&ad, &sd)
		})
		if !ok {
			asrv.pending.dropped(ad.bulk)
			continue
		}
		if !send(ctx, out, sd) {
			break
		}
	}

	asrv.logger.Info("Stop")
	close(out)
}

// ----- store service ----------

type storeData struct {
	zid  id.Zid
	dzid denseZid
	aztl *analyzedZettel // nil, if the zettel was deleted; owned by the store step
	bulk bool
}

type storeService struct {
	logger  *slog.Logger
	store   *store
	pending *pendingQueue
}

func (ssrv *storeService) service(ctx context.Context, in <-chan storeData) {
	for sd := range recvSeq(ctx, in) {
		ok := runRecovered("IndexStore", func() {
			if sd.aztl == nil {
				ssrv.logger.Info("Deleted", "zid", sd.zid, "dzid", sd.dzid)
				ssrv.store.removeZettel(sd.dzid)
				return
			}
			ssrv.logger.Info("Upsert", "zid", sd.zid, "dzid", sd.dzid)
			ssrv.store.upsertZettel(sd.dzid, sd.aztl)
		})
		if sd.bulk {
			ssrv.pending.finishBulk(!ok)
		}
	}

	ssrv.logger.Info("Stop")
}
