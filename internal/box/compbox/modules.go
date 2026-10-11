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

func genModulesC() []byte {
	info, ok := debug.ReadBuildInfo()
	var buf []byte
	if !ok {
		buf = append(buf, "No module info available\n"...)
	} else {
		buf = append(buf, "|=Module|Version\n"...)
		buf = fmt.Appendf(buf, "|Zettelstore|{{%v}}\n", id.ZidVersion)
		for _, m := range info.Deps {
			buf = fmt.Appendf(buf, "|%s|%s\n", m.Path, m.Version)
		}
	}
	buf = fmt.Appendf(buf, "\nSee [[Zettelstore Dependencies|%v]] for license details.", id.ZidDependencies)
	return buf
}
