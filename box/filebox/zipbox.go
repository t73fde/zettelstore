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

package filebox

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"strings"

	"t73f.de/r/zsc/input"
	"zettelstore.de/z/box"
	"zettelstore.de/z/box/notify"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

type zipBox struct {
	log      *logger.Logger
	number   int
	name     string
	enricher box.Enricher
	notify   chan<- box.UpdateInfo
	dirSrv   *notify.DirService
}

func (zb *zipBox) Location() string {
	if strings.HasPrefix(zb.name, "/") {
		return "file://" + zb.name
	}
	return "file:" + zb.name
}

func (zb *zipBox) State() box.StartState {
	if ds := zb.dirSrv; ds != nil {
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

func (zb *zipBox) Start(context.Context) error {
	reader, err := zip.OpenReader(zb.name)
	if err != nil {
		return err
	}
	reader.Close()
	zipNotifier := notify.NewSimpleZipNotifier(zb.log, zb.name)
	zb.dirSrv = notify.NewDirService(zb, zb.log, zipNotifier, zb.notify)
	zb.dirSrv.Start()
	return nil
}

func (zb *zipBox) Refresh(_ context.Context) {
	zb.dirSrv.Refresh()
	zb.log.Trace().Msg("Refresh")
}

func (zb *zipBox) Stop(context.Context) {
	zb.dirSrv.Stop()
	zb.dirSrv = nil
}

func (zb *zipBox) GetZettel(_ context.Context, zid id.ZidO) (zettel.Zettel, error) {
	entry := zb.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		return zettel.Zettel{}, box.ErrZettelNotFound{Zid: zid}
	}
	reader, err := zip.OpenReader(zb.name)
	if err != nil {
		return zettel.Zettel{}, err
	}
	defer reader.Close()

	var m *meta.Meta
	var src []byte
	var inMeta bool

	contentName := entry.ContentName
	if metaName := entry.MetaName; metaName == "" {
		if contentName == "" {
			err = fmt.Errorf("no meta, no content in getZettel, zid=%v", zid)
			return zettel.Zettel{}, err
		}
		src, err = readZipFileContent(reader, entry.ContentName)
		if err != nil {
			return zettel.Zettel{}, err
		}
		if entry.HasMetaInContent() {
			inp := input.NewInput(src)
			m = meta.NewFromInput(zid, inp)
			src = src[inp.Pos:]
		} else {
			m = CalcDefaultMeta(zid, entry.ContentExt)
		}
	} else {
		m, err = readZipMetaFile(reader, zid, metaName)
		if err != nil {
			return zettel.Zettel{}, err
		}
		inMeta = true
		if contentName != "" {
			src, err = readZipFileContent(reader, entry.ContentName)
			if err != nil {
				return zettel.Zettel{}, err
			}
		}
	}

	CleanupMeta(m, zid, entry.ContentExt, inMeta, entry.UselessFiles)
	zb.log.Trace().Zid(zid).Msg("GetZettel")
	return zettel.Zettel{Meta: m, Content: zettel.NewContent(src)}, nil
}

func (zb *zipBox) HasZettel(_ context.Context, zid id.ZidO) bool {
	return zb.dirSrv.GetDirEntry(zid).IsValid()
}

func (zb *zipBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint query.RetrievePredicate) error {
	entries := zb.dirSrv.GetDirEntries(constraint)
	zb.log.Trace().Int("entries", int64(len(entries))).Msg("ApplyZid")
	for _, entry := range entries {
		handle(entry.Zid)
	}
	return nil
}

func (zb *zipBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint query.RetrievePredicate) error {
	reader, err := zip.OpenReader(zb.name)
	if err != nil {
		return err
	}
	defer reader.Close()
	entries := zb.dirSrv.GetDirEntries(constraint)
	zb.log.Trace().Int("entries", int64(len(entries))).Msg("ApplyMeta")
	for _, entry := range entries {
		if !constraint(entry.Zid) {
			continue
		}
		m, err2 := zb.readZipMeta(reader, entry.Zid, entry)
		if err2 != nil {
			continue
		}
		zb.enricher.Enrich(ctx, m, zb.number)
		handle(m)
	}
	return nil
}

func (zb *zipBox) AllowRenameZettel(_ context.Context, zid id.ZidO) bool {
	entry := zb.dirSrv.GetDirEntry(zid)
	return !entry.IsValid()
}

func (zb *zipBox) RenameZettel(_ context.Context, curZid, newZid id.ZidO) error {
	err := box.ErrReadOnly
	if curZid == newZid {
		err = nil
	}
	curEntry := zb.dirSrv.GetDirEntry(curZid)
	if !curEntry.IsValid() {
		err = box.ErrZettelNotFound{Zid: curZid}
	}
	zb.log.Trace().Err(err).Msg("RenameZettel")
	return err
}

func (*zipBox) CanDeleteZettel(context.Context, id.ZidO) bool { return false }

func (zb *zipBox) DeleteZettel(_ context.Context, zid id.ZidO) error {
	err := box.ErrReadOnly
	entry := zb.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		err = box.ErrZettelNotFound{Zid: zid}
	}
	zb.log.Trace().Err(err).Msg("DeleteZettel")
	return err
}

func (zb *zipBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = true
	st.Zettel = zb.dirSrv.NumDirEntries()
	zb.log.Trace().Int("zettel", int64(st.Zettel)).Msg("ReadStats")
}

func (zb *zipBox) readZipMeta(reader *zip.ReadCloser, zid id.ZidO, entry *notify.DirEntry) (m *meta.Meta, err error) {
	var inMeta bool
	if metaName := entry.MetaName; metaName == "" {
		contentName := entry.ContentName
		contentExt := entry.ContentExt
		if contentName == "" || contentExt == "" {
			err = fmt.Errorf("no meta, no content in getMeta, zid=%v", zid)
		} else if entry.HasMetaInContent() {
			m, err = readZipMetaFile(reader, zid, contentName)
		} else {
			m = CalcDefaultMeta(zid, contentExt)
		}
	} else {
		m, err = readZipMetaFile(reader, zid, metaName)
	}
	if err == nil {
		CleanupMeta(m, zid, entry.ContentExt, inMeta, entry.UselessFiles)
	}
	return m, err
}

func readZipMetaFile(reader *zip.ReadCloser, zid id.ZidO, name string) (*meta.Meta, error) {
	src, err := readZipFileContent(reader, name)
	if err != nil {
		return nil, err
	}
	inp := input.NewInput(src)
	return meta.NewFromInput(zid, inp), nil
}

func readZipFileContent(reader *zip.ReadCloser, name string) ([]byte, error) {
	f, err := reader.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
