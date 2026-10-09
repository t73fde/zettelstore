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
	"zettelstore.de/z/internal/kernel"
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

	idx.store = &store{}

	ctx, cancel := context.WithCancel(context.Background())
	idx.cancel = cancel

	idx.pending = &pendingQueue{maxLoad: 1000} // TODO: maxLoad could be configurable?
	idx.notify = make(chan notifyData, 10)     // TODO: make 10 configurable?

	store := make(chan storeData, 4) // TODO: make configurable?
	parse := make(chan parseData, 4) // TODO: make configurable?
	wake := make(chan struct{}, 1)   // collector signals fetcher that new data was placed in pending.

	storeSrv := storeService{logger: idx.logger.With("service", "store")}
	parseSrv := parseService{logger: idx.logger.With("service", "parse"), config: idx.config}
	fetchSrv := fetchService{logger: idx.logger.With("service", "fetcher")}
	collectSrv := collectService{logger: idx.logger.With("service", "collector")}

	idx.spawn(ctx, "IndexStore", func() { storeSrv.service(store) })
	idx.spawn(ctx, "IndexParser", func() { parseSrv.service(ctx, parse, store) })
	idx.spawn(ctx, "IndexFetcher", func() { fetchSrv.service(ctx, wake, idx.pending, parse) })
	idx.spawn(ctx, "IndexCollector", func() { collectSrv.service(idx.notify, idx.pending, wake) })

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
	idx.cancel() // interrupts blocking GetZettel / FetchZids
	idx.wg.Wait()
	idx.store, idx.notify, idx.pending, idx.cancel = nil, nil, nil, nil
}

// collectService receives update notifications and adds them to an unlimited
// queue of pending jobs.
type collectService struct {
	logger *slog.Logger
}

func (csrv *collectService) service(notify <-chan notifyData, pending *pendingQueue, wake chan<- struct{}) {
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
			case box.OnReady:
				csrv.logger.Info("OnReady")
				continue
			case box.OnReload:
				csrv.logger.Info("OnReload")
				pending.reset(d.fetcher)
			case box.OnZettel:
				csrv.logger.Info("OnZettel", "zid", d.zid)
				pending.addZettel(d.fetcher, d.zid)
			case box.OnDelete:
				csrv.logger.Info("OnDelete", "zid", d.zid)
				pending.addZettel(d.fetcher, d.zid)
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
		}
	}
	csrv.logger.Debug("Good Buy")
	close(wake)
}

// ----- fetch service ----------

// fetchService reads from the pending queue and tries to fetch zettel.
type fetchService struct {
	logger *slog.Logger
}

func (fsrv *fetchService) service(ctx context.Context, wake <-chan struct{}, pending *pendingQueue, out chan<- parseData) {
	ctx = box.NoEnrichContext(ctx)
	for range wake {
		fsrv.logger.Info("WakeUp")
	loop:
		for {
			switch job := pending.getJob(); job.action {
			case pendingNothing:
				fsrv.logger.Info("Nothing")
				break loop

			case pendingZettel:
				fsrv.logger.Info("Zettel", "zid", job.zid, "fromReload", job.fromReload)
				zettel, err := job.fetcher.GetZettel(ctx, job.zid)
				if err != nil {
					if ctx.Err() != nil {
						break loop // cancelled by Stop: not an error, not a deletion
					}
					if _, notFound := errors.AsType[box.ErrZettelNotFound](err); notFound {
						out <- parseData{zid: job.zid, deleted: true}
					} else {
						fsrv.logger.Error("GetZettel", "zid", job.zid, "error", err)
					}
					continue
				}
				out <- parseData{zid: job.zid, zettel: zettel, deleted: false}

			case pendingReload:
				fsrv.logger.Info("Reload", "fromReload", job.fromReload)
				zids, err := job.fetcher.FetchZids(ctx)
				if err != nil {
					if ctx.Err() != nil {
						break loop // cancelled by Stop: not an error
					}
					fsrv.logger.Error("FetchZids", "error", err)
					continue
				}
				pending.reload(job.fetcher, zids)
			}
		}
	}
	fsrv.logger.Info("Byebye")
	close(out)
}

// ----- parse service ----------

type parseData struct {
	zid     id.Zid
	zettel  zettel.Zettel
	deleted bool
}

type parseService struct {
	logger *slog.Logger
	config config.Config
}

func (psrv *parseService) service(ctx context.Context, in <-chan parseData, out chan<- storeData) {
	ctx = box.NoEnrichContext(ctx)
	for zd := range in {
		if zd.deleted {
			psrv.logger.Info("Deleted", "zid", zd.zid)
			out <- storeData{zid: zd.zid, zettel: nil}
			continue
		}
		psrv.logger.Info("Update", "zid", zd.zid)
		ztl := parser.ParseZettel(ctx, zd.zettel, "", psrv.config)
		out <- storeData{zid: zd.zid, zettel: ztl}
	}
	psrv.logger.Info("NotTwoPi")
	close(out)
}

// ----- store service ----------

type storeData struct {
	zid    id.Zid // for use when zettel is deleted
	zettel *zettel.ParsedZettel
}

type storeService struct {
	logger *slog.Logger
}

func (ssrv *storeService) service(in <-chan storeData) {
	for sd := range in {
		if sd.zettel == nil {
			ssrv.logger.Info("Deleted", "zid", sd.zid)
			// TODO: remove data from store
			continue
		}
		ssrv.logger.Info("Update", "zid", sd.zid)
		// TODO: update zettel in store.

	}

	ssrv.logger.Info("Finalforget")
}

// ----- helper functions

// spawn runs a service in its own goroutine, restarts it after a panic,
// and registers it with the wait group for the whole time.
func (idx *Index) spawn(ctx context.Context, name string, run func()) {
	idx.wg.Go(func() {
		const (
			maxRestarts = 5           // within one window
			window      = time.Minute // restarts older than this are forgotten
			baseDelay   = 100 * time.Millisecond
			maxDelay    = 10 * time.Second
		)
		var (
			restarts int
			first    time.Time
			delay    = baseDelay
		)
		for !runRecovered(name, run) {
			now := time.Now()
			if restarts == 0 || now.Sub(first) > window {
				first, restarts, delay = now, 0, baseDelay // new window
			}
			restarts++
			if restarts > maxRestarts {
				idx.logger.Error("Giving up", "service", name, "restarts", restarts)
				return
			}
			idx.logger.Warn("Restarting", "service", name, "restart", restarts, "delay", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return // Stop was called during the pause
			}
			delay = min(delay*2, maxDelay)
		}
	})
}

// runRecovered returns false if run panicked.
func runRecovered(name string, run func()) (ok bool) {
	defer func() {
		if ri := recover(); ri != nil {
			kernel.Main.LogRecover(name, ri)
			ok = false
		}
	}()
	run()
	return true
}
