//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

// Package dirbox provides a directory-based zettel box.
package dirbox

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/box/manager"
	"zettelstore.de/z/internal/box/notify"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/zettel"
)

func init() {
	manager.Register("dir", func(u *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
		var logger *slog.Logger
		if krnl := kernel.Main; krnl != nil {
			logger = krnl.GetLogger(kernel.BoxService).With("box", "dir", "boxnum", cdata.Number)
		}
		path := getDirPath(u)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		dp := dirBox{
			logger:     logger,
			number:     cdata.Number,
			location:   u.String(),
			readonly:   box.GetQueryBool(u, "readonly"),
			cdata:      *cdata,
			dir:        path,
			notifySpec: getDirSrvInfo(logger, u.Query().Get("type")),
			fSrvs:      makePrime(uint32(box.GetQueryInt(u, "worker", 1, 7, 1499))),
		}
		return &dp, nil
	})
}

func makePrime(n uint32) uint32 {
	for !isPrime(n) {
		n++
	}
	return n
}

func isPrime(n uint32) bool {
	if n == 0 {
		return false
	}
	if n <= 3 {
		return true
	}
	if n%2 == 0 {
		return false
	}
	for i := uint32(3); i*i <= n; i += 2 {
		if n%i == 0 {
			return false
		}
	}
	return true
}

type notifyTypeSpec int

const (
	_ notifyTypeSpec = iota
	dirNotifyAny
	dirNotifySimple
	dirNotifyFS
)

func getDirSrvInfo(logger *slog.Logger, notifyType string) notifyTypeSpec {
	for range 2 {
		switch notifyType {
		case kernel.BoxDirTypeNotify:
			return dirNotifyFS
		case kernel.BoxDirTypeSimple:
			return dirNotifySimple
		default:
			notifyType = kernel.Main.GetConfig(kernel.BoxService, kernel.BoxDefaultDirType).(string)
		}
	}
	logger.Error("Unable to set notify type, using a default", "notifyType", notifyType)
	return dirNotifySimple
}

func getDirPath(u *url.URL) string {
	if u.Opaque != "" {
		return filepath.Clean(u.Opaque)
	}
	return filepath.Clean(u.Path)
}

// dirBox uses a directory to store zettel as files.
type dirBox struct {
	logger     *slog.Logger
	number     int
	location   string
	readonly   bool
	cdata      manager.ConnectData
	dir        string
	notifySpec notifyTypeSpec
	dirSrv     *notify.DirService
	fSrvs      uint32
	fCmds      []chan fileCmd
	mxCmds     sync.RWMutex
}

func (dp *dirBox) Location() string {
	return dp.location
}

func (dp *dirBox) State() box.StartState {
	if ds := dp.dirSrv; ds != nil {
		switch ds.State() {
		case notify.DsCreated:
			return box.StartStateStopped
		case notify.DsStarting:
			return box.StartStateStarting
		case notify.DsWorking:
			return box.StartStateStarted
		case notify.DsMissing:
			return box.StartStateStarted
		case notify.DsStopping:
			return box.StartStateStopping
		}
	}
	return box.StartStateStopped
}

func (dp *dirBox) Start(context.Context) error {
	dp.mxCmds.Lock()
	defer dp.mxCmds.Unlock()
	dp.fCmds = make([]chan fileCmd, 0, dp.fSrvs)
	for i := range dp.fSrvs {
		cc := make(chan fileCmd)
		go fileService(i, dp.logger.With("sub", "file", "fn", i), dp.dir, cc)
		dp.fCmds = append(dp.fCmds, cc)
	}

	var notifier notify.Notifier
	var err error
	switch dp.notifySpec {
	case dirNotifySimple:
		notifier, err = notify.NewSimpleDirNotifier(dp.logger.With("notify", "simple"), dp.dir)
	default:
		notifier, err = notify.NewFSDirNotifier(dp.logger.With("notify", "fs"), dp.dir)
	}
	if err != nil {
		dp.logger.Error("Unable to create directory supervisor", "err", err)
		dp.stopFileServices()
		return err
	}
	dp.dirSrv = notify.NewDirService(
		dp,
		dp.logger.With("sub", "dirsrv"),
		notifier,
		dp.cdata.Notify,
	)
	dp.dirSrv.Start()
	return nil
}

func (dp *dirBox) Refresh(_ context.Context) {
	dp.dirSrv.Refresh()
	logging.LogTrace(dp.logger, "Refresh")
}

func (dp *dirBox) Stop(_ context.Context) {
	dirSrv := dp.dirSrv
	dp.dirSrv = nil
	if dirSrv != nil {
		dirSrv.Stop()
	}
	dp.stopFileServices()
}

func (dp *dirBox) stopFileServices() {
	for _, c := range dp.fCmds {
		close(c)
	}
}

func (dp *dirBox) notifyChanged(zid id.Zid, reason box.UpdateReason) {
	if notify := dp.cdata.Notify; notify != nil {
		logging.LogTrace(dp.logger, "notifyChanged", "zid", zid, "reason", reason)
		notify(dp, zid, reason)
	}
}

func (dp *dirBox) getFileChan(zid id.Zid) chan fileCmd {
	// Based on https://en.wikipedia.org/wiki/Fowler%E2%80%93Noll%E2%80%93Vo_hash_function
	sum := 2166136261 ^ uint32(zid)
	sum *= 16777619
	sum ^= uint32(zid >> 32)
	sum *= 16777619

	dp.mxCmds.RLock()
	defer dp.mxCmds.RUnlock()
	return dp.fCmds[sum%dp.fSrvs]
}

func (dp *dirBox) CanCreateZettel(_ context.Context) bool {
	return !dp.readonly
}

func (dp *dirBox) CreateZettel(ctx context.Context, zettel zettel.Zettel) (id.Zid, error) {
	if dp.readonly {
		return id.Invalid, box.ErrReadOnly
	}

	newZid, err := dp.dirSrv.SetNewDirEntry()
	if err != nil {
		return id.Invalid, err
	}
	meta := zettel.Meta
	meta.Zid = newZid
	entry := notify.DirEntry{Zid: newZid}
	dp.updateEntryFromMetaContent(&entry, meta, zettel.Content)

	err = dp.srvSetZettel(ctx, &entry, zettel)
	if err == nil {
		err = dp.dirSrv.UpdateDirEntry(&entry)
	}
	dp.notifyChanged(meta.Zid, box.OnZettel)
	logging.LogTrace(dp.logger, "CreateZettel", logging.Err(err), "zid", meta.Zid)
	return meta.Zid, err
}

func (dp *dirBox) GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error) {
	entry := dp.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		return zettel.Zettel{}, box.ErrZettelNotFound{Zid: zid}
	}
	m, c, err := dp.srvGetMetaContent(ctx, entry, zid)
	if err != nil {
		return zettel.Zettel{}, err
	}
	zettel := zettel.Zettel{Meta: m, Content: zettel.NewContent(c)}
	logging.LogTrace(dp.logger, "GetZettel", "zid", zid)
	return zettel, nil
}

func (dp *dirBox) HasZettel(_ context.Context, zid id.Zid) bool {
	return dp.dirSrv.GetDirEntry(zid).IsValid()
}

func (dp *dirBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint query.RetrievePredicate) error {
	entries := dp.dirSrv.GetDirEntries(constraint)
	logging.LogTrace(dp.logger, "ApplyZid", "entries", len(entries))
	for _, entry := range entries {
		handle(entry.Zid)
	}
	return nil
}

func (dp *dirBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint query.RetrievePredicate) error {
	entries := dp.dirSrv.GetDirEntries(constraint)
	logging.LogTrace(dp.logger, "ApplyMeta", "entries", len(entries))

	// The following loop could be parallelized if needed for performance.
	for _, entry := range entries {
		m, err := dp.srvGetMeta(ctx, entry, entry.Zid)
		if err != nil {
			logging.LogTrace(dp.logger, "ApplyMeta/getMeta", "err", err)
			return err
		}
		dp.cdata.Enricher.Enrich(ctx, m, dp.number)
		handle(m)
	}
	return nil
}

func (dp *dirBox) CanUpdateZettel(context.Context, zettel.Zettel) bool {
	return !dp.readonly
}

func (dp *dirBox) UpdateZettel(ctx context.Context, zettel zettel.Zettel) error {
	if dp.readonly {
		return box.ErrReadOnly
	}

	meta := zettel.Meta
	zid := meta.Zid
	if !zid.IsValid() {
		return box.ErrInvalidZid{Zid: zid.String()}
	}
	entry := dp.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		// Existing zettel, but new in this box.
		entry = &notify.DirEntry{Zid: zid}
	}
	dp.updateEntryFromMetaContent(entry, meta, zettel.Content)
	err := dp.dirSrv.UpdateDirEntry(entry)
	if err != nil {
		return err
	}
	err = dp.srvSetZettel(ctx, entry, zettel)
	if err == nil {
		dp.notifyChanged(zid, box.OnZettel)
	}
	logging.LogTrace(dp.logger, "UpdateZettel", "zid", zid, logging.Err(err))
	return err
}

func (dp *dirBox) updateEntryFromMetaContent(entry *notify.DirEntry, m *meta.Meta, content zettel.Content) {
	entry.SetupFromMetaContent(m, content, dp.cdata.Config.GetZettelFileSyntax)
}

func (dp *dirBox) CanDeleteZettel(_ context.Context, zid id.Zid) bool {
	if dp.readonly {
		return false
	}
	entry := dp.dirSrv.GetDirEntry(zid)
	return entry.IsValid()
}

func (dp *dirBox) DeleteZettel(ctx context.Context, zid id.Zid) error {
	if dp.readonly {
		return box.ErrReadOnly
	}

	entry := dp.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		return box.ErrZettelNotFound{Zid: zid}
	}
	err := dp.dirSrv.DeleteDirEntry(zid)
	if err != nil {
		return nil
	}
	err = dp.srvDeleteZettel(ctx, entry, zid)
	if err == nil {
		dp.notifyChanged(zid, box.OnDelete)
	}
	logging.LogTrace(dp.logger, "DeleteZettel", "zid", zid, logging.Err(err))
	return err
}

func (dp *dirBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = dp.readonly
	st.Zettel = dp.dirSrv.NumDirEntries()
	logging.LogTrace(dp.logger, "ReadStats", "zettel", st.Zettel)
}
