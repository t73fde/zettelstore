//-----------------------------------------------------------------------------
// Copyright (c) 2023-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2023-present Detlef Stern
//-----------------------------------------------------------------------------

package usecase

import (
	"context"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/meta"
)

// TagZettel is the usecase of retrieving a "tag zettel", i.e. a zettel that
// describes a given tag. A tag zettel must have the tag's name in its title
// and must have a role=tag.

// TagZettelPort is the interface used by this use case.
type TagZettelPort interface {
	// GetZettel retrieves a specific zettel.
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)
}

// TagZettel is the data for this use case.
type TagZettel struct {
	port  GetZettelPort
	query *Query
}

// NewTagZettel creates a new use case.
func NewTagZettel(port GetZettelPort, query *Query) TagZettel {
	return TagZettel{port: port, query: query}
}

// Run executes the use case.
func (uc TagZettel) Run(ctx context.Context, tag string) (zettel.Zettel, error) {
	tag = meta.NormalizeTag(tag)
	q := query.Parse(
		api.KeyTitle + api.SearchOperatorEqual + tag + " " +
			api.KeyRole + api.SearchOperatorHas + api.ValueRoleTag)
	ml, err := uc.query.Run(ctx, q)
	if err != nil {
		return zettel.Zettel{}, err
	}
	for _, m := range ml {
		z, errZ := uc.port.GetZettel(ctx, m.Zid)
		if errZ == nil {
			return z, nil
		}
	}
	return zettel.Zettel{}, ErrTagZettelNotFound{Tag: tag}
}

// ErrTagZettelNotFound is returned if a tag zettel was not found.
type ErrTagZettelNotFound struct{ Tag string }

func (etznf ErrTagZettelNotFound) Error() string { return "tag zettel not found: " + etznf.Tag }

// RoleZettel is the usecase of retrieving a "role zettel", i.e. a zettel that
// describes a given role. A role zettel must have the role's name in its title
// and must have a role=role.

// RoleZettelPort is the interface used by this use case.
type RoleZettelPort interface {
	// GetZettel retrieves a specific zettel.
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)
}

// RoleZettel is the data for this use case.
type RoleZettel struct {
	port  GetZettelPort
	query *Query
}

// NewRoleZettel creates a new use case.
func NewRoleZettel(port GetZettelPort, query *Query) RoleZettel {
	return RoleZettel{port: port, query: query}
}

// Run executes the use case.
func (uc RoleZettel) Run(ctx context.Context, role string) (zettel.Zettel, error) {
	q := query.Parse(
		api.KeyTitle + api.SearchOperatorEqual + role + " " +
			api.KeyRole + api.SearchOperatorHas + api.ValueRoleRole)
	ml, err := uc.query.Run(ctx, q)
	if err != nil {
		return zettel.Zettel{}, err
	}
	for _, m := range ml {
		z, errZ := uc.port.GetZettel(ctx, m.Zid)
		if errZ == nil {
			return z, nil
		}
	}
	return zettel.Zettel{}, ErrRoleZettelNotFound{Role: role}
}

// ErrRoleZettelNotFound is returned if a role zettel was not found.
type ErrRoleZettelNotFound struct{ Role string }

func (etznf ErrRoleZettelNotFound) Error() string { return "role zettel not found: " + etznf.Role }
