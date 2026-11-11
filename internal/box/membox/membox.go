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

// Package membox stores zettel volatile in main memory.
package membox

import (
	"context"
	"log/slog"
	"net/url"
	"sync"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/box/manager"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logging"
)

func init() {
	manager.Register(
		box.SchemeMemoryBox,
		func(u *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
			name := u.Query().Get(manager.QueryName)
			return &memBox{
				logger: kernel.Main.GetLogger(kernel.BoxService).With(
					"box", box.SchemeMemoryBox, "name", name),
				cdata:     *cdata,
				name:      name,
				location:  u.String(),
				maxZettel: box.GetQueryInt(u, "max-zettel", 0, 127, 65535),
				maxBytes:  box.GetQueryInt(u, "max-bytes", 0, 65535, (1024*1024*1024)-1),
				readonly:  box.GetQueryBool(u, manager.QueryReadOnly),
			}, nil
		})
}

type memBox struct {
	logger    *slog.Logger
	cdata     manager.ConnectData
	name      string
	location  string
	maxZettel int
	maxBytes  int
	mx        sync.RWMutex // Protects the following fields
	zettel    map[id.Zid]box.Zettel
	curBytes  int
	readonly  bool
}

func (mbx *memBox) notifyChanged(zid id.Zid, reason box.UpdateReason) {
	if notify := mbx.cdata.Notify; notify != nil {
		notify(mbx, zid, reason)
	}
}

func (mbx *memBox) Name() string     { return mbx.name }
func (mbx *memBox) Location() string { return mbx.location }

func (mbx *memBox) State() box.StartState {
	mbx.mx.RLock()
	defer mbx.mx.RUnlock()
	if mbx.zettel == nil {
		return box.StartStateStopped
	}
	return box.StartStateStarted
}

func (mbx *memBox) Start(context.Context) error {
	mbx.mx.Lock()
	mbx.zettel = make(map[id.Zid]box.Zettel)
	mbx.curBytes = 0
	mbx.mx.Unlock()
	logging.LogTrace(mbx.logger, "Start box", "max-zettel", mbx.maxZettel, "max-bytes", mbx.maxBytes)
	return nil
}

func (mbx *memBox) Stop(context.Context) {
	mbx.mx.Lock()
	mbx.zettel = nil
	mbx.mx.Unlock()
}

func (mbx *memBox) CanCreateZettel(context.Context) bool {
	mbx.mx.RLock()
	defer mbx.mx.RUnlock()
	return len(mbx.zettel) < mbx.maxZettel && !mbx.readonly
}

func (mbx *memBox) CreateZettel(_ context.Context, zettel box.Zettel) (id.Zid, error) {
	if mbx.readonly {
		return id.Invalid, box.ErrReadOnly
	}
	mbx.mx.Lock()
	newBytes := mbx.curBytes + zettel.ByteSize()
	if mbx.maxZettel <= len(mbx.zettel) || mbx.maxBytes < newBytes {
		mbx.mx.Unlock()
		return id.Invalid, box.ErrCapacity
	}
	zid, err := box.GetNewZid(func(zid id.Zid) (bool, error) {
		_, ok := mbx.zettel[zid]
		return !ok, nil
	})
	if err != nil {
		mbx.mx.Unlock()
		return id.Invalid, err
	}
	meta := zettel.Meta.Clone()
	meta.Zid = zid
	zettel.Meta = meta
	mbx.zettel[zid] = zettel
	mbx.curBytes = newBytes
	mbx.mx.Unlock()

	mbx.notifyChanged(zid, box.OnZettel)
	logging.LogTrace(mbx.logger, "CreateZettel", "zid", zid)
	return zid, nil
}

func (mbx *memBox) GetZettel(_ context.Context, zid id.Zid) (box.Zettel, error) {
	mbx.mx.RLock()
	z, ok := mbx.zettel[zid]
	mbx.mx.RUnlock()
	if !ok {
		return box.Zettel{}, box.ErrZettelNotFound{Zid: zid}
	}
	z.Meta = z.Meta.Clone()
	logging.LogTrace(mbx.logger, "GetZettel")
	return z, nil
}

func (mbx *memBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint box.RetrievePredicate) error {
	mbx.mx.RLock()
	defer mbx.mx.RUnlock()
	logging.LogTrace(mbx.logger, "ApplyZid", "entries", len(mbx.zettel))
	for zid := range mbx.zettel {
		if constraint(zid) {
			handle(zid)
		}
	}
	return nil
}

func (mbx *memBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint box.RetrievePredicate) error {
	mbx.mx.RLock()
	defer mbx.mx.RUnlock()
	logging.LogTrace(mbx.logger, "ApplyMeta", "entries", len(mbx.zettel))
	for zid, zettel := range mbx.zettel {
		if constraint(zid) {
			m := zettel.Meta.Clone()
			mbx.cdata.Enricher.Enrich(ctx, m, mbx.name)
			handle(m)
		}
	}
	return nil
}

func (mbx *memBox) CanUpdateZettel(_ context.Context, zettel box.Zettel) bool {
	mbx.mx.RLock()
	defer mbx.mx.RUnlock()
	zid := zettel.Meta.Zid
	if !zid.IsValid() {
		return false
	}

	newBytes := mbx.curBytes + zettel.ByteSize()
	if prevZettel, found := mbx.zettel[zid]; found {
		newBytes -= prevZettel.ByteSize()
	} else if len(mbx.zettel) >= mbx.maxZettel { // from other box && ...
		return false
	}
	return newBytes <= mbx.maxBytes && !mbx.readonly
}

func (mbx *memBox) UpdateZettel(_ context.Context, zettel box.Zettel) error {
	if mbx.readonly {
		return box.ErrReadOnly
	}
	m := zettel.Meta.Clone()
	if !m.Zid.IsValid() {
		return box.ErrInvalidZid{Zid: m.Zid.String()}
	}

	mbx.mx.Lock()
	newBytes := mbx.curBytes + zettel.ByteSize()
	if prevZettel, found := mbx.zettel[m.Zid]; found {
		newBytes -= prevZettel.ByteSize()
	} else if len(mbx.zettel) >= mbx.maxZettel { // from other box && ...
		mbx.mx.Unlock()
		return box.ErrCapacity
	}
	if mbx.maxBytes < newBytes {
		mbx.mx.Unlock()
		return box.ErrCapacity
	}

	zettel.Meta = m
	mbx.zettel[m.Zid] = zettel
	mbx.curBytes = newBytes
	mbx.mx.Unlock()
	mbx.notifyChanged(m.Zid, box.OnZettel)
	logging.LogTrace(mbx.logger, "UpdateZettel")
	return nil
}

func (mbx *memBox) CanDeleteZettel(_ context.Context, zid id.Zid) bool {
	mbx.mx.RLock()
	_, ok := mbx.zettel[zid]
	mbx.mx.RUnlock()
	return ok && !mbx.readonly
}

func (mbx *memBox) DeleteZettel(_ context.Context, zid id.Zid) error {
	if mbx.readonly {
		return box.ErrReadOnly
	}
	mbx.mx.Lock()
	oldZettel, found := mbx.zettel[zid]
	if !found {
		mbx.mx.Unlock()
		return box.ErrZettelNotFound{Zid: zid}
	}
	delete(mbx.zettel, zid)
	mbx.curBytes -= oldZettel.ByteSize()
	mbx.mx.Unlock()
	mbx.notifyChanged(zid, box.OnDelete)
	logging.LogTrace(mbx.logger, "DeleteZettel")
	return nil
}

func (mbx *memBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = mbx.readonly
	mbx.mx.RLock()
	st.Zettel = len(mbx.zettel)
	mbx.mx.RUnlock()
	logging.LogTrace(mbx.logger, "ReadStats", "zettel", st.Zettel)
}
