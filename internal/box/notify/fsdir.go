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
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"zettelstore.de/z/internal/logging"
)

type fsdirNotifier struct {
	logger  *slog.Logger
	events  chan Event
	done    chan struct{}
	refresh chan struct{}
	base    *fsnotify.Watcher
	path    string
	fetcher EntryFetcher
	parent  string
}

// NewFSDirNotifier creates a directory based notifier that receives notifications
// from the file system.
func NewFSDirNotifier(logger *slog.Logger, path string) (Notifier, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		logger.Debug("Unable to create absolute path", "err", err, "path", path)
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Debug("Unable to create watcher", "err", err, "absPath", absPath)
		return nil, err
	}
	absParentDir := filepath.Dir(absPath)
	errParent := watcher.Add(absParentDir)
	err = watcher.Add(absPath)
	if errParent != nil {
		if err != nil {
			logger.Error("Unable to access Zettel directory and its parent directory",
				"parentDir", absParentDir, "err", errParent, "path", absPath, "err", err)
			_ = watcher.Close()
			return nil, err
		}
		logger.Info("Parent of Zettel directory cannot be supervised", "parentDir", absParentDir, "err", errParent)
		logger.Info("Zettelstore might not detect a deletion or movement of the Zettel directory", "path", absPath)
	} else if err != nil {
		// Not a problem, if container is not available. It might become available later.
		logger.Info("Zettel directory currently not available", "err", err, "path", absPath)
	}

	fsdn := &fsdirNotifier{
		logger:  logger,
		events:  make(chan Event),
		refresh: make(chan struct{}),
		done:    make(chan struct{}),
		base:    watcher,
		path:    absPath,
		fetcher: newDirPathFetcher(absPath),
		parent:  absParentDir,
	}
	go fsdn.eventLoop()
	return fsdn, nil
}

func (fsdn *fsdirNotifier) Events() <-chan Event {
	return fsdn.events
}

func (fsdn *fsdirNotifier) Refresh() {
	fsdn.refresh <- struct{}{}
}

func (fsdn *fsdirNotifier) eventLoop() {
	defer func() { _ = fsdn.base.Close() }()
	defer close(fsdn.events)
	defer close(fsdn.refresh)
	if !listDirElements(fsdn.logger, fsdn.fetcher, fsdn.events, fsdn.done) {
		return
	}

	for fsdn.readAndProcessEvent() {
	}
}

func (fsdn *fsdirNotifier) readAndProcessEvent() bool {
	select {
	case <-fsdn.done:
		fsdn.traceDone(1)
		return false
	default:
	}
	select {
	case <-fsdn.done:
		fsdn.traceDone(2)
		return false
	case <-fsdn.refresh:
		logging.LogTrace(fsdn.logger, "refresh")
		listDirElements(fsdn.logger, fsdn.fetcher, fsdn.events, fsdn.done)
	case err, ok := <-fsdn.base.Errors:
		logging.LogTrace(fsdn.logger, "got errors", "err", err, "ok", ok)
		if !ok {
			return false
		}
		select {
		case fsdn.events <- Event{Op: Error, Err: err}:
		case <-fsdn.done:
			fsdn.traceDone(3)
			return false
		}
	case ev, ok := <-fsdn.base.Events:
		logging.LogTrace(fsdn.logger, "file event", "name", ev.Name, "op", ev.Op, "ok", ok)
		if !ok {
			return false
		}
		if !fsdn.processEvent(&ev) {
			return false
		}
	}
	return true
}

func (fsdn *fsdirNotifier) traceDone(pos int64) {
	logging.LogTrace(fsdn.logger, "done with read and process events", "i", pos)
}

func (fsdn *fsdirNotifier) processEvent(ev *fsnotify.Event) bool {
	if strings.HasPrefix(ev.Name, fsdn.path) {
		if len(ev.Name) == len(fsdn.path) {
			return fsdn.processDirEvent(ev)
		}
		return fsdn.processFileEvent(ev)
	}
	logging.LogTrace(fsdn.logger, "event does not match", "path", fsdn.path, "name", ev.Name, "op", ev.Op)
	return true
}

func (fsdn *fsdirNotifier) processDirEvent(ev *fsnotify.Event) bool {
	if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
		fsdn.logger.Debug("Directory removed", "name", fsdn.path)
		_ = fsdn.base.Remove(fsdn.path)
		select {
		case fsdn.events <- Event{Op: Destroy}:
		case <-fsdn.done:
			logging.LogTrace(fsdn.logger, "done dir event processing", "i", 1)
			return false
		}
		return true
	}

	if ev.Has(fsnotify.Create) {
		err := fsdn.base.Add(fsdn.path)
		if err != nil {
			fsdn.logger.Error("Unable to add directory", "err", err, "name", fsdn.path)
			select {
			case fsdn.events <- Event{Op: Error, Err: err}:
			case <-fsdn.done:
				logging.LogTrace(fsdn.logger, "done dir event processing", "i", 2)
				return false
			}
		}
		fsdn.logger.Debug("Directory added", "name", fsdn.path)
		return listDirElements(fsdn.logger, fsdn.fetcher, fsdn.events, fsdn.done)
	}

	logging.LogTrace(fsdn.logger, "Directory processed", "name", ev.Name, "op", ev.Op)
	return true
}

func (fsdn *fsdirNotifier) processFileEvent(ev *fsnotify.Event) bool {
	if ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) {
		if fi, err := os.Lstat(ev.Name); err != nil || !fi.Mode().IsRegular() {
			regular := err == nil && fi.Mode().IsRegular()
			logging.LogTrace(fsdn.logger, "error with file",
				"name", ev.Name, "op", ev.Op, "err", err, "regular", regular)
			return true
		}
		logging.LogTrace(fsdn.logger, "File updated", "name", ev.Name, "op", ev.Op)
		return fsdn.sendEvent(Update, filepath.Base(ev.Name))
	}

	if ev.Has(fsnotify.Rename) {
		fi, err := os.Lstat(ev.Name)
		if err != nil {
			logging.LogTrace(fsdn.logger, "File deleted", "name", ev.Name, "op", ev.Op)
			return fsdn.sendEvent(Delete, filepath.Base(ev.Name))
		}
		if fi.Mode().IsRegular() {
			logging.LogTrace(fsdn.logger, "File updated", "name", ev.Name, "op", ev.Op)
			return fsdn.sendEvent(Update, filepath.Base(ev.Name))
		}
		logging.LogTrace(fsdn.logger, "File not regular", "name", ev.Name)
		return true
	}

	if ev.Has(fsnotify.Remove) {
		logging.LogTrace(fsdn.logger, "File deleted", "name", ev.Name, "op", ev.Op)
		return fsdn.sendEvent(Delete, filepath.Base(ev.Name))
	}

	logging.LogTrace(fsdn.logger, "File processed", "name", ev.Name, "op", ev.Op)
	return true
}

func (fsdn *fsdirNotifier) sendEvent(op EventOp, filename string) bool {
	select {
	case fsdn.events <- Event{Op: op, Name: filename}:
	case <-fsdn.done:
		logging.LogTrace(fsdn.logger, "done file event processing")
		return false
	}
	return true
}

func (fsdn *fsdirNotifier) Close() {
	close(fsdn.done)
}
