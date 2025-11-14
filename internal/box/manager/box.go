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

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/zettel"
)

// Conatains all box.Box related functions

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
func (mgr *Manager) CreateZettel(ctx context.Context, ztl zettel.Zettel) (id.Zid, error) {
	mgr.mgrLogger.Debug("CreateZettel")
	if err := mgr.checkContinue(ctx); err != nil {
		return id.Invalid, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	if createBox, isCreateBox := mgr.boxes[0].(box.CreateBox); isCreateBox {
		ztl.Meta = mgr.cleanMetaProperties(ztl.Meta)
		zid, err := createBox.CreateZettel(ctx, ztl)
		if err == nil {
			mgr.idxUpdateZettel(ctx, ztl)
		}
		return zid, err
	}
	return id.Invalid, box.ErrReadOnly
}

// GetZettel retrieves a specific zettel.
func (mgr *Manager) GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error) {
	mgr.mgrLogger.Debug("GetZettel", "zid", zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return zettel.Zettel{}, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	return mgr.getZettel(ctx, zid)
}
func (mgr *Manager) getZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error) {
	for i, p := range mgr.boxes {
		var errZNF box.ErrZettelNotFound
		if z, err := p.GetZettel(ctx, zid); !errors.As(err, &errZNF) {
			if err == nil {
				mgr.Enrich(ctx, z.Meta, i+1)
			}
			return z, err
		}
	}
	return zettel.Zettel{}, box.ErrZettelNotFound{Zid: zid}
}

// GetAllZettel retrieves a specific zettel from all managed boxes.
func (mgr *Manager) GetAllZettel(ctx context.Context, zid id.Zid) ([]zettel.Zettel, error) {
	mgr.mgrLogger.Debug("GetAllZettel", "zid", zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return nil, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	var result []zettel.Zettel
	for i, p := range mgr.boxes {
		if z, err := p.GetZettel(ctx, zid); err == nil {
			mgr.Enrich(ctx, z.Meta, i+1)
			result = append(result, z)
		}
	}
	return result, nil
}

// FetchZids returns the set of all zettel identifer managed by the box.
func (mgr *Manager) FetchZids(ctx context.Context) (*idset.Set, error) {
	mgr.mgrLogger.Debug("FetchZids")
	if err := mgr.checkContinue(ctx); err != nil {
		return nil, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	return mgr.fetchZids(ctx)
}
func (mgr *Manager) fetchZids(ctx context.Context) (*idset.Set, error) {
	numZettel := 0
	for _, p := range mgr.boxes {
		var mbstats box.ManagedBoxStats
		p.ReadStats(&mbstats)
		numZettel += mbstats.Zettel
	}
	result := idset.NewCap(numZettel)
	for _, p := range mgr.boxes {
		err := p.ApplyZid(ctx, func(zid id.Zid) { result.Add(zid) }, query.AlwaysIncluded)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (mgr *Manager) hasZettel(ctx context.Context, zid id.Zid) bool {
	mgr.mgrLogger.Debug("HasZettel", "zid", zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return false
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	for _, bx := range mgr.boxes {
		if bx.HasZettel(ctx, zid) {
			return true
		}
	}
	return false
}

// GetMeta returns just the metadata of the zettel with the given identifier.
func (mgr *Manager) GetMeta(ctx context.Context, zid id.Zid) (*meta.Meta, error) {
	mgr.mgrLogger.Debug("GetMeta", "zid", zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return nil, err
	}

	m, err := mgr.idxStore.GetMeta(ctx, zid)
	if err != nil {
		// TODO: Call GetZettel and return just metadata, in case the index is not complete.
		return nil, err
	}
	mgr.Enrich(ctx, m, 0)
	return m, nil
}

// SelectMeta returns all zettel meta data that match the selection
// criteria. The result is ordered by descending zettel id.
func (mgr *Manager) SelectMeta(ctx context.Context, metaSeq []*meta.Meta, q *query.Query) ([]*meta.Meta, error) {
	mgr.mgrLogger.Debug("SelectMeta", "query", q)
	if err := mgr.checkContinue(ctx); err != nil {
		return nil, err
	}
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()

	compSearch := q.RetrieveAndCompile(ctx, mgr, metaSeq)
	if result := compSearch.Result(); result != nil {
		logging.LogTrace(mgr.mgrLogger, "found without ApplyMeta", "count", len(result))
		return result, nil
	}
	selected := map[id.Zid]*meta.Meta{}
	for _, term := range compSearch.Terms {
		rejected := idset.New()
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
				rejected.Add(zid)
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
func (mgr *Manager) CanUpdateZettel(ctx context.Context, zettel zettel.Zettel) bool {
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
func (mgr *Manager) UpdateZettel(ctx context.Context, zettel zettel.Zettel) error {
	mgr.mgrLogger.Debug("UpdateZettel", "zid", zettel.Meta.Zid)
	if err := mgr.checkContinue(ctx); err != nil {
		return err
	}
	return mgr.updateZettel(ctx, zettel)
}
func (mgr *Manager) updateZettel(ctx context.Context, zettel zettel.Zettel) error {
	if updateBox, isUpdateBox := mgr.boxes[0].(box.UpdateBox); isUpdateBox {
		zettel.Meta = mgr.cleanMetaProperties(zettel.Meta)
		if err := updateBox.UpdateZettel(ctx, zettel); err != nil {
			return err
		}
		mgr.idxUpdateZettel(ctx, zettel)
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
				mgr.idxDeleteZettel(ctx, zid)
				return err
			}
			var errZNF box.ErrZettelNotFound
			if !errors.As(err, &errZNF) && !errors.Is(err, box.ErrReadOnly) {
				return err
			}
		}
	}
	return box.ErrZettelNotFound{Zid: zid}
}

// Remove all (computed) properties from metadata before storing the zettel.
func (mgr *Manager) cleanMetaProperties(m *meta.Meta) *meta.Meta {
	result := m.Clone()
	for key := range result.ComputedRest() {
		if mgr.propertyKeys.Contains(key) {
			result.Delete(key)
		}
	}
	return result
}
