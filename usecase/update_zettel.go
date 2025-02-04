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

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/box"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/zettel"
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
	log  *logger.Logger
	port UpdateZettelPort
}

// NewUpdateZettel creates a new use case.
func NewUpdateZettel(log *logger.Logger, port UpdateZettelPort) UpdateZettel {
	return UpdateZettel{log: log, port: port}
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
	if _, found := m.Get(api.KeyCreated); !found {
		if val, crFound := oldZettel.Meta.Get(api.KeyCreated); crFound {
			m.Set(api.KeyCreated, val)
		}
	}
	m.SetNow(api.KeyModified)

	m.YamlSep = oldZettel.Meta.YamlSep
	if m.Zid == api.ZidConfiguration {
		m.Set(api.KeySyntax, meta.SyntaxNone)
	}

	if !hasContent {
		zettel.Content = oldZettel.Content
	}
	zettel.Content.TrimSpace()
	err = uc.port.UpdateZettel(ctx, zettel)
	uc.log.Info().User(ctx).Zid(m.Zid).Err(err).Msg("Update zettel")
	return err
}
