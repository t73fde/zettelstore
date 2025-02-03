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

package usecase

import (
	"context"

	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/zettel"
)

// GetAllZettelPort is the interface used by this use case.
type GetAllZettelPort interface {
	GetAllZettel(ctx context.Context, zid id.Zid) ([]zettel.Zettel, error)
}

// GetAllZettel is the data for this use case.
type GetAllZettel struct {
	port GetAllZettelPort
}

// NewGetAllZettel creates a new use case.
func NewGetAllZettel(port GetAllZettelPort) GetAllZettel {
	return GetAllZettel{port: port}
}

// Run executes the use case.
func (uc GetAllZettel) Run(ctx context.Context, zid id.Zid) ([]zettel.Zettel, error) {
	return uc.port.GetAllZettel(ctx, zid)
}
