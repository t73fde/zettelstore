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

package kernel

import (
	"log/slog"
	"sync"

	"zettelstore.de/z/internal/index"
)

type indexService struct {
	srvConfig
	mxService sync.RWMutex
	indexer   index.Indexer
	create    CreateIndexerFunc
}

func (is *indexService) Initialize(levelVar *slog.LevelVar, logger *slog.Logger) {
	is.logLevelVar = levelVar
	is.logger = logger
	is.descr = descriptionMap{}
	is.next = interfaceMap{}
}

func (is *indexService) GetLogger() *slog.Logger { return is.logger }
func (is *indexService) GetLevel() slog.Level    { return is.logLevelVar.Level() }
func (is *indexService) SetLevel(l slog.Level)   { is.logLevelVar.Set(l) }

func (is *indexService) Start(*Kernel) error {
	idx := is.create()
	is.logger.Info("Start Index")
	idx.Start()
	is.indexer = idx
	return nil
}
func (is *indexService) IsStarted() bool {
	is.mxService.RLock()
	started := is.indexer != nil
	is.mxService.RUnlock()
	return started
}
func (is *indexService) Stop(*Kernel) {
	is.logger.Info("Stop Index")
	is.mxService.RLock()
	idx := is.indexer
	is.mxService.RUnlock()
	idx.Stop()
	is.mxService.Lock()
	is.indexer = nil
	is.mxService.Unlock()

}
func (is *indexService) GetStatistics() []KeyValue { return nil }
