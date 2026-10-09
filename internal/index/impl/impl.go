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

// Package impl implements the index.Indexer interface.
package impl

import (
	"context"
	"log/slog"
	"sync"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/index"
	"zettelstore.de/z/internal/kernel"
)

var _ index.Indexer = (*Index)(nil)
var _ index.Enqueuer = (*Index)(nil)

// Index stores all data to provide an index.
type Index struct {
	logger *slog.Logger
	config config.Config

	mx     sync.RWMutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
	notify chan notifyData

	pending *pendingQueue
}

// New creates a new index object.
func New(config config.Config) *Index {
	return &Index{
		logger: kernel.Main.GetLogger(kernel.IndexService),
		config: config,
	}
}

// Enqueue a new zettel to update the search index.
func (idx *Index) Enqueue(fetcher index.Fetcher, reason box.UpdateReason, zid id.Zid) {
	idx.mx.RLock()
	defer idx.mx.RUnlock()
	if idx.notify == nil {
		return // not running
	}
	idx.notify <- notifyData{fetcher: fetcher, reason: reason, zid: zid}
}

type notifyData struct {
	fetcher index.Fetcher
	reason  box.UpdateReason
	zid     id.Zid
}
