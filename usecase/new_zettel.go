//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// Zettelstore is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with Zettelstore. If not, see <http://www.gnu.org/licenses/>.
//-----------------------------------------------------------------------------

// Package usecase provides (business) use cases for the zettelstore.
package usecase

import (
	"context"

	"zettelstore.de/z/config"
	"zettelstore.de/z/domain"
)

// NewZettelPort is the interface used by this use case.
type NewZettelPort interface {
	// SetZettel updates an existing zettel or creates a new one.
	SetZettel(ctx context.Context, zettel domain.Zettel) error
}

// NewZettel is the data for this use case.
type NewZettel struct {
	store NewZettelPort
}

// NewNewZettel creates a new use case.
func NewNewZettel(port NewZettelPort) NewZettel {
	return NewZettel{store: port}
}

// Run executes the use case.
func (uc NewZettel) Run(ctx context.Context, zettel domain.Zettel) error {
	meta := zettel.Meta
	if meta.ID.IsValid() {
		return nil // TODO: new error: already exists
	}

	if title, ok := meta.Get(domain.MetaKeyTitle); !ok || title == "" {
		meta.Set(domain.MetaKeyTitle, config.Config.GetDefaultTitle())
	}
	if role, ok := meta.Get(domain.MetaKeyRole); !ok || role == "" {
		meta.Set(domain.MetaKeyRole, config.Config.GetDefaultRole())
	}
	if syntax, ok := meta.Get(domain.MetaKeySyntax); !ok || syntax == "" {
		meta.Set(domain.MetaKeySyntax, config.Config.GetDefaultSyntax())
	}
	meta.YamlSep = config.Config.GetYAMLHeader()

	return uc.store.SetZettel(ctx, zettel)
}
