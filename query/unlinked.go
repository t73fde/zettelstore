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
	"t73f.de/r/zsc/api"
	"zettelstore.de/z/strfun"
	"zettelstore.de/z/zettel/meta"
)

// UnlinkedSpec contains all specification values to calculate unlinked references.
type UnlinkedSpec struct {
	words []string
}

// Print the spec on the given print environment.
func (spec *UnlinkedSpec) Print(pe *PrintEnv) {
	pe.printSpace()
	pe.writeString(api.UnlinkedDirective)
	for _, word := range spec.words {
		pe.writeStrings(" ", api.PhraseDirective, " ", word)
	}
}

// GetWords returns all title words of a given query result.
func (spec *UnlinkedSpec) GetWords(metaSeq []*meta.Meta) []string {
	if words := spec.words; len(words) > 0 {
		result := make([]string, len(words))
		copy(result, words)
		return result
	}
	result := make([]string, 0, len(metaSeq)*4) // Assumption: four words per title
	for _, m := range metaSeq {
		title, hasTitle := m.Get(api.KeyTitle)
		if !hasTitle {
			continue
		}
		result = append(result, strfun.MakeWords(title)...)
	}
	return result
}
