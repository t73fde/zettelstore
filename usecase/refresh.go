//-----------------------------------------------------------------------------
// Copyright (c) 2021 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package usecase

import (
	"context"

	"zettelstore.de/z/logger"
)

// RefreshPort is the interface used by this use case.
type RefreshPort interface {
	Refresh(context.Context) error
}

// Refresh is the data for this use case.
type Refresh struct {
	log  *logger.Logger
	port RefreshPort
}

// NewRefresh creates a new use case.
func NewRefresh(log *logger.Logger, port RefreshPort) Refresh {
	return Refresh{log: log, port: port}
}

// Run executes the use case.
func (uc *Refresh) Run(ctx context.Context) error {
	err := uc.port.Refresh(ctx)
	uc.log.Info().User(ctx).Err(err).Msg("Refresh internal data")
	return err
}
