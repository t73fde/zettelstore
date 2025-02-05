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

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/kernel"
)

func genLogM(zid id.Zid) *meta.Meta {
	m := getTitledMeta(zid, "Zettelstore Log")
	m.Set(api.KeySyntax, meta.SyntaxText)
	m.Set(api.KeyModified, meta.Value(kernel.Main.GetLastLogTime().Local().Format(id.TimestampLayout)))
	return m
}

func genLogC(context.Context, *compBox) []byte {
	const tsFormat = "2006-01-02 15:04:05.999999"
	entries := kernel.Main.RetrieveLogEntries()
	var buf bytes.Buffer
	for _, entry := range entries {
		ts := entry.TS.Format(tsFormat)
		buf.WriteString(ts)
		for j := len(ts); j < len(tsFormat); j++ {
			buf.WriteByte('0')
		}
		buf.WriteByte(' ')
		buf.WriteString(entry.Level.Format())
		buf.WriteByte(' ')
		buf.WriteString(entry.Prefix)
		buf.WriteByte(' ')
		buf.WriteString(entry.Message)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}
