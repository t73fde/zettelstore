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

package policy

import (
	"context"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/query"
	"zettelstore.de/z/internal/web/server"
	"zettelstore.de/z/internal/zettel"
)

// BoxWithPolicy wraps the given box inside a policy box.
func BoxWithPolicy(manager auth.AuthzManager, box box.Box, authConfig config.AuthConfig) (box.Box, auth.Policy) {
	pol := newPolicy(manager, authConfig)
	return newBox(box, pol), pol
}

// polBox implements a policy box.
type polBox struct {
	box    box.Box
	policy auth.Policy
}

// newBox creates a new policy box.
func newBox(box box.Box, policy auth.Policy) box.Box {
	return &polBox{
		box:    box,
		policy: policy,
	}
}

func (pp *polBox) Location() string {
	return pp.box.Location()
}

func (pp *polBox) CanCreateZettel(ctx context.Context) bool {
	return pp.box.CanCreateZettel(ctx)
}

func (pp *polBox) CreateZettel(ctx context.Context, zettel zettel.Zettel) (id.Zid, error) {
	user := server.GetCurrentUser(ctx)
	if pp.policy.CanCreate(user, zettel.Meta) {
		return pp.box.CreateZettel(ctx, zettel)
	}
	return id.Invalid, box.NewErrNotAllowed("Create", user, id.Invalid)
}

func (pp *polBox) GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error) {
	z, err := pp.box.GetZettel(ctx, zid)
	if err != nil {
		return zettel.Zettel{}, err
	}
	user := server.GetCurrentUser(ctx)
	if pp.policy.CanRead(user, z.Meta) {
		return z, nil
	}
	return zettel.Zettel{}, box.NewErrNotAllowed("GetZettel", user, zid)
}

func (pp *polBox) GetAllZettel(ctx context.Context, zid id.Zid) ([]zettel.Zettel, error) {
	return pp.box.GetAllZettel(ctx, zid)
}

func (pp *polBox) FetchZids(ctx context.Context) (*idset.Set, error) {
	return nil, box.NewErrNotAllowed("fetch-zids", server.GetCurrentUser(ctx), id.Invalid)
}

func (pp *polBox) GetMeta(ctx context.Context, zid id.Zid) (*meta.Meta, error) {
	m, err := pp.box.GetMeta(ctx, zid)
	if err != nil {
		return nil, err
	}
	user := server.GetCurrentUser(ctx)
	if pp.policy.CanRead(user, m) {
		return m, nil
	}
	return nil, box.NewErrNotAllowed("GetMeta", user, zid)
}

func (pp *polBox) SelectMeta(ctx context.Context, metaSeq []*meta.Meta, q *query.Query) ([]*meta.Meta, error) {
	user := server.GetCurrentUser(ctx)
	canRead := pp.policy.CanRead
	q = q.SetPreMatch(func(m *meta.Meta) bool { return canRead(user, m) })
	return pp.box.SelectMeta(ctx, metaSeq, q)
}

func (pp *polBox) CanUpdateZettel(ctx context.Context, zettel zettel.Zettel) bool {
	return pp.box.CanUpdateZettel(ctx, zettel)
}

func (pp *polBox) UpdateZettel(ctx context.Context, zettel zettel.Zettel) error {
	zid := zettel.Meta.Zid
	user := server.GetCurrentUser(ctx)
	if !zid.IsValid() {
		return box.ErrInvalidZid{Zid: zid.String()}
	}
	// Write existing zettel
	oldZettel, err := pp.box.GetZettel(ctx, zid)
	if err != nil {
		return err
	}
	if pp.policy.CanWrite(user, oldZettel.Meta, zettel.Meta) {
		return pp.box.UpdateZettel(ctx, zettel)
	}
	return box.NewErrNotAllowed("Write", user, zid)
}

func (pp *polBox) CanDeleteZettel(ctx context.Context, zid id.Zid) bool {
	return pp.box.CanDeleteZettel(ctx, zid)
}

func (pp *polBox) DeleteZettel(ctx context.Context, zid id.Zid) error {
	z, err := pp.box.GetZettel(ctx, zid)
	if err != nil {
		return err
	}
	user := server.GetCurrentUser(ctx)
	if pp.policy.CanDelete(user, z.Meta) {
		return pp.box.DeleteZettel(ctx, zid)
	}
	return box.NewErrNotAllowed("Delete", user, zid)
}

func (pp *polBox) Refresh(ctx context.Context) error {
	user := server.GetCurrentUser(ctx)
	if pp.policy.CanRefresh(user) {
		return pp.box.Refresh(ctx)
	}
	return box.NewErrNotAllowed("Refresh", user, id.Invalid)
}
func (pp *polBox) ReIndex(ctx context.Context, zid id.Zid) error {
	user := server.GetCurrentUser(ctx)
	if pp.policy.CanRefresh(user) {
		// If a user is allowed to refresh all data, it it also allowed to re-index a zettel.
		return pp.box.ReIndex(ctx, zid)
	}
	return box.NewErrNotAllowed("ReIndex", user, zid)
}
