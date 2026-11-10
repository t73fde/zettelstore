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

package query

import (
	"slices"

	"t73f.de/r/zsc/domain/meta"
)

// SelectResult is the outcome of a selection of zettel metadata, as performed
// by the SelectMeta method of a box. It contains the selected metadata,
// ordered as requested by the query (by default: descending zettel id).
//
// The zero value is an empty result that is ready to use. It is returned
// together with a non-nil error if the selection failed.
//
// The metadata is accessed by methods only, so that the internal
// representation can be changed (e.g. to a sequence) without affecting
// callers. Information about the execution of the query (index readiness,
// duration, explanation) will be added later.
//
// Copies of a SelectResult share the same underlying storage. After a copy
// has been made, only one of the copies should be modified.
type SelectResult struct {
	// metas contains the metadata of all zettel that match the query, in the
	// order of the result. It is nil if no zettel matched or if the selection
	// failed.
	metas []*meta.Meta
}

// NewSelectResult creates a result from the given metadata. The order of the
// slice is the order of the result. The result takes ownership of the slice,
// the caller must not modify it afterwards.
func NewSelectResult(from []*meta.Meta) SelectResult {
	return SelectResult{metas: from}
}

// IsEmpty returns true if no metadata was selected.
func (sr SelectResult) IsEmpty() bool { return len(sr.metas) == 0 }

// Count returns the number of selected metadata.
func (sr SelectResult) Count() int { return len(sr.metas) }

// Values returns a sequence of the selected metadata in the order of the
// result. The result must not be modified while the sequence is consumed.
func (sr SelectResult) Values() meta.Seq { return slices.Values(sr.metas) }

// First returns the first metadata of the result, i.e. the one that comes
// first in the order of the result (by default: the zettel with the highest
// zettel id). It panics if the result is empty; use [SelectResult.IsEmpty]
// or [SelectResult.Count] to check beforehand.
func (sr SelectResult) First() *meta.Meta { return sr.metas[0] }

// Last returns the last metadata of the result, i.e. the one that comes
// last in the order of the result (by default: the zettel with the lowest
// zettel id). It panics if the result is empty; use [SelectResult.IsEmpty]
// or [SelectResult.Count] to check beforehand.
func (sr SelectResult) Last() *meta.Meta { return sr.metas[len(sr.metas)-1] }
