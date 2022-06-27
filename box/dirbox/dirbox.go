//-----------------------------------------------------------------------------
// Copyright (c) 2020-2022 Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package dirbox provides a directory-based zettel box.
package dirbox

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"zettelstore.de/z/box"
	"zettelstore.de/z/box/manager"
	"zettelstore.de/z/box/notify"
	"zettelstore.de/z/domain"
	"zettelstore.de/z/domain/id"
	"zettelstore.de/z/domain/meta"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/search"
)

func init() {
	manager.Register("dir", func(u *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
		var log *logger.Logger
		if krnl := kernel.Main; krnl != nil {
			log = krnl.GetLogger(kernel.BoxService).Clone().Str("box", "dir").Int("boxnum", int64(cdata.Number)).Child()
		}
		path := getDirPath(u)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		dp := dirBox{
			log:        log,
			number:     cdata.Number,
			location:   u.String(),
			readonly:   box.GetQueryBool(u, "readonly"),
			cdata:      *cdata,
			dir:        path,
			notifySpec: getDirSrvInfo(log, u.Query().Get("type")),
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

func getDirSrvInfo(log *logger.Logger, notifyType string) notifyTypeSpec {
	for count := 0; count < 2; count++ {
		switch notifyType {
		case kernel.BoxDirTypeNotify:
			return dirNotifyFS
		case kernel.BoxDirTypeSimple:
			return dirNotifySimple
		default:
			notifyType = kernel.Main.GetConfig(kernel.BoxService, kernel.BoxDefaultDirType).(string)
		}
	}
	log.Error().Str("notifyType", notifyType).Msg("Unable to set notify type, using a default")
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
	log        *logger.Logger
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

func (dp *dirBox) Start(context.Context) error {
	dp.mxCmds.Lock()
	defer dp.mxCmds.Unlock()
	dp.fCmds = make([]chan fileCmd, 0, dp.fSrvs)
	for i := uint32(0); i < dp.fSrvs; i++ {
		cc := make(chan fileCmd)
		go fileService(i, dp.log.Clone().Str("sub", "file").Uint("fn", uint64(i)).Child(), dp.dir, cc)
		dp.fCmds = append(dp.fCmds, cc)
	}

	var notifier notify.Notifier
	var err error
	switch dp.notifySpec {
	case dirNotifySimple:
		notifier, err = notify.NewSimpleDirNotifier(dp.log.Clone().Str("notify", "simple").Child(), dp.dir)
	default:
		notifier, err = notify.NewFSDirNotifier(dp.log.Clone().Str("notify", "fs").Child(), dp.dir)
	}
	if err != nil {
		dp.log.Fatal().Err(err).Msg("Unable to create directory supervisor")
		dp.stopFileServices()
		return err
	}
	dp.dirSrv = notify.NewDirService(
		dp.log.Clone().Str("sub", "dirsrv").Child(),
		notifier,
		dp.cdata.Notify,
	)
	dp.dirSrv.Start()
	return nil
}

func (dp *dirBox) Refresh(_ context.Context) {
	dp.dirSrv.Refresh()
	dp.log.Trace().Msg("Refresh")
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

func (dp *dirBox) notifyChanged(reason box.UpdateReason, zid id.Zid) {
	if chci := dp.cdata.Notify; chci != nil {
		dp.log.Trace().Zid(zid).Uint("reason", uint64(reason)).Msg("notifyChanged")
		chci <- box.UpdateInfo{Reason: reason, Zid: zid}
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

func (dp *dirBox) CreateZettel(ctx context.Context, zettel domain.Zettel) (id.Zid, error) {
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
	dp.notifyChanged(box.OnUpdate, meta.Zid)
	dp.log.Trace().Err(err).Zid(meta.Zid).Msg("CreateZettel")
	return meta.Zid, err
}

func (dp *dirBox) GetZettel(ctx context.Context, zid id.Zid) (domain.Zettel, error) {
	entry := dp.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		return domain.Zettel{}, box.ErrNotFound
	}
	m, c, err := dp.srvGetMetaContent(ctx, entry, zid)
	if err != nil {
		return domain.Zettel{}, err
	}
	zettel := domain.Zettel{Meta: m, Content: domain.NewContent(c)}
	dp.log.Trace().Zid(zid).Msg("GetZettel")
	return zettel, nil
}

func (dp *dirBox) GetMeta(ctx context.Context, zid id.Zid) (*meta.Meta, error) {
	m, err := dp.doGetMeta(ctx, zid)
	dp.log.Trace().Zid(zid).Err(err).Msg("GetMeta")
	return m, err
}
func (dp *dirBox) doGetMeta(ctx context.Context, zid id.Zid) (*meta.Meta, error) {
	entry := dp.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		return nil, box.ErrNotFound
	}
	m, err := dp.srvGetMeta(ctx, entry, zid)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (dp *dirBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint search.RetrievePredicate) error {
	entries := dp.dirSrv.GetDirEntries(constraint)
	dp.log.Trace().Int("entries", int64(len(entries))).Msg("ApplyZid")
	for _, entry := range entries {
		handle(entry.Zid)
	}
	return nil
}

func (dp *dirBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint search.RetrievePredicate) error {
	entries := dp.dirSrv.GetDirEntries(constraint)
	dp.log.Trace().Int("entries", int64(len(entries))).Msg("ApplyMeta")

	// The following loop could be parallelized if needed for performance.
	for _, entry := range entries {
		m, err := dp.srvGetMeta(ctx, entry, entry.Zid)
		if err != nil {
			dp.log.Trace().Err(err).Msg("ApplyMeta/getMeta")
			return err
		}
		dp.cdata.Enricher.Enrich(ctx, m, dp.number)
		handle(m)
	}
	return nil
}

func (dp *dirBox) CanUpdateZettel(context.Context, domain.Zettel) bool {
	return !dp.readonly
}

func (dp *dirBox) UpdateZettel(ctx context.Context, zettel domain.Zettel) error {
	if dp.readonly {
		return box.ErrReadOnly
	}

	meta := zettel.Meta
	zid := meta.Zid
	if !zid.IsValid() {
		return &box.ErrInvalidID{Zid: zid}
	}
	entry := dp.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		// Existing zettel, but new in this box.
		entry = &notify.DirEntry{Zid: zid}
	}
	dp.updateEntryFromMetaContent(entry, meta, zettel.Content)
	dp.dirSrv.UpdateDirEntry(entry)
	err := dp.srvSetZettel(ctx, entry, zettel)
	if err == nil {
		dp.notifyChanged(box.OnUpdate, zid)
	}
	dp.log.Trace().Zid(zid).Err(err).Msg("UpdateZettel")
	return err
}

func (dp *dirBox) updateEntryFromMetaContent(entry *notify.DirEntry, m *meta.Meta, content domain.Content) {
	entry.SetupFromMetaContent(m, content, dp.cdata.Config.GetZettelFileSyntax)
}

func (dp *dirBox) AllowRenameZettel(context.Context, id.Zid) bool {
	return !dp.readonly
}

func (dp *dirBox) RenameZettel(ctx context.Context, curZid, newZid id.Zid) error {
	if curZid == newZid {
		return nil
	}
	curEntry := dp.dirSrv.GetDirEntry(curZid)
	if !curEntry.IsValid() {
		return box.ErrNotFound
	}
	if dp.readonly {
		return box.ErrReadOnly
	}

	// Check whether zettel with new ID already exists in this box.
	if _, err := dp.doGetMeta(ctx, newZid); err == nil {
		return &box.ErrInvalidID{Zid: newZid}
	}

	oldMeta, oldContent, err := dp.srvGetMetaContent(ctx, curEntry, curZid)
	if err != nil {
		return err
	}

	newEntry, err := dp.dirSrv.RenameDirEntry(curEntry, newZid)
	if err != nil {
		return err
	}
	oldMeta.Zid = newZid
	newZettel := domain.Zettel{Meta: oldMeta, Content: domain.NewContent(oldContent)}
	if err = dp.srvSetZettel(ctx, &newEntry, newZettel); err != nil {
		// "Rollback" rename. No error checking...
		dp.dirSrv.RenameDirEntry(&newEntry, curZid)
		return err
	}
	err = dp.srvDeleteZettel(ctx, curEntry, curZid)
	if err == nil {
		dp.notifyChanged(box.OnDelete, curZid)
		dp.notifyChanged(box.OnUpdate, newZid)
	}
	dp.log.Trace().Zid(curZid).Zid(newZid).Err(err).Msg("RenameZettel")
	return err
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
		return box.ErrNotFound
	}
	err := dp.dirSrv.DeleteDirEntry(zid)
	if err != nil {
		return nil
	}
	err = dp.srvDeleteZettel(ctx, entry, zid)
	if err == nil {
		dp.notifyChanged(box.OnDelete, zid)
	}
	dp.log.Trace().Zid(zid).Err(err).Msg("DeleteZettel")
	return err
}

func (dp *dirBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = dp.readonly
	st.Zettel = dp.dirSrv.NumDirEntries()
	dp.log.Trace().Int("zettel", int64(st.Zettel)).Msg("ReadStats")
}
