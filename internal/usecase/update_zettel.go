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
	"log/slog"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/zettel"
)

// UpdateZettelPort is the interface used by this use case.
type UpdateZettelPort interface {
	// GetZettel retrieves a specific zettel.
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)

	// UpdateZettel updates an existing zettel.
	UpdateZettel(ctx context.Context, zettel zettel.Zettel) error
}

// UpdateZettel is the data for this use case.
type UpdateZettel struct {
	logger *slog.Logger
	port   UpdateZettelPort
}

// NewUpdateZettel creates a new use case.
func NewUpdateZettel(logger *slog.Logger, port UpdateZettelPort) UpdateZettel {
	return UpdateZettel{logger: logger, port: port}
}

// Run executes the use case.
func (uc *UpdateZettel) Run(ctx context.Context, zettel zettel.Zettel, hasContent bool) error {
	m := zettel.Meta
	oldZettel, err := uc.port.GetZettel(box.NoEnrichContext(ctx), m.Zid)
	if err != nil {
		return err
	}
	if zettel.Equal(oldZettel, false) {
		return nil
	}

	// Update relevant computed, but stored values.
	if _, found := m.Get(meta.KeyCreated); !found {
		if val, crFound := oldZettel.Meta.Get(meta.KeyCreated); crFound {
			m.Set(meta.KeyCreated, val)
		}
	}
	m.SetNow(meta.KeyModified)

	m.YamlSep = oldZettel.Meta.YamlSep
	if m.Zid == id.ZidConfiguration {
		m.Set(meta.KeySyntax, meta.ValueSyntaxNone)
	}

	if !hasContent {
		zettel.Content = oldZettel.Content
	}
	zettel.Content.TrimSpace()
	err = uc.port.UpdateZettel(ctx, zettel)
	uc.logger.Info("Update zettel", "zid", m.Zid, logging.Err(err)) // TODO: add user=
	return err
}
