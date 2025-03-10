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

import "testing"

func TestCapitalizeMetaKey(t *testing.T) {
	var testcases = []struct {
		key string
		exp string
	}{
		{"", ""},
		{"alt-url", "Alt URL"},
		{"author", "Author"},
		{"back", "Back"},
		{"box-number", "Box Number"},
		{"cite-key", "Cite Key"},
		{"fedi-url", "Fedi URL"},
		{"github-url", "GitHub URL"},
		{"hshn-bib", "Hshn Bib"},
		{"job-url", "Job URL"},
		{"new-user-id", "New User Id"},
		{"origin-zid", "Origin Zid"},
		{"site-url", "Site URL"},
	}
	for _, tc := range testcases {
		t.Run(tc.key, func(t *testing.T) {
			got := capitalizeMetaKey(tc.key)
			if got != tc.exp {
				t.Errorf("capitalize(%q) == %q, but got %q", tc.key, tc.exp, got)
			}
		})
	}
}
