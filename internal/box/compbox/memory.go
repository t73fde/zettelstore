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
	"os"
	"runtime"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/kernel"
)

func genMemoryM(zid id.Zid) *meta.Meta {
	return getTitledMeta(zid, "Zettelstore Memory")
}

func genMemoryC() []byte {
	pageSize := os.Getpagesize()
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)

	var buf []byte
	buf = append(buf, "|=Name|=Value>\n"...)
	buf = fmt.Appendf(buf, "|Page Size|%d\n", pageSize)
	buf = fmt.Appendf(buf, "|Pages|%d\n", m.HeapSys/uint64(pageSize))
	buf = fmt.Appendf(buf, "|Heap Objects|%d\n", m.HeapObjects)
	buf = fmt.Appendf(buf, "|Heap Sys (KiB)|%d\n", m.HeapSys/1024)
	buf = fmt.Appendf(buf, "|Heap Inuse (KiB)|%d\n", m.HeapInuse/1024)
	buf = fmt.Appendf(buf, "|CPUs|%d\n", runtime.NumCPU())
	buf = fmt.Appendf(buf, "|Threads|%d\n", runtime.NumGoroutine())
	debug := kernel.Main.GetConfig(kernel.CoreService, kernel.CoreDebug).(bool)
	if debug {
		for i, bysize := range m.BySize {
			buf = fmt.Appendf(buf, "|Size %2d: %d|%d - %d &rarr; %d\n",
				i, bysize.Size, bysize.Mallocs, bysize.Frees, bysize.Mallocs-bysize.Frees)
		}
	}
	return buf
}
