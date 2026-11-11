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
	"log/slog"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/box/notify"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/zettel"
)

type zipBox struct {
	logger   *slog.Logger
	name     string
	location string
	path     string
	enricher box.Enricher
	notify   box.UpdateNotifier
	dirSrv   *notify.DirService
}

func (zbx *zipBox) Name() string     { return zbx.name }
func (zbx *zipBox) Location() string { return zbx.location }

func (zbx *zipBox) State() box.StartState {
	if ds := zbx.dirSrv; ds != nil {
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

func (zbx *zipBox) Start(context.Context) error {
	reader, err := zip.OpenReader(zbx.path)
	if err != nil {
		return err
	}
	if err = reader.Close(); err != nil {
		return err
	}
	zipNotifier := notify.NewSimpleZipNotifier(zbx.logger, zbx.path)
	zbx.dirSrv = notify.NewDirService(zbx, zbx.logger, zipNotifier, zbx.notify)
	zbx.dirSrv.Start()
	return nil
}

func (zbx *zipBox) Refresh(context.Context) {
	zbx.dirSrv.Refresh()
	logging.LogTrace(zbx.logger, "Refresh")
}

func (zbx *zipBox) Stop(context.Context) {
	zbx.dirSrv.Stop()
	zbx.dirSrv = nil
}

func (zbx *zipBox) GetZettel(_ context.Context, zid id.Zid) (box.Zettel, error) {
	entry := zbx.dirSrv.GetDirEntry(zid)
	if !entry.IsValid() {
		return box.Zettel{}, box.ErrZettelNotFound{Zid: zid}
	}
	reader, err := zip.OpenReader(zbx.path)
	if err != nil {
		return box.Zettel{}, err
	}
	defer func() { _ = reader.Close() }()

	var m *meta.Meta
	var src []byte
	var inMeta bool

	contentName := entry.ContentName
	if metaName := entry.MetaName; metaName == "" {
		if contentName == "" {
			err = fmt.Errorf("no meta, no content in getZettel, zid=%v", zid)
			return box.Zettel{}, err
		}
		src, err = readZipFileContent(reader, entry.ContentName)
		if err != nil {
			return box.Zettel{}, err
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
			return box.Zettel{}, err
		}
		inMeta = true
		if contentName != "" {
			src, err = readZipFileContent(reader, entry.ContentName)
			if err != nil {
				return box.Zettel{}, err
			}
		}
	}

	CleanupMeta(m, entry.ContentExt, inMeta, entry.UselessFiles)
	logging.LogTrace(zbx.logger, "GetZettel", "zid", zid)
	return box.Zettel{Meta: m, Content: zettel.NewContent(src)}, nil
}

func (zbx *zipBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint box.RetrievePredicate) error {
	entries := zbx.dirSrv.GetDirEntries(constraint)
	logging.LogTrace(zbx.logger, "ApplyZid", "entries", len(entries))
	for _, entry := range entries {
		handle(entry.Zid)
	}
	return nil
}

func (zbx *zipBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint box.RetrievePredicate) error {
	reader, err := zip.OpenReader(zbx.path)
	if err != nil {
		return err
	}
	entries := zbx.dirSrv.GetDirEntries(constraint)
	logging.LogTrace(zbx.logger, "ApplyMeta", "entries", len(entries))
	for _, entry := range entries {
		if !constraint(entry.Zid) {
			continue
		}
		m, err2 := zbx.readZipMeta(reader, entry.Zid, entry)
		if err2 != nil {
			continue
		}
		zbx.enricher.Enrich(ctx, m, zbx.name)
		handle(m)
	}
	return reader.Close()
}

func (zbx *zipBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = true
	st.Zettel = zbx.dirSrv.NumDirEntries()
	logging.LogTrace(zbx.logger, "ReadStats", "zettel", st.Zettel)
}

func (zbx *zipBox) readZipMeta(reader *zip.ReadCloser, zid id.Zid, entry *notify.DirEntry) (m *meta.Meta, err error) {
	var inMeta bool
	if metaName := entry.MetaName; metaName == "" {
		contentName := entry.ContentName
		contentExt := entry.ContentExt
		switch {
		case contentName == "", contentExt == "":
			err = fmt.Errorf("no meta, no content in getMeta, zid=%v", zid)
		case entry.HasMetaInContent():
			m, err = readZipMetaFile(reader, zid, contentName)
		default:
			m = CalcDefaultMeta(zid, contentExt)
		}
	} else {
		m, err = readZipMetaFile(reader, zid, metaName)
	}
	if err == nil {
		CleanupMeta(m, entry.ContentExt, inMeta, entry.UselessFiles)
	}
	return m, err
}

func readZipMetaFile(reader *zip.ReadCloser, zid id.Zid, name string) (*meta.Meta, error) {
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
	data, err := io.ReadAll(f)
	err2 := f.Close()
	if err == nil {
		err = err2
	}
	return data, err
}
