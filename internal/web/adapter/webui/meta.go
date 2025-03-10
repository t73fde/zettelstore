//-----------------------------------------------------------------------------
// Copyright (c) 2024-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2024-present Detlef Stern
//-----------------------------------------------------------------------------

package webui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func capitalizeMetaKey(key string) string {
	var sb strings.Builder
	for i, word := range strings.Split(key, "-") {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if newWord, isSpecial := specialWords[word]; isSpecial {
			if newWord == "" {
				sb.WriteString(strings.ToTitle(word))
			} else {
				sb.WriteString(newWord)
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(word)
		if r == utf8.RuneError {
			sb.WriteString(word)
			continue
		}
		sb.WriteRune(unicode.ToTitle(r))
		sb.WriteString(word[size:])
	}
	return sb.String()
}

var specialWords = map[string]string{
	"css":    "",
	"html":   "",
	"github": "GitHub",
	"http":   "",
	"https":  "",
	"pdf":    "",
	"svg":    "",
	"url":    "",
}
