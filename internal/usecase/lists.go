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

package usecase

import (
	"context"

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/webapi"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/query"
)

// -------- List syntax ------------------------------------------------------

// ListSyntaxPort is the interface used by this use case.
type ListSyntaxPort interface {
	SelectMeta(ctx context.Context, metaSeq []*meta.Meta, q *query.Query) ([]*meta.Meta, error)
}

// ListSyntax is the data for this use case.
type ListSyntax struct {
	port ListSyntaxPort
}

// NewListSyntax creates a new use case.
func NewListSyntax(port ListSyntaxPort) ListSyntax {
	return ListSyntax{port: port}
}

// Run executes the use case.
func (uc ListSyntax) Run(ctx context.Context) (meta.Arrangement, error) {
	q := query.Parse(meta.KeySyntax + webapi.ExistOperator) // We look for all metadata with a syntax key
	metas, err := uc.port.SelectMeta(box.NoEnrichContext(ctx), nil, q)
	if err != nil {
		return nil, err
	}
	result := meta.CreateArrangement(metas, meta.KeySyntax)
	for _, syn := range parser.GetSyntaxes() {
		if _, found := result[syn]; !found {
			delete(result, syn)
		}
	}
	return result, nil
}

// -------- List roles -------------------------------------------------------

// ListRolesPort is the interface used by this use case.
type ListRolesPort interface {
	SelectMeta(ctx context.Context, metaSeq []*meta.Meta, q *query.Query) ([]*meta.Meta, error)
}

// ListRoles is the data for this use case.
type ListRoles struct {
	port ListRolesPort
}

// NewListRoles creates a new use case.
func NewListRoles(port ListRolesPort) ListRoles {
	return ListRoles{port: port}
}

// Run executes the use case.
func (uc ListRoles) Run(ctx context.Context) (meta.Arrangement, error) {
	q := query.Parse(meta.KeyRole + webapi.ExistOperator) // We look for all metadata with an existing role key
	metas, err := uc.port.SelectMeta(box.NoEnrichContext(ctx), nil, q)
	if err != nil {
		return nil, err
	}
	return meta.CreateArrangement(metas, meta.KeyRole), nil
}
