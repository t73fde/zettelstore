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
	"zettelstore.de/z/logger"
)

// DeleteZettelPort is the interface used by this use case.
type DeleteZettelPort interface {
	// DeleteZettel removes the zettel from the box.
	DeleteZettel(ctx context.Context, zid id.Zid) error
}

// DeleteZettel is the data for this use case.
type DeleteZettel struct {
	log  *logger.Logger
	port DeleteZettelPort
}

// NewDeleteZettel creates a new use case.
func NewDeleteZettel(log *logger.Logger, port DeleteZettelPort) DeleteZettel {
	return DeleteZettel{log: log, port: port}
}

// Run executes the use case.
func (uc *DeleteZettel) Run(ctx context.Context, zid id.Zid) error {
	err := uc.port.DeleteZettel(ctx, zid)
	uc.log.Info().User(ctx).Zid(zid).Err(err).Msg("Delete zettel")
	return err
}
