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

package query_test

import (
	"context"
	"testing"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/query"
)

func TestMatchZidNegate(t *testing.T) {
	q := query.Parse(meta.KeyID + api.SearchOperatorHasNot + id.ZidVersion.String() + " " +
		meta.KeyID + api.SearchOperatorHasNot + id.ZidLicense.String())
	compiled := q.RetrieveAndCompile(context.Background(), nil, nil)

	testCases := []struct {
		zid id.Zid
		exp bool
	}{
		{id.ZidVersion, false},
		{id.ZidLicense, false},
		{id.ZidAuthors, true},
	}
	for i, tc := range testCases {
		m := meta.New(tc.zid)
		if compiled.Terms[0].Match(m) != tc.exp {
			if tc.exp {
				t.Errorf("%d: meta %v must match %q", i, m.Zid, q)
			} else {
				t.Errorf("%d: meta %v must not match %q", i, m.Zid, q)
			}
		}
	}
}
