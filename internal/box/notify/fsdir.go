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
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"zettelstore.de/z/internal/logger"
)

type fsdirNotifier struct {
	log     *logger.Logger
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
func NewFSDirNotifier(log *logger.Logger, path string) (Notifier, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Debug().Err(err).Str("path", path).Msg("Unable to create absolute path")
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Debug().Err(err).Str("absPath", absPath).Msg("Unable to create watcher")
		return nil, err
	}
	absParentDir := filepath.Dir(absPath)
	errParent := watcher.Add(absParentDir)
	err = watcher.Add(absPath)
	if errParent != nil {
		if err != nil {
			log.Error().
				Str("parentDir", absParentDir).Err(errParent).
				Str("path", absPath).Err(err).
				Msg("Unable to access Zettel directory and its parent directory")
			watcher.Close()
			return nil, err
		}
		log.Info().Str("parentDir", absParentDir).Err(errParent).
			Msg("Parent of Zettel directory cannot be supervised")
		log.Info().Str("path", absPath).
			Msg("Zettelstore might not detect a deletion or movement of the Zettel directory")
	} else if err != nil {
		// Not a problem, if container is not available. It might become available later.
		log.Info().Err(err).Str("path", absPath).Msg("Zettel directory currently not available")
	}

	fsdn := &fsdirNotifier{
		log:     log,
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
	defer fsdn.base.Close()
	defer close(fsdn.events)
	defer close(fsdn.refresh)
	if !listDirElements(fsdn.log, fsdn.fetcher, fsdn.events, fsdn.done) {
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
		fsdn.log.Trace().Msg("refresh")
		listDirElements(fsdn.log, fsdn.fetcher, fsdn.events, fsdn.done)
	case err, ok := <-fsdn.base.Errors:
		fsdn.log.Trace().Err(err).Bool("ok", ok).Msg("got errors")
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
		fsdn.log.Trace().Str("name", ev.Name).Str("op", ev.Op.String()).Bool("ok", ok).Msg("file event")
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
	fsdn.log.Trace().Int("i", pos).Msg("done with read and process events")
}

func (fsdn *fsdirNotifier) processEvent(ev *fsnotify.Event) bool {
	if strings.HasPrefix(ev.Name, fsdn.path) {
		if len(ev.Name) == len(fsdn.path) {
			return fsdn.processDirEvent(ev)
		}
		return fsdn.processFileEvent(ev)
	}
	fsdn.log.Trace().Str("path", fsdn.path).Str("name", ev.Name).Str("op", ev.Op.String()).Msg("event does not match")
	return true
}

func (fsdn *fsdirNotifier) processDirEvent(ev *fsnotify.Event) bool {
	if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
		fsdn.log.Debug().Str("name", fsdn.path).Msg("Directory removed")
		fsdn.base.Remove(fsdn.path)
		select {
		case fsdn.events <- Event{Op: Destroy}:
		case <-fsdn.done:
			fsdn.log.Trace().Int("i", 1).Msg("done dir event processing")
			return false
		}
		return true
	}

	if ev.Has(fsnotify.Create) {
		err := fsdn.base.Add(fsdn.path)
		if err != nil {
			fsdn.log.Error().Err(err).Str("name", fsdn.path).Msg("Unable to add directory")
			select {
			case fsdn.events <- Event{Op: Error, Err: err}:
			case <-fsdn.done:
				fsdn.log.Trace().Int("i", 2).Msg("done dir event processing")
				return false
			}
		}
		fsdn.log.Debug().Str("name", fsdn.path).Msg("Directory added")
		return listDirElements(fsdn.log, fsdn.fetcher, fsdn.events, fsdn.done)
	}

	fsdn.log.Trace().Str("name", ev.Name).Str("op", ev.Op.String()).Msg("Directory processed")
	return true
}

func (fsdn *fsdirNotifier) processFileEvent(ev *fsnotify.Event) bool {
	if ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) {
		if fi, err := os.Lstat(ev.Name); err != nil || !fi.Mode().IsRegular() {
			regular := err == nil && fi.Mode().IsRegular()
			fsdn.log.Trace().Str("name", ev.Name).Str("op", ev.Op.String()).Err(err).Bool("regular", regular).Msg("error with file")
			return true
		}
		fsdn.log.Trace().Str("name", ev.Name).Str("op", ev.Op.String()).Msg("File updated")
		return fsdn.sendEvent(Update, filepath.Base(ev.Name))
	}

	if ev.Has(fsnotify.Rename) {
		fi, err := os.Lstat(ev.Name)
		if err != nil {
			fsdn.log.Trace().Str("name", ev.Name).Str("op", ev.Op.String()).Msg("File deleted")
			return fsdn.sendEvent(Delete, filepath.Base(ev.Name))
		}
		if fi.Mode().IsRegular() {
			fsdn.log.Trace().Str("name", ev.Name).Str("op", ev.Op.String()).Msg("File updated")
			return fsdn.sendEvent(Update, filepath.Base(ev.Name))
		}
		fsdn.log.Trace().Str("name", ev.Name).Msg("File not regular")
		return true
	}

	if ev.Has(fsnotify.Remove) {
		fsdn.log.Trace().Str("name", ev.Name).Str("op", ev.Op.String()).Msg("File deleted")
		return fsdn.sendEvent(Delete, filepath.Base(ev.Name))
	}

	fsdn.log.Trace().Str("name", ev.Name).Str("op", ev.Op.String()).Msg("File processed")
	return true
}

func (fsdn *fsdirNotifier) sendEvent(op EventOp, filename string) bool {
	select {
	case fsdn.events <- Event{Op: op, Name: filename}:
	case <-fsdn.done:
		fsdn.log.Trace().Msg("done file event processing")
		return false
	}
	return true
}

func (fsdn *fsdirNotifier) Close() {
	close(fsdn.done)
}
