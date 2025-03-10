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

	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/internal/zettel"
)

// GetZettelPort is the interface used by this use case.
type GetZettelPort interface {
	// GetZettel retrieves a specific zettel.
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)
}

// GetZettel is the data for this use case.
type GetZettel struct {
	port GetZettelPort
}

// NewGetZettel creates a new use case.
func NewGetZettel(port GetZettelPort) GetZettel {
	return GetZettel{port: port}
}

// Run executes the use case.
func (uc GetZettel) Run(ctx context.Context, zid id.Zid) (zettel.Zettel, error) {
	return uc.port.GetZettel(ctx, zid)
}
