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

package manager

import (
	"context"
	"errors"
	"strings"

	"t73f.de/r/zero/roster"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/query"
)

// Contains all box.Box related functions

// Name returns the managers name.
func (*Manager) Name() string { return "" }

// Location returns some information where the box is located.
func (mgr *Manager) Location() string {
	if len(mgr.boxes) <= 2 {
		return "NONE"
	}
	var sb strings.Builder
	for i := range len(mgr.boxes) - 2 {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(mgr.boxes[i].Location())
	}
	return sb.String()
}

// CanCreateZettel returns true, if box could possibly create a new zettel.
func (mgr *Manager) CanCreateZettel(ctx context.Context) bool {
	if err := mgr.checkContinue(ctx); err != nil {
		return false
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	if CreateBox, isCreateBox := mgr.boxes[0].(box.CreateBox); isCreateBox {
		return CreateBox.CanCreateZettel(ctx)
	}
	return false
}

// CreateZettel creates a new zettel.
func (mgr *Manager) CreateZettel(ctx context.Context, ztl box.Zettel) (id.Zid, error) {
	mgr.mgrLogger.Debug("CreateZettel")
	if err := mgr.checkContinue(ctx); err != nil {
		return id.Invalid, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	if createBox, isCreateBox := mgr.boxes[0].(box.CreateBox); isCreateBox {
		ztl.Meta = mgr.cleanMetaProperties(ztl.Meta)
		zid, err := createBox.CreateZettel(ctx, ztl)
		if err != nil {
			mgr.enqueueAndWait(ctx, zid)
		}
		return zid, err
	}
	return id.Invalid, box.ErrReadOnly
}

// GetZettel retrieves a specific zettel.
func (mgr *Manager) GetZettel(ctx context.Context, zid id.Zid) (box.Zettel, error) {
	mgr.mgrLogger.Debug("GetZettel", "zid", zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return box.Zettel{}, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	return mgr.getZettel(ctx, zid)
}
func (mgr *Manager) getZettel(ctx context.Context, zid id.Zid) (box.Zettel, error) {
	for _, p := range mgr.boxes {
		z, err := p.GetZettel(ctx, zid)
		if _, isErr := errors.AsType[box.ErrZettelNotFound](err); !isErr {
			if err == nil {
				mgr.Enrich(ctx, z.Meta, p.Name())
			}
			return z, err
		}
	}
	return box.Zettel{}, box.ErrZettelNotFound{Zid: zid}
}

// GetAllZettel retrieves a specific zettel from all managed boxes.
func (mgr *Manager) GetAllZettel(ctx context.Context, zid id.Zid) ([]box.Zettel, error) {
	mgr.mgrLogger.Debug("GetAllZettel", "zid", zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return nil, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	var result []box.Zettel
	for _, p := range mgr.boxes {
		if z, err := p.GetZettel(ctx, zid); err == nil {
			mgr.Enrich(ctx, z.Meta, p.Name())
			result = append(result, z)
		}
	}
	return result, nil
}

// FetchZids returns the set of all zettel identifer managed by the box.
func (mgr *Manager) FetchZids(ctx context.Context) (map[id.Zid]struct{}, error) {
	mgr.mgrLogger.Debug("FetchZids")
	if err := mgr.checkContinue(ctx); err != nil {
		return nil, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	return mgr.fetchZids(ctx)
}
func (mgr *Manager) fetchZids(ctx context.Context) (map[id.Zid]struct{}, error) {
	numZettel := 0
	for _, p := range mgr.boxes {
		var mbstats box.ManagedBoxStats
		p.ReadStats(&mbstats)
		numZettel += mbstats.Zettel
	}
	result := make(map[id.Zid]struct{}, numZettel)
	for _, p := range mgr.boxes {
		err := p.ApplyZid(ctx, func(zid id.Zid) { result[zid] = struct{}{} }, query.AlwaysIncluded)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// FindMeta returns metadata of the given zettel.
// It returns nil if the zettel is not found.
func (mgr *Manager) FindMeta(ctx context.Context, zid id.Zid) *meta.Meta {
	mgr.mgrLogger.Debug("FindMeta", "zid", zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return nil
	}
	m := mgr.index.FindMeta(zid)
	if m != nil {
		mgr.Enrich(ctx, m, "")
	}
	// TODO: Call GetZettel and return just metadata, in case the index is not complete.
	return m
}

// SelectMeta returns the metadata of the zettel that match the query q.
//
// If metaList is nil, all zettel of the manager's boxes are candidates.
// Otherwise, only the zettel of metaList are candidates; an empty, non-nil
// metaList therefore selects nothing. The slice metaList may be modified by
// this call and must not be used afterwards.
//
// The query may further restrict the result (pick, offset, limit), so the
// result does not necessarily contain all matching zettel.
//
// The order of the result is defined by the order specification of q. Without
// such a specification, the order depends on the candidates: if metaList is
// nil, the result is ordered by descending zettel id; otherwise, the relative
// order of the elements of metaList is kept.
//
// If an error occurs, e.g. because ctx is done or a box fails, no metadata is
// returned. The returned metadata is created anew for each call and may be
// modified by the caller.
func (mgr *Manager) SelectMeta(
	ctx context.Context,
	metaList []*meta.Meta,
	q *box.Query,
) ([]*meta.Meta, error) {
	mgr.mgrLogger.Debug("SelectMeta", "query", q)
	if err := mgr.checkContinue(ctx); err != nil {
		return nil, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()

	compSearch := q.RetrieveAndCompile(ctx, mgr.index, metaList)
	if result := compSearch.Result(); result != nil {
		logging.LogTrace(mgr.mgrLogger, "found without ApplyMeta", "count", len(result))
		return result, nil
	}
	selected := map[id.Zid]*meta.Meta{}
	for _, term := range compSearch.Terms {
		var rejected roster.Roster[id.Zid]
		handleMeta := func(m *meta.Meta) {
			zid := m.Zid
			if rejected.Contains(zid) {
				logging.LogTrace(mgr.mgrLogger, "SelectMeta/alreadyRejected", "zid", zid)
				return
			}
			if _, ok := selected[zid]; ok {
				logging.LogTrace(mgr.mgrLogger, "SelectMeta/alreadySelected", "zid", zid)
				return
			}
			if compSearch.PreMatch(m) && term.Match(m) {
				selected[zid] = m
				logging.LogTrace(mgr.mgrLogger, "SelectMeta/match", "zid", zid)
			} else {
				rejected.Insert(zid)
				logging.LogTrace(mgr.mgrLogger, "SelectMeta/reject", "zid", zid)
			}
		}
		for _, p := range mgr.boxes {
			if err2 := p.ApplyMeta(ctx, handleMeta, term.Retrieve); err2 != nil {
				return nil, err2
			}
		}
	}
	result := make([]*meta.Meta, 0, len(selected))
	for _, m := range selected {
		result = append(result, m)
	}
	result = compSearch.AfterSearch(result)
	logging.LogTrace(mgr.mgrLogger, "found with ApplyMeta", "count", len(result))
	return result, nil
}

// CanUpdateZettel returns true, if box could possibly update the given zettel.
func (mgr *Manager) CanUpdateZettel(ctx context.Context, zettel box.Zettel) bool {
	if err := mgr.checkContinue(ctx); err != nil {
		return false
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	if updateBox, isUpdateBox := mgr.boxes[0].(box.UpdateBox); isUpdateBox {
		return updateBox.CanUpdateZettel(ctx, zettel)
	}
	return false

}

// UpdateZettel updates an existing zettel.
func (mgr *Manager) UpdateZettel(ctx context.Context, zettel box.Zettel) error {
	mgr.mgrLogger.Debug("UpdateZettel", "zid", zettel.Meta.Zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return err
	}
	return mgr.updateZettel(ctx, zettel)
}
func (mgr *Manager) updateZettel(ctx context.Context, zettel box.Zettel) error {
	if updateBox, isUpdateBox := mgr.boxes[0].(box.UpdateBox); isUpdateBox {
		zettel.Meta = mgr.cleanMetaProperties(zettel.Meta)
		if err := updateBox.UpdateZettel(ctx, zettel); err != nil {
			return err
		}
		mgr.enqueueAndWait(ctx, zettel.Meta.Zid)
		return nil
	}
	return box.ErrReadOnly
}

// CanDeleteZettel returns true, if box could possibly delete the given zettel.
func (mgr *Manager) CanDeleteZettel(ctx context.Context, zid id.Zid) bool {
	if err := mgr.checkContinue(ctx); err != nil {
		return false
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	for _, p := range mgr.boxes {
		if deleteBox, isDeleteBox := p.(box.DeleteBox); isDeleteBox && deleteBox.CanDeleteZettel(ctx, zid) {
			return true
		}
	}
	return false
}

// DeleteZettel removes the zettel from the box.
func (mgr *Manager) DeleteZettel(ctx context.Context, zid id.Zid) error {
	mgr.mgrLogger.Debug("DeleteZettel", "zid", zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	for _, p := range mgr.boxes {
		if deleteBox, isDeleteBox := p.(box.DeleteBox); isDeleteBox {
			err := deleteBox.DeleteZettel(ctx, zid)
			if err == nil {
				mgr.enqueueAndWait(ctx, zid)
				return nil
			}
			if _, isErr := errors.AsType[box.ErrZettelNotFound](err); !isErr && !errors.Is(err, box.ErrReadOnly) {
				return err
			}
		}
	}
	return box.ErrZettelNotFound{Zid: zid}
}

// Remove all (computed) properties from metadata before storing the zettel.
func (mgr *Manager) cleanMetaProperties(m *meta.Meta) *meta.Meta {
	result := m.Clone()
	for key := range result.RestAny() {
		if mgr.propertyKeys.Contains(key) {
			result.Delete(key)
		}
	}
	return result
}
