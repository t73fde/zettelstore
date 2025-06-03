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
	"log/slog"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/logging"
)

// ReIndexPort is the interface used by this use case.
type ReIndexPort interface {
	ReIndex(context.Context, id.Zid) error
}

// ReIndex is the data for this use case.
type ReIndex struct {
	logger *slog.Logger
	port   ReIndexPort
}

// NewReIndex creates a new use case.
func NewReIndex(logger *slog.Logger, port ReIndexPort) ReIndex {
	return ReIndex{logger: logger, port: port}
}

// Run executes the use case.
func (uc *ReIndex) Run(ctx context.Context, zid id.Zid) error {
	err := uc.port.ReIndex(ctx, zid)
	uc.logger.Info("ReIndex zettel", "zid", zid, logging.User(ctx), logging.Err(err))
	return err
}
