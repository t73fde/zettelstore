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
	"log/slog"
	"sync"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/index"
	"zettelstore.de/z/internal/kernel"
)

var _ index.Indexer = (*Index)(nil)
var _ index.Enqueuer = (*Index)(nil)
var _ index.Searcher = (*Index)(nil)

// Index stores all data to provide an index.
type Index struct {
	logger    *slog.Logger
	config    config.Config
	retriever index.Retriever

	lifecycleMx sync.Mutex   // serializes Start and Stop; may be held while waiting for the pipeline
	mx          sync.RWMutex // protects the fields below; never held while waiting
	pl          *pipeline
	notify      chan notifyData

	stats   *stats
	pending *pendingQueue
	dict    *dictionary
	store   *store
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

// Enqueue a new zettel to update the search index.
func (idx *Index) Enqueue(reason index.Reason, zid id.Zid) {
	idx.mx.RLock()
	defer idx.mx.RUnlock()
	if idx.notify == nil {
		return // not running
	}
	// dropped if the pipeline has failed
	send(idx.pl.ctx, idx.notify, notifyData{reason: reason, zid: zid})
}

type notifyData struct {
	reason index.Reason
	zid    id.Zid
}

// ReadStats populates st with index statistics.
func (idx *Index) ReadStats(st *index.Stats) {
	idx.mx.RLock()
	defer idx.mx.RUnlock()
	if idx.pl == nil {
		return // not running: st stays zero
	}

	var dst dictStats
	d := idx.store.dict
	d.RLock()
	defer d.RUnlock()
	d.readStats(&dst)

	st.NumZettel = dst.NumZettel
	st.NumWords = dst.NumWords
	st.Vocabulary = dst.Vocabulary
	st.WordCount = dst.WordCount

	stats := idx.stats
	stats.mx.RLock()
	defer stats.mx.RUnlock()
	st.UpdatedZettel = stats.updates
	st.RemovedZettel = stats.removes
	st.NumResets = stats.maxGen
}
