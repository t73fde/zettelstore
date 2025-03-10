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

package store

// WordSet contains the set of all words, with the count of their occurrences.
type WordSet map[string]int

// NewWordSet returns a new WordSet.
func NewWordSet() WordSet { return make(WordSet) }

// Add one word to the set
func (ws WordSet) Add(s string) {
	ws[s] = ws[s] + 1
}

// Words gives the slice of all words in the set.
func (ws WordSet) Words() []string {
	if len(ws) == 0 {
		return nil
	}
	words := make([]string, 0, len(ws))
	for w := range ws {
		words = append(words, w)
	}
	return words
}

// Diff calculates the word slice to be added and to be removed from oldWords
// to get the given word set.
func (ws WordSet) Diff(oldWords []string) (newWords, removeWords []string) {
	if len(ws) == 0 {
		return nil, oldWords
	}
	if len(oldWords) == 0 {
		return ws.Words(), nil
	}
	oldSet := make(WordSet, len(oldWords))
	for _, ow := range oldWords {
		if _, ok := ws[ow]; ok {
			oldSet[ow] = 1
			continue
		}
		removeWords = append(removeWords, ow)
	}
	for w := range ws {
		if _, ok := oldSet[w]; ok {
			continue
		}
		newWords = append(newWords, w)
	}
	return newWords, removeWords
}
