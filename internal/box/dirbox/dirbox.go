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
	"zettelstore.de/z/internal/zettel"
)

func init() {
	manager.Register(
		box.SchemeDirBox,
		func(u *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
			q := u.Query()
			name := q.Get(manager.QueryName)
			var logger *slog.Logger
			if krnl := kernel.Main; krnl != nil {
				logger = krnl.GetLogger(kernel.BoxService).With(
					"box", box.SchemeDirBox, "name", name)
			}
			path := getDirPath(u)
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			dp := dirBox{
				logger:     logger,
				name:       name,
				location:   u.String(),
				readonly:   box.GetQueryBool(u, manager.QueryReadOnly),
				cdata:      *cdata,
				dir:        path,
				notifySpec: getDirSrvInfo(logger, q.Get("type")),
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
	name       string
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

func (dbx *dirBox) Name() string     { return dbx.name }
func (dbx *dirBox) Location() string { return dbx.location }

func (dbx *dirBox) State() box.StartState {
	if ds := dbx.dirSrv; ds != nil {
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

func (dbx *dirBox) Start(context.Context) error {
	dbx.mxCmds.Lock()
	defer dbx.mxCmds.Unlock()
	dbx.fCmds = make([]chan fileCmd, 0, dbx.fSrvs)
	for i := range dbx.fSrvs {
		cc := make(chan fileCmd)
		go fileService(i, dbx.logger.With("sub", "file", "fn", i), dbx.dir, cc)
		dbx.fCmds = append(dbx.fCmds, cc)
	}

	var notifier notify.Notifier
	var err error
	switch dbx.notifySpec {
	case dirNotifySimple:
		notifier, err = notify.NewSimpleDirNotifier(dbx.logger.With("notify", "simple"), dbx.dir)
	default:
		notifier, err = notify.NewFSDirNotifier(dbx.logger.With("notify", "fs"), dbx.dir)
	}
	if err != nil {
		dbx.logger.Error("Unable to create directory supervisor", "err", err)
		dbx.stopFileServices()
		return err
	}
	dbx.dirSrv = notify.NewDirService(
		dbx,
		dbx.logger.With("sub", "dirsrv"),
		notifier,
		dbx.cdata.Notify,
	)
	dbx.dirSrv.Start()
	return nil
}

func (dbx *dirBox) Refresh(context.Context) {
	dbx.dirSrv.Refresh()
	logging.LogTrace(dbx.logger, "Refresh")
}

func (dbx *dirBox) Stop(context.Context) {
	dirSrv := dbx.dirSrv
	dbx.dirSrv = nil
	if dirSrv != nil {
		dirSrv.Stop()
	}
	dbx.stopFileServices()
}

func (dbx *dirBox) stopFileServices() {
	for _, c := range dbx.fCmds {
		close(c)
	}
}

func (dbx *dirBox) notifyChanged(zid id.Zid, reason box.UpdateReason) {
	if notify := dbx.cdata.Notify; notify != nil {
		logging.LogTrace(dbx.logger, "notifyChanged", "zid", zid, "reason", reason)
		notify(dbx, zid, reason)
	}
}

func (dbx *dirBox) getFileChan(zid id.Zid) chan fileCmd {
	// Based on https://en.wikipedia.org/wiki/Fowler%E2%80%93Noll%E2%80%93Vo_hash_function
	sum := 2166136261 ^ uint32(zid)
	sum *= 16777619
	sum ^= uint32(zid >> 32)
	sum *= 16777619

	dbx.mxCmds.RLock()
	defer dbx.mxCmds.RUnlock()
	return dbx.fCmds[sum%dbx.fSrvs]
}

func (dbx *dirBox) CanCreateZettel(context.Context) bool {
	return !dbx.readonly
}

func (dbx *dirBox) CreateZettel(ctx context.Context, zettel box.Zettel) (id.Zid, error) {
	if dbx.readonly {
		return id.Invalid, box.ErrReadOnly
	}

	newZid, err := dbx.dirSrv.SetNewDirEntry()
	if err != nil {
		return id.Invalid, err
	}
	meta := zettel.Meta
	meta.Zid = newZid
	entry := notify.DirEntry{Zid: newZid}
	dbx.updateEntryFromMetaContent(&entry, meta, zettel.Content)

	err = dbx.srvSetZettel(ctx, &entry, zettel)
	if err == nil {
		err = dbx.dirSrv.UpdateDirEntry(&entry)
	}
	dbx.notifyChanged(meta.Zid, box.OnZettel)
	logging.LogTrace(dbx.logger, "CreateZettel", logging.Err(err), "zid", meta.Zid)
	return meta.Zid, err
}

func (dbx *dirBox) GetZettel(ctx context.Context, zid id.Zid) (box.Zettel, error) {
	entry := dbx.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		return box.Zettel{}, box.ErrZettelNotFound{Zid: zid}
	}
	m, c, err := dbx.srvGetMetaContent(ctx, entry, zid)
	if err != nil {
		return box.Zettel{}, err
	}
	zettel := box.Zettel{Meta: m, Content: zettel.NewContent(c)}
	logging.LogTrace(dbx.logger, "GetZettel", "zid", zid)
	return zettel, nil
}

func (dbx *dirBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint box.RetrievePredicate) error {
	entries := dbx.dirSrv.GetDirEntries(constraint)
	logging.LogTrace(dbx.logger, "ApplyZid", "entries", len(entries))
	for _, entry := range entries {
		handle(entry.Zid)
	}
	return nil
}

func (dbx *dirBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint box.RetrievePredicate) error {
	entries := dbx.dirSrv.GetDirEntries(constraint)
	logging.LogTrace(dbx.logger, "ApplyMeta", "entries", len(entries))

	// The following loop could be parallelized if needed for performance.
	for _, entry := range entries {
		m, err := dbx.srvGetMeta(ctx, entry, entry.Zid)
		if err != nil {
			logging.LogTrace(dbx.logger, "ApplyMeta/getMeta", "err", err)
			return err
		}
		dbx.cdata.Enricher.Enrich(ctx, m, dbx.name)
		handle(m)
	}
	return nil
}

func (dbx *dirBox) CanUpdateZettel(context.Context, box.Zettel) bool {
	return !dbx.readonly
}

func (dbx *dirBox) UpdateZettel(ctx context.Context, zettel box.Zettel) error {
	if dbx.readonly {
		return box.ErrReadOnly
	}

	meta := zettel.Meta
	zid := meta.Zid
	if !zid.IsValid() {
		return box.ErrInvalidZid{Zid: zid.String()}
	}
	entry := dbx.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		// Existing zettel, but new in this box.
		entry = &notify.DirEntry{Zid: zid}
	}
	dbx.updateEntryFromMetaContent(entry, meta, zettel.Content)
	err := dbx.dirSrv.UpdateDirEntry(entry)
	if err != nil {
		return err
	}
	err = dbx.srvSetZettel(ctx, entry, zettel)
	if err == nil {
		dbx.notifyChanged(zid, box.OnZettel)
	}
	logging.LogTrace(dbx.logger, "UpdateZettel", "zid", zid, logging.Err(err))
	return err
}

func (dbx *dirBox) updateEntryFromMetaContent(entry *notify.DirEntry, m *meta.Meta, content zettel.Content) {
	entry.SetupFromMetaContent(m, content, dbx.cdata.Config.IsZettelFileSyntax)
}

func (dbx *dirBox) CanDeleteZettel(_ context.Context, zid id.Zid) bool {
	if dbx.readonly {
		return false
	}
	entry := dbx.dirSrv.GetDirEntry(zid)
	return entry.IsValid()
}

func (dbx *dirBox) DeleteZettel(ctx context.Context, zid id.Zid) error {
	if dbx.readonly {
		return box.ErrReadOnly
	}

	entry := dbx.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		return box.ErrZettelNotFound{Zid: zid}
	}
	err := dbx.dirSrv.DeleteDirEntry(zid)
	if err != nil {
		return nil
	}
	err = dbx.srvDeleteZettel(ctx, entry, zid)
	if err == nil {
		dbx.notifyChanged(zid, box.OnDelete)
	}
	logging.LogTrace(dbx.logger, "DeleteZettel", "zid", zid, logging.Err(err))
	return err
}

func (dbx *dirBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = dbx.readonly
	st.Zettel = dbx.dirSrv.NumDirEntries()
	logging.LogTrace(dbx.logger, "ReadStats", "zettel", st.Zettel)
}
