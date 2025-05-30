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
	"net/url"
	"sync"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/box/manager"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logger"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/zettel"
)

func init() {
	manager.Register(
		"mem",
		func(u *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
			return &memBox{
				log: kernel.Main.GetLogger(kernel.BoxService).Clone().
					Str("box", "mem").Int("boxnum", int64(cdata.Number)).Child(),
				u:         u,
				cdata:     *cdata,
				maxZettel: box.GetQueryInt(u, "max-zettel", 0, 127, 65535),
				maxBytes:  box.GetQueryInt(u, "max-bytes", 0, 65535, (1024*1024*1024)-1),
			}, nil
		})
}

type memBox struct {
	log       *logger.DLogger
	u         *url.URL
	cdata     manager.ConnectData
	maxZettel int
	maxBytes  int
	mx        sync.RWMutex // Protects the following fields
	zettel    map[id.Zid]zettel.Zettel
	curBytes  int
}

func (mb *memBox) notifyChanged(zid id.Zid, reason box.UpdateReason) {
	if notify := mb.cdata.Notify; notify != nil {
		notify(mb, zid, reason)
	}
}

func (mb *memBox) Location() string {
	return mb.u.String()
}

func (mb *memBox) State() box.StartState {
	mb.mx.RLock()
	defer mb.mx.RUnlock()
	if mb.zettel == nil {
		return box.StartStateStopped
	}
	return box.StartStateStarted
}

func (mb *memBox) Start(context.Context) error {
	mb.mx.Lock()
	mb.zettel = make(map[id.Zid]zettel.Zettel)
	mb.curBytes = 0
	mb.mx.Unlock()
	mb.log.Trace().Int("max-zettel", int64(mb.maxZettel)).Int("max-bytes", int64(mb.maxBytes)).Msg("Start Box")
	return nil
}

func (mb *memBox) Stop(context.Context) {
	mb.mx.Lock()
	mb.zettel = nil
	mb.mx.Unlock()
}

func (mb *memBox) CanCreateZettel(context.Context) bool {
	mb.mx.RLock()
	defer mb.mx.RUnlock()
	return len(mb.zettel) < mb.maxZettel
}

func (mb *memBox) CreateZettel(_ context.Context, zettel zettel.Zettel) (id.Zid, error) {
	mb.mx.Lock()
	newBytes := mb.curBytes + zettel.ByteSize()
	if mb.maxZettel < len(mb.zettel) || mb.maxBytes < newBytes {
		mb.mx.Unlock()
		return id.Invalid, box.ErrCapacity
	}
	zid, err := box.GetNewZid(func(zid id.Zid) (bool, error) {
		_, ok := mb.zettel[zid]
		return !ok, nil
	})
	if err != nil {
		mb.mx.Unlock()
		return id.Invalid, err
	}
	meta := zettel.Meta.Clone()
	meta.Zid = zid
	zettel.Meta = meta
	mb.zettel[zid] = zettel
	mb.curBytes = newBytes
	mb.mx.Unlock()

	mb.notifyChanged(zid, box.OnZettel)
	mb.log.Trace().Zid(zid).Msg("CreateZettel")
	return zid, nil
}

func (mb *memBox) GetZettel(_ context.Context, zid id.Zid) (zettel.Zettel, error) {
	mb.mx.RLock()
	z, ok := mb.zettel[zid]
	mb.mx.RUnlock()
	if !ok {
		return zettel.Zettel{}, box.ErrZettelNotFound{Zid: zid}
	}
	z.Meta = z.Meta.Clone()
	mb.log.Trace().Msg("GetZettel")
	return z, nil
}

func (mb *memBox) HasZettel(_ context.Context, zid id.Zid) bool {
	mb.mx.RLock()
	_, found := mb.zettel[zid]
	mb.mx.RUnlock()
	return found
}

func (mb *memBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint query.RetrievePredicate) error {
	mb.mx.RLock()
	defer mb.mx.RUnlock()
	mb.log.Trace().Int("entries", int64(len(mb.zettel))).Msg("ApplyZid")
	for zid := range mb.zettel {
		if constraint(zid) {
			handle(zid)
		}
	}
	return nil
}

func (mb *memBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint query.RetrievePredicate) error {
	mb.mx.RLock()
	defer mb.mx.RUnlock()
	mb.log.Trace().Int("entries", int64(len(mb.zettel))).Msg("ApplyMeta")
	for zid, zettel := range mb.zettel {
		if constraint(zid) {
			m := zettel.Meta.Clone()
			mb.cdata.Enricher.Enrich(ctx, m, mb.cdata.Number)
			handle(m)
		}
	}
	return nil
}

func (mb *memBox) CanUpdateZettel(_ context.Context, zettel zettel.Zettel) bool {
	mb.mx.RLock()
	defer mb.mx.RUnlock()
	zid := zettel.Meta.Zid
	if !zid.IsValid() {
		return false
	}

	newBytes := mb.curBytes + zettel.ByteSize()
	if prevZettel, found := mb.zettel[zid]; found {
		newBytes -= prevZettel.ByteSize()
	}
	return newBytes < mb.maxBytes
}

func (mb *memBox) UpdateZettel(_ context.Context, zettel zettel.Zettel) error {
	m := zettel.Meta.Clone()
	if !m.Zid.IsValid() {
		return box.ErrInvalidZid{Zid: m.Zid.String()}
	}

	mb.mx.Lock()
	newBytes := mb.curBytes + zettel.ByteSize()
	if prevZettel, found := mb.zettel[m.Zid]; found {
		newBytes -= prevZettel.ByteSize()
	}
	if mb.maxBytes < newBytes {
		mb.mx.Unlock()
		return box.ErrCapacity
	}

	zettel.Meta = m
	mb.zettel[m.Zid] = zettel
	mb.curBytes = newBytes
	mb.mx.Unlock()
	mb.notifyChanged(m.Zid, box.OnZettel)
	mb.log.Trace().Msg("UpdateZettel")
	return nil
}

func (mb *memBox) CanDeleteZettel(_ context.Context, zid id.Zid) bool {
	mb.mx.RLock()
	_, ok := mb.zettel[zid]
	mb.mx.RUnlock()
	return ok
}

func (mb *memBox) DeleteZettel(_ context.Context, zid id.Zid) error {
	mb.mx.Lock()
	oldZettel, found := mb.zettel[zid]
	if !found {
		mb.mx.Unlock()
		return box.ErrZettelNotFound{Zid: zid}
	}
	delete(mb.zettel, zid)
	mb.curBytes -= oldZettel.ByteSize()
	mb.mx.Unlock()
	mb.notifyChanged(zid, box.OnDelete)
	mb.log.Trace().Msg("DeleteZettel")
	return nil
}

func (mb *memBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = false
	mb.mx.RLock()
	st.Zettel = len(mb.zettel)
	mb.mx.RUnlock()
	mb.log.Trace().Int("zettel", int64(st.Zettel)).Msg("ReadStats")
}
