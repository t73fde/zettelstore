//-----------------------------------------------------------------------------
// Copyright (c) 2023-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2023-present Detlef Stern
//-----------------------------------------------------------------------------

package query

import (
	"slices"

	"t73f.de/r/zero/strings"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/webapi"
)

// UnlinkedSpec contains all specification values to calculate unlinked references.
type UnlinkedSpec struct {
	phraseWords []string
}

// Print the spec on the given print environment.
func (spec *UnlinkedSpec) Print(pe *PrintEnv) {
	pe.printSpace()
	pe.writeString(webapi.UnlinkedDirective)
	for _, phrase := range spec.phraseWords {
		pe.writeStrings(" ", webapi.PhraseDirective, " ", phrase)
	}
}

// GetPhraseWords returns all phrase words of a "UNLINKED" query.
func (spec *UnlinkedSpec) GetPhraseWords(metaList []*meta.Meta) []string {
	if words := spec.phraseWords; len(words) > 0 {
		// Found an explicit phrase
		return slices.Clone(words)
	}

	// Calculate phrase out of zettel title.
	result := make([]string, 0, len(metaList)*6) // Assumption: six words per title
	for _, m := range metaList {
		title, hasTitle := m.Get(meta.KeyTitle)
		if !hasTitle {
			continue
		}
		result = append(result, strings.SplitWords(string(title))...)
	}
	return result
}
