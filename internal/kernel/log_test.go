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

package kernel

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"
)

func TestLogHand(t *testing.T) {
	t.SkipNow() // Handler does not implement Groups
	var sb strings.Builder
	logWriter := newKernelLogWriter(&sb, 1000)
	h := newKernelLogHandler(logWriter, slog.LevelInfo)
	results := func() []map[string]any {
		var ms []map[string]any
		for _, entry := range logWriter.retrieveLogEntries() {
			m := map[string]any{
				slog.LevelKey:   entry.Level,
				slog.MessageKey: entry.Message,
			}
			if ts := entry.TS; !ts.IsZero() {
				m[slog.TimeKey] = ts
			}
			details := entry.Details
			fmt.Printf("%q\n", details)
			for len(details) > 0 {
				pos := strings.Index(details, "=")
				if pos <= 0 {
					break
				}
				key := details[:pos]
				details = details[pos+1:]

				if details == "" || details[0] == '[' {
					break
				}

				pos = strings.Index(details, " ")
				var val string
				if pos <= 0 {
					val = details
					details = ""
				} else {
					val = details[:pos]
					details = details[pos+1:]
				}
				fmt.Printf("key %q, val %q\n", key, val)
				m[key] = val
			}
			ms = append(ms, m)
		}
		return ms
	}
	err := slogtest.TestHandler(h, results)
	if err != nil {
		t.Error(err)
	}
	t.Error(sb.String())
}
