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
	"iter"
	"log/slog"
	"sync"
	"time"

	"zettelstore.de/z/internal/kernel"
)

type pipeline struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	wg     sync.WaitGroup
	logger *slog.Logger
}

func (p *pipeline) spawn(name string, run func()) {
	p.wg.Go(func() { p.supervise(name, run) })
}

func (p *pipeline) supervise(name string, run func()) {
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
			p.logger.Error("Giving up", "service", name, "restarts", restarts)
			return
		}
		p.logger.Warn("Restarting", "service", name, "restart", restarts, "delay", delay)
		select {
		case <-time.After(delay):
		case <-p.ctx.Done():
			return // Stop was called during the pause
		}
		delay = min(delay*2, maxDelay)
	}
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

// send delivers v unless ctx is done first.
func send[T any](ctx context.Context, ch chan<- T, v T) bool {
	select {
	case ch <- v:
		return true
	case <-ctx.Done():
		return false
	}
}

// recvSeq yields values until ch is closed or ctx is done.
func recvSeq[T any](ctx context.Context, ch <-chan T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			select {
			case v, ok := <-ch:
				if !ok || !yield(v) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}
