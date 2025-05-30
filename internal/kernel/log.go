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

package kernel

import (
	"os"
	"slices"
	"sync"
	"time"

	"zettelstore.de/z/internal/logger"
)

// kernelDLogWriter adapts an io.Writer to a LogWriter
type kernelDLogWriter struct {
	mx       sync.RWMutex // protects buf, serializes w.Write and retrieveLogEntries
	lastLog  time.Time
	buf      []byte
	writePos int
	data     []logEntry
	full     bool
}

// newKernelLogWriter creates a new LogWriter for kernel logging.
func newKernelLogWriter(capacity int) *kernelDLogWriter {
	if capacity < 1 {
		capacity = 1
	}
	return &kernelDLogWriter{
		lastLog: time.Now(),
		buf:     make([]byte, 0, 500),
		data:    make([]logEntry, capacity),
	}
}

func (klw *kernelDLogWriter) DWriteMessage(level logger.DLevel, ts time.Time, prefix, msg string, details []byte) error {
	klw.mx.Lock()

	if level > logger.DDebugLevel {
		klw.lastLog = ts
		klw.data[klw.writePos] = logEntry{
			level:   level,
			ts:      ts,
			prefix:  prefix,
			msg:     msg,
			details: slices.Clone(details),
		}
		klw.writePos++
		if klw.writePos >= cap(klw.data) {
			klw.writePos = 0
			klw.full = true
		}
	}

	klw.buf = klw.buf[:0]
	buf := klw.buf
	addTimestamp(&buf, ts)
	buf = append(buf, ' ')
	buf = append(buf, level.Format()...)
	buf = append(buf, ' ')
	if prefix != "" {
		buf = append(buf, prefix...)
		buf = append(buf, ' ')
	}
	buf = append(buf, msg...)
	buf = append(buf, details...)
	buf = append(buf, '\n')
	_, err := os.Stdout.Write(buf)

	klw.mx.Unlock()
	return err
}

func addTimestamp(buf *[]byte, ts time.Time) {
	year, month, day := ts.Date()
	itoa(buf, year, 4)
	*buf = append(*buf, '-')
	itoa(buf, int(month), 2)
	*buf = append(*buf, '-')
	itoa(buf, day, 2)
	*buf = append(*buf, ' ')
	hour, minute, second := ts.Clock()
	itoa(buf, hour, 2)
	*buf = append(*buf, ':')
	itoa(buf, minute, 2)
	*buf = append(*buf, ':')
	itoa(buf, second, 2)

}

func itoa(buf *[]byte, i, wid int) {
	var b [20]byte
	for bp := wid - 1; bp >= 0; bp-- {
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		i = q
	}
	*buf = append(*buf, b[:wid]...)
}

type logEntry struct {
	level   logger.DLevel
	ts      time.Time
	prefix  string
	msg     string
	details []byte
}

func (klw *kernelDLogWriter) retrieveLogEntries() []DLogEntry {
	klw.mx.RLock()
	defer klw.mx.RUnlock()

	if !klw.full {
		if klw.writePos == 0 {
			return nil
		}
		result := make([]DLogEntry, klw.writePos)
		for i := range klw.writePos {
			copyE2E(&result[i], &klw.data[i])
		}
		return result
	}
	result := make([]DLogEntry, cap(klw.data))
	pos := 0
	for j := klw.writePos; j < cap(klw.data); j++ {
		copyE2E(&result[pos], &klw.data[j])
		pos++
	}
	for j := range klw.writePos {
		copyE2E(&result[pos], &klw.data[j])
		pos++
	}
	return result
}

func (klw *kernelDLogWriter) getLastLogTime() time.Time {
	klw.mx.RLock()
	defer klw.mx.RUnlock()
	return klw.lastLog
}

func copyE2E(result *DLogEntry, origin *logEntry) {
	result.Level = origin.level
	result.TS = origin.ts
	result.Prefix = origin.prefix
	result.Message = origin.msg + string(origin.details)
}
