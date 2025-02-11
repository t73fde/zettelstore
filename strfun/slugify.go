//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

package strfun

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeWords produces a word list that is normalized for better searching.
func NormalizeWords(s string) (result []string) {
	word := make([]rune, 0, len(s))
	for _, r := range norm.NFKD.String(s) {
		if unicode.Is(unicode.Diacritic, r) {
			continue
		}
		if unicode.In(r, unicode.Letter, unicode.Number) {
			word = append(word, unicode.ToLower(r))
		} else if !unicode.In(r, unicode.Mark, unicode.Sk, unicode.Lm) && len(word) > 0 {
			result = append(result, string(word))
			word = word[:0]
		}
	}
	if len(word) > 0 {
		result = append(result, string(word))
	}
	return result
}

// Slugify returns a string that can be used as part of an URL
func Slugify(s string) string {
	words := NormalizeWords(s)
	return strings.Join(words, "-")
}
