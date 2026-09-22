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
type WordSet map[string]struct{}

// NewWordSet returns a new WordSet.
func NewWordSet() WordSet { return make(WordSet) }

// AddBytes adds one byte-slice word to the set
func (ws WordSet) AddBytes(b []byte) { ws[string(b)] = struct{}{} }

// AddString adds one string word to the set
func (ws WordSet) AddString(s string) { ws[s] = struct{}{} }

// AddURI adds one string-encoded URI to the set
func (ws WordSet) AddURI(s string) { ws[s] = struct{}{} }

// Has returns an indication, where s is an element of the set.
func (ws WordSet) Has(s string) bool {
	_, found := ws[s]
	return found
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
func (ws WordSet) Diff(oldState []string) (newWords, removeWords []string) {
	if len(ws) == 0 {
		return nil, oldState
	}
	if len(oldState) == 0 {
		return ws.Words(), nil
	}
	oldSet := make(WordSet, len(oldState))
	for _, ow := range oldState {
		if ws.Has(ow) {
			oldSet.AddString(ow)
		} else {
			removeWords = append(removeWords, ow)
		}
	}
	for w := range ws {
		if !oldSet.Has(w) {
			newWords = append(newWords, w)
		}
	}
	return newWords, removeWords
}
