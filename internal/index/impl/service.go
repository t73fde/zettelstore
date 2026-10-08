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
	"time"

	"zettelstore.de/z/internal/kernel"
)

// Implements the background service(s).

// Start the search index.
func (idx *Index) Start() {
	idx.mx.Lock()
	defer idx.mx.Unlock()
	if idx.queue != nil {
		return // already running
	}
	queue := make(chan queueData, 10) // TODO: make 10 configurable
	idx.queue = queue
	go idx.backgroundService(queue)
	idx.logger.Debug("Started")
}

// Stop the search index.
func (idx *Index) Stop() {
	idx.mx.Lock()
	defer idx.mx.Unlock()
	if idx.queue == nil {
		return // not running
	}
	idx.logger.Debug("Stop")
	close(idx.queue)
	idx.queue = nil
}

func (idx *Index) backgroundService(queue <-chan queueData) {
	// Something may panic. Ensure a running search index.
	defer func() {
		if ri := recover(); ri != nil {
			kernel.Main.LogRecover("Index", ri)
			go idx.backgroundService(queue)
		}
	}()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
loop:
	for {
		select {
		case d, ok := <-queue:
			if !ok {
				break loop
			}
			idx.logger.Debug("Index", "reason", d.reason, "zid", d.zid)
		case <-ticker.C:
			idx.logger.Debug("Tick") // placeholder for a real periodic task
		}
	}
	idx.logger.Debug("Good Buy")
}
