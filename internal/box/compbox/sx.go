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

package compbox

import (
	"fmt"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

func genSxM(zid id.Zid) *meta.Meta {
	return getTitledMeta(zid, "Zettelstore Sx Engine")
}

func genSxC() []byte {
	var buf []byte
	buf = append(buf, "|=Name|=Value>\n"...)
	numSymbols := 0
	for pkg := range sx.AllPackages() {
		if size := pkg.Size(); size > 0 {
			buf = fmt.Appendf(buf, "|Symbols in package %q|%d\n", pkg.Name(), size)
			numSymbols += size
		}
	}
	buf = fmt.Appendf(buf, "|All symbols|%d\n", numSymbols)
	return buf
}
