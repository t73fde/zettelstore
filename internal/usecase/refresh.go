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

	"zettelstore.de/z/internal/logger"
)

// RefreshPort is the interface used by this use case.
type RefreshPort interface {
	Refresh(context.Context) error
}

// Refresh is the data for this use case.
type Refresh struct {
	dlog *logger.DLogger
	port RefreshPort
}

// NewRefresh creates a new use case.
func NewRefresh(dlog *logger.DLogger, port RefreshPort) Refresh {
	return Refresh{dlog: dlog, port: port}
}

// Run executes the use case.
func (uc *Refresh) Run(ctx context.Context) error {
	err := uc.port.Refresh(ctx)
	uc.dlog.Info().User(ctx).Err(err).Msg("Refresh internal data")
	return err
}
