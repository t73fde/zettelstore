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

package manager

import (
	"net/url"
	"slices"
	"testing"
)

func TestSetupBoxURIs(t *testing.T) {
	testcases := []struct {
		name     string
		uris     []string
		exp      []string
		expErr   bool
		readonly bool
	}{
		// {"nothing", nil, nil, false, false},
		// {"nothing/ro", nil, nil, false, true},

		// {"only comp", []string{"comp://"}, []string{"comp:?name=1"}, false, false},

		{"single-mem", []string{"mem:"}, []string{"mem:?name=mem"}, false, false},
		{"single-mem/ro", []string{"mem:"}, []string{"mem:?name=mem&readonly="}, false, true},
		{"single-mem/ro/ro", []string{"mem:?readonly"}, []string{"mem:?name=mem&readonly="}, false, true},
		{"double-mem", []string{"mem:", "mem:"}, []string{"mem:?name=mem", "mem:?name=2"}, false, false},
		{"double-mem/ro", []string{"mem:", "mem:"}, []string{"mem:?name=mem&readonly=", "mem:?name=2&readonly="}, false, true},
		{"triple-mem", []string{"mem:", "mem:", "mem:"}, []string{"mem:?name=mem", "mem:?name=2", "mem:?name=3"}, false, false},
		{"collide-mem-3", []string{"mem:", "mem:?name=3", "mem:"}, []string{"mem:?name=mem", "mem:?name=3", "mem:?name=3b1"}, false, false},

		{"name-mem", []string{"mem:?name=mm", "mem:?name=mm"}, nil, true, false},
		{"name-mem-space", []string{"mem:?name=mm", "mem:?name=m m"}, nil, true, false},
		{"name-mem-space-only", []string{"mem:?name= "}, []string{"mem:?name=mem"}, false, false},

		{"dir-path", []string{"dir://./zettel/"}, []string{"dir://./zettel/?name=zettel"}, false, false},
		{"file-opaque", []string{"file:zettel.zip"}, []string{"file:zettel.zip?name=zettel"}, false, false},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			uris := make([]*url.URL, len(tc.uris))
			for i, uri := range tc.uris {
				u, err := url.Parse(uri)
				if err != nil {
					t.Fatal(err)
				}
				uris[i] = u
			}
			err := setupBoxURIs(uris, tc.readonly)
			if err == nil && tc.expErr {
				t.Errorf("error expected, but got none")
			} else if err != nil && !tc.expErr {
				t.Errorf("no error expected, but got %v", err)
			} else if err == nil {
				got := make([]string, len(uris))
				for i, u := range uris {
					got[i] = u.String()
				}
				if !slices.Equal(tc.exp, got) {
					t.Errorf("expected %v, but got %v", tc.exp, got)
				}
			}
		})
	}
}

func TestNameFromPath(t *testing.T) {
	testcases := []struct {
		path string
		exp  string
	}{
		{"", ""},
		{"/", ""},
		{".", ""},
		{"..", ""},
		{"manual", "manual"},
		{"manual.", "manual"},
		{"/manual", "manual"},
		{"/manual/", "manual"},
		{"manual/", "manual"},
		{"./manual", "manual"},
		{"/manual.zip", "manual"},
		{"/manual.zip/", "manual"},
		{"Exposé", "expose"},
		{"Mensch Maschine", "menschmaschine"},
	}
	for _, tc := range testcases {
		t.Run(tc.path, func(t *testing.T) {
			if got := nameFromPath(tc.path); got != tc.exp {
				t.Errorf("path %q, expected %q, but got %q", tc.path, tc.exp, got)
			}
		})
	}
}
