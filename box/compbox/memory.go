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
	"os"
	"runtime"

	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/zettel/meta"
)

func genMemoryM(zid id.Zid) *meta.Meta {
	return getTitledMeta(zid, "Zettelstore Memory")
}

func genMemoryC(context.Context, *compBox) []byte {
	pageSize := os.Getpagesize()
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)

	var buf bytes.Buffer
	buf.WriteString("|=Name|=Value>\n")
	fmt.Fprintf(&buf, "|Page Size|%d\n", pageSize)
	fmt.Fprintf(&buf, "|Pages|%d\n", m.HeapSys/uint64(pageSize))
	fmt.Fprintf(&buf, "|Heap Objects|%d\n", m.HeapObjects)
	fmt.Fprintf(&buf, "|Heap Sys (KiB)|%d\n", m.HeapSys/1024)
	fmt.Fprintf(&buf, "|Heap Inuse (KiB)|%d\n", m.HeapInuse/1024)
	fmt.Fprintf(&buf, "|CPUs|%d\n", runtime.NumCPU())
	fmt.Fprintf(&buf, "|Threads|%d\n", runtime.NumGoroutine())
	debug := kernel.Main.GetConfig(kernel.CoreService, kernel.CoreDebug).(bool)
	if debug {
		for i, bysize := range m.BySize {
			fmt.Fprintf(&buf, "|Size %2d: %d|%d - %d &rarr; %d\n",
				i, bysize.Size, bysize.Mallocs, bysize.Frees, bysize.Mallocs-bysize.Frees)
		}
	}
	return buf.Bytes()
}
