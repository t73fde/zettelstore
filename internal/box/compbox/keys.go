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
	"fmt"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/internal/kernel"
)

func genKeysM(zid id.Zid) *meta.Meta {
	m := getTitledMeta(zid, "Zettelstore Supported Metadata Keys")
	m.Set(meta.KeyCreated, meta.Value(kernel.Main.GetConfig(kernel.CoreService, kernel.CoreVTime).(string)))
	m.Set(meta.KeyVisibility, meta.ValueVisibilityLogin)
	return m
}

func genKeysC(context.Context, *compBox) []byte {
	keys := meta.GetSortedKeyDescriptions()
	var buf bytes.Buffer
	buf.WriteString("|=Name<|=Type<|=Computed?:|=Property?:\n")
	for _, kd := range keys {
		fmt.Fprintf(&buf,
			"|[[%v|query:%v?]]|%v|%v|%v\n", kd.Name, kd.Name, kd.Type.Name, kd.IsComputed(), kd.IsProperty())
	}
	return buf.Bytes()
}
