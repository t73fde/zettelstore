//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

package compbox

import (
	"bytes"
	"context"
	"fmt"
	"runtime/debug"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

func genModulesM(zid id.Zid) *meta.Meta {
	m := getTitledMeta(zid, "Zettelstore Modules")
	m.Set(meta.KeyVisibility, meta.ValueVisibilityLogin)
	return m
}

func genModulesC(context.Context, *compBox) []byte {
	info, ok := debug.ReadBuildInfo()
	var buf bytes.Buffer
	if !ok {
		buf.WriteString("No module info available\n")
	} else {
		buf.WriteString("|=Module|Version\n")
		fmt.Fprintf(&buf, "|Zettelstore|{{%v}}\n", id.ZidVersion)
		for _, m := range info.Deps {
			fmt.Fprintf(&buf, "|%s|%s\n", m.Path, m.Version)
		}
	}
	fmt.Fprintf(&buf, "\nSee [[Zettelstore Dependencies|%v]] for license details.", id.ZidDependencies)
	return buf.Bytes()
}
