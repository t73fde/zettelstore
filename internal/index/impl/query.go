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

package impl

import (
	"context"

	"t73f.de/r/zero/roster"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/index"
)

// SearchEqual returns all zettel that contains the given exact word.
// The word must be normalized through Unicode NFKD, trimmed and not empty.
func (idx *Index) SearchEqual(word string) *index.Result {
	return idx.newResult(roster.Roster[id.Zid]{})
}

// SearchPrefix returns all zettel that have a word with the given prefix.
// The prefix must be normalized through Unicode NFKD, trimmed and not empty.
func (idx *Index) SearchPrefix(prefix string) *index.Result {
	return idx.newResult(roster.Roster[id.Zid]{})
}

// SearchSuffix returns all zettel that have a word with the given suffix.
// The suffix must be normalized through Unicode NFKD, trimmed and not empty.
func (idx *Index) SearchSuffix(suffix string) *index.Result {
	return idx.newResult(roster.Roster[id.Zid]{})
}

// SearchContains returns all zettel that contains the given string.
// The string must be normalized through Unicode NFKD, trimmed and not empty.
func (idx *Index) SearchContains(s string) *index.Result {
	return idx.newResult(roster.Roster[id.Zid]{})
}

// newResult wraps base and marks it as possibly incomplete, as long as the
// initial build of the index is not complete.
func (idx *Index) newResult(base roster.Roster[id.Zid]) *index.Result {
	if idx.isReady() {
		return index.NewResult(base)
	}
	return index.NewPartialResult(base)
}

// GetMeta returns metadata of the given zettel. It signals an error if
// the zettel is not found or for other retrieval errors.
func (idx *Index) GetMeta(zid id.Zid) (*meta.Meta, error) {
	if in := idx.getActiveInstance(); in != nil {
		if zettel := in.getZettel(zid); zettel != nil {
			m := zettel.meta.toMeta(zid, in.dict)
			return m, nil
		}
	}
	return nil, box.ErrZettelNotFound{Zid: zid}
}

// Enrich computes additional properties and updates the given metadata.
func (idx *Index) Enrich(context.Context, *meta.Meta) {
	in := idx.getActiveInstance()
	if in == nil {
		return
	}
}
