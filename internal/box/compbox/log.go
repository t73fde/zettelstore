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
	"fmt"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logging"
)

func genLogM(zid id.Zid) *meta.Meta {
	m := getTitledMeta(zid, "Zettelstore Log")
	m.Set(meta.KeySyntax, meta.ValueSyntaxText)
	m.Set(meta.KeyModified, meta.Value(kernel.Main.GetLastLogTime().Local().Format(id.TimestampLayout)))
	return m
}

func genLogC() []byte {
	const tsFormat = "2006-01-02 15:04:05.999999"
	var buf []byte
	for _, entry := range kernel.Main.RetrieveLogEntries() {
		ts := entry.TS.Format(tsFormat)
		buf = append(buf, ts...)
		for j := len(ts); j < len(tsFormat); j++ {
			buf = append(buf, '0')
		}
		buf = fmt.Appendf(buf, " %s %s %s %s\n",
			logging.LevelStringPad(entry.Level),
			entry.Prefix,
			entry.Message,
			entry.Details,
		)
	}
	return buf
}
