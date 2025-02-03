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

// Package store contains general index data for storing a zettel index.
package store

import (
	"context"
	"io"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel/meta"
)

// Stats records statistics about the store.
type Stats struct {
	// Zettel is the number of zettel managed by the indexer.
	Zettel int

	// Updates count the number of metadata updates.
	Updates uint64

	// Words count the different words stored in the store.
	Words uint64

	// Urls count the different URLs stored in the store.
	Urls uint64
}

// Store all relevant zettel data. There may be multiple implementations, i.e.
// memory-based, file-based, based on SQLite, ...
type Store interface {
	query.Searcher

	// GetMeta returns the metadata of the zettel with the given identifier.
	GetMeta(context.Context, id.Zid) (*meta.Meta, error)

	// Entrich metadata with data from store.
	Enrich(ctx context.Context, m *meta.Meta)

	// UpdateReferences for a specific zettel.
	// Returns set of zettel identifier that must also be checked for changes.
	UpdateReferences(context.Context, *ZettelIndex) *idset.Set

	// DeleteZettel removes index data for given zettel.
	// Returns set of zettel identifier that must also be checked for changes.
	DeleteZettel(context.Context, id.Zid) *idset.Set

	// Optimize removes unneeded space.
	Optimize()

	// ReadStats populates st with store statistics.
	ReadStats(st *Stats)

	// Dump the content to a Writer.
	Dump(io.Writer)
}
