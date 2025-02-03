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
	"bytes"
	"context"
	"fmt"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/zettel/meta"
)

func genSxM(zid id.Zid) *meta.Meta {
	return getTitledMeta(zid, "Zettelstore Sx Engine")
}

func genSxC(context.Context, *compBox) []byte {
	var buf bytes.Buffer
	buf.WriteString("|=Name|=Value>\n")
	fmt.Fprintf(&buf, "|Symbols|%d\n", sx.MakeSymbol("NIL").Factory().Size())
	return buf.Bytes()
}
