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

package compbox

import (
	"bytes"
	"context"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

func genConfigZettelM(zid id.Zid) *meta.Meta {
	if myConfig == nil {
		return nil
	}
	return getTitledMeta(zid, "Zettelstore Startup Configuration")
}

func genConfigZettelC(context.Context, *compBox) []byte {
	var buf bytes.Buffer
	for i, p := range myConfig.Pairs() {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString("; ''")
		buf.WriteString(p.Key)
		buf.WriteString("''")
		if p.Value != "" {
			buf.WriteString("\n: ``")
			for _, r := range p.Value {
				if r == '`' {
					buf.WriteByte('\\')
				}
				buf.WriteRune(r)
			}
			buf.WriteString("``")
		}
	}
	return buf.Bytes()
}
