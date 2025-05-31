//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package notify

import (
	"log/slog"
	"path/filepath"
)

type simpleDirNotifier struct {
	logger  *slog.Logger
	events  chan Event
	done    chan struct{}
	refresh chan struct{}
	fetcher EntryFetcher
}

// NewSimpleDirNotifier creates a directory based notifier that will not receive
// any notifications from the operating system.
func NewSimpleDirNotifier(logger *slog.Logger, path string) (Notifier, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	sdn := &simpleDirNotifier{
		logger:  logger,
		events:  make(chan Event),
		done:    make(chan struct{}),
		refresh: make(chan struct{}),
		fetcher: newDirPathFetcher(absPath),
	}
	go sdn.eventLoop()
	return sdn, nil
}

// NewSimpleZipNotifier creates a zip-file based notifier that will not receive
// any notifications from the operating system.
func NewSimpleZipNotifier(logger *slog.Logger, zipPath string) Notifier {
	sdn := &simpleDirNotifier{
		logger:  logger,
		events:  make(chan Event),
		done:    make(chan struct{}),
		refresh: make(chan struct{}),
		fetcher: newZipPathFetcher(zipPath),
	}
	go sdn.eventLoop()
	return sdn
}

func (sdn *simpleDirNotifier) Events() <-chan Event {
	return sdn.events
}

func (sdn *simpleDirNotifier) Refresh() {
	sdn.refresh <- struct{}{}
}

func (sdn *simpleDirNotifier) eventLoop() {
	defer close(sdn.events)
	defer close(sdn.refresh)
	if !listDirElements(sdn.logger, sdn.fetcher, sdn.events, sdn.done) {
		return
	}
	for {
		select {
		case <-sdn.done:
			return
		case <-sdn.refresh:
			listDirElements(sdn.logger, sdn.fetcher, sdn.events, sdn.done)
		}
	}
}

func (sdn *simpleDirNotifier) Close() {
	close(sdn.done)
}
