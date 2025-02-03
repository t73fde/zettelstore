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

package compbox

import (
	"bytes"
	"context"
	"fmt"

	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/zettel/meta"
)

func genManagerM(zid id.Zid) *meta.Meta {
	return getTitledMeta(zid, "Zettelstore Box Manager")
}

func genManagerC(context.Context, *compBox) []byte {
	kvl := kernel.Main.GetServiceStatistics(kernel.BoxService)
	if len(kvl) == 0 {
		return nil
	}
	var buf bytes.Buffer
	buf.WriteString("|=Name|=Value>\n")
	for _, kv := range kvl {
		fmt.Fprintf(&buf, "| %v | %v\n", kv.Key, kv.Value)
	}
	return buf.Bytes()
}
