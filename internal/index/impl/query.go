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
	"log/slog"
	"maps"
	"slices"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/index"
	"zettelstore.de/z/internal/logging"
)

// queryResult stores the result of a query, to be used in index.Result.
type queryResult = map[id.Zid]struct{}

type queryResultLog struct{ result queryResult }

func (qrl queryResultLog) LogValue() slog.Value {
	return slog.AnyValue(slices.Sorted(maps.Keys(qrl.result)))
}

// SearchEqual returns all zettel that contains the given exact word.
// The word must be normalized through Unicode NFKD, trimmed and not empty.
// In addition, word may contain a full URI.
func (idx *Index) SearchEqual(word string) *index.Result {
	if in := idx.activeInstance(); in != nil {
		ready := in.ready.Load()
		result := in.searchEqual(word)

		ctx := context.Background()
		idx.logger.LogAttrs(ctx, slog.LevelDebug, "SearchEqual",
			slog.Bool("ready", ready),
			slog.String("word", word),
			slog.Int("found", len(result)),
		)
		idx.logger.LogAttrs(ctx, logging.LevelTrace, "IDs",
			slog.Any("ids", queryResultLog{result}))
		return newResult(ready, result)
	}
	idx.logger.Debug("SearchEqual/inactive")
	return index.NewPartialResult(nil)
}

// SearchPrefix returns all zettel that have a word with the given prefix.
// The prefix must be normalized through Unicode NFKD, trimmed and not empty.
func (idx *Index) SearchPrefix(prefix string) *index.Result {
	if in := idx.activeInstance(); in != nil {
		ready := in.ready.Load()
		result := in.searchPrefix(prefix)

		ctx := context.Background()
		idx.logger.LogAttrs(ctx, slog.LevelDebug, "SearchPrefix",
			slog.Bool("ready", ready),
			slog.String("prefix", prefix),
			slog.Int("found", len(result)),
		)
		idx.logger.LogAttrs(ctx, logging.LevelTrace, "IDs",
			slog.Any("ids", queryResultLog{result}))
		return newResult(ready, result)
	}
	idx.logger.Debug("SearchPrefix/inactive")
	return index.NewPartialResult(nil)
}

// SearchSuffix returns all zettel that have a word with the given suffix.
// The suffix must be normalized through Unicode NFKD, trimmed and not empty.
func (idx *Index) SearchSuffix(suffix string) *index.Result {
	if in := idx.activeInstance(); in != nil {
		ready := in.ready.Load()
		result := in.searchSuffix(suffix)

		ctx := context.Background()
		idx.logger.LogAttrs(ctx, slog.LevelDebug, "SearchSuffix",
			slog.Bool("ready", ready),
			slog.String("suffix", suffix),
			slog.Int("found", len(result)),
		)
		idx.logger.LogAttrs(ctx, logging.LevelTrace, "IDs",
			slog.Any("ids", queryResultLog{result}))
		return newResult(ready, result)
	}
	idx.logger.Debug("SearchSuffix/inactive")
	return index.NewPartialResult(nil)
}

// SearchContains returns all zettel that contains the given string.
// The string must be normalized through Unicode NFKD, trimmed and not empty.
func (idx *Index) SearchContains(s string) *index.Result {
	if in := idx.activeInstance(); in != nil {
		ready := in.ready.Load()
		result := in.searchContains(s)

		ctx := context.Background()
		idx.logger.LogAttrs(ctx, slog.LevelDebug, "SearchContains",
			slog.Bool("ready", ready),
			slog.String("s", s),
			slog.Int("found", len(result)),
		)
		idx.logger.LogAttrs(ctx, logging.LevelTrace, "IDs",
			slog.Any("ids", queryResultLog{result}))
		return newResult(ready, result)
	}
	idx.logger.Debug("SearchContains/inactive")
	return index.NewPartialResult(nil)
}

// newResult wraps base. The result is marked as possibly incomplete, if the
// initial build was not complete when the search started.
func newResult(ready bool, base queryResult) *index.Result {
	if ready {
		return index.NewResult(base)
	}
	return index.NewPartialResult(base)
}

// GetMeta returns metadata of the given zettel. It signals an error if
// the zettel is not found or for other retrieval errors.
func (idx *Index) GetMeta(zid id.Zid) (*meta.Meta, error) {
	if in := idx.activeInstance(); in != nil {
		if _, sz, found := in.findZettel(zid); found {
			return in.dict.buildMeta(zid, sz.meta), nil
		}
	}
	return nil, box.ErrZettelNotFound{Zid: zid}
}

// Enrich computes additional properties and updates the given metadata.
func (idx *Index) Enrich(m *meta.Meta) {
	if in := idx.activeInstance(); in != nil {
		in.enrich(m)
	} else {
		m.Set(meta.KeyIndexState, meta.ValueIndexStateIncomplete)
	}
}
