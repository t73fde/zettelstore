//-----------------------------------------------------------------------------
// Copyright (c) 2026-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2026-present Detlef Stern
//-----------------------------------------------------------------------------

// Package index manages the search index.
package index

import (
	"context"

	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/idset"
	"zettelstore.de/z/internal/zettel"
)

// Indexer provides methods to work with a search index.
type Indexer interface {
	Enqueuer

	// Start the indexer.
	Start()

	// Stop the indexer.
	Stop()
}

// Fetcher declares all methods an Indexer needs to operate.
type Fetcher interface {
	// FetchZids returns the set of all zettel identifer.
	FetchZids(ctx context.Context) (idset.ZidSet, error)

	// GetZettel retrieves a specific zettel.
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)
}

// Enqueuer allows to operate with the search index.
type Enqueuer interface {
	// Enqueue signals that one or many zettel has been changed and the index
	// should be updated to reflect the changes. Updates zettel data can be
	// retrieved by the given Fetcher.
	// Enqueue is called in the context of the box.Manager goroutine. It must
	// not take too long to process the enqueue signal.
	Enqueue(Fetcher, box.UpdateReason, id.Zid)
}
