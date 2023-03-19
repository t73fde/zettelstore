//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package impl

import (
	"os"
	"sync"
	"time"

	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
)

// kernelLogWriter adapts an io.Writer to a LogWriter
type kernelLogWriter struct {
	mx       sync.RWMutex // protects buf, serializes w.Write and retrieveLogEntries
	lastLog  time.Time
	buf      []byte
	writePos int
	data     []logEntry
	full     bool
}

// newKernelLogWriter creates a new LogWriter for kernel logging.
func newKernelLogWriter(capacity int) *kernelLogWriter {
	if capacity < 1 {
		capacity = 1
	}
	return &kernelLogWriter{
		lastLog: time.Now(),
		buf:     make([]byte, 0, 500),
		data:    make([]logEntry, capacity),
	}
}

func (klw *kernelLogWriter) WriteMessage(level logger.Level, ts time.Time, prefix, msg string, details []byte) error {
	klw.mx.Lock()

	if level > logger.DebugLevel {
		klw.lastLog = ts
		klw.data[klw.writePos] = logEntry{
			level:   level,
			ts:      ts,
			prefix:  prefix,
			msg:     msg,
			details: append([]byte(nil), details...),
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
	if level == logger.PanicLevel {
		panic(err)
	}
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
	level   logger.Level
	ts      time.Time
	prefix  string
	msg     string
	details []byte
}

func (klw *kernelLogWriter) retrieveLogEntries() []kernel.LogEntry {
	klw.mx.RLock()
	defer klw.mx.RUnlock()

	if !klw.full {
		if klw.writePos == 0 {
			return nil
		}
		result := make([]kernel.LogEntry, klw.writePos)
		for i := 0; i < klw.writePos; i++ {
			copyE2E(&result[i], &klw.data[i])
		}
		return result
	}
	result := make([]kernel.LogEntry, cap(klw.data))
	pos := 0
	for j := klw.writePos; j < cap(klw.data); j++ {
		copyE2E(&result[pos], &klw.data[j])
		pos++
	}
	for j := 0; j < klw.writePos; j++ {
		copyE2E(&result[pos], &klw.data[j])
		pos++
	}
	return result
}

func (klw *kernelLogWriter) getLastLogTime() time.Time {
	klw.mx.RLock()
	defer klw.mx.RUnlock()
	return klw.lastLog
}

func copyE2E(result *kernel.LogEntry, origin *logEntry) {
	result.Level = origin.level
	result.TS = origin.ts
	result.Prefix = origin.prefix
	result.Message = origin.msg + string(origin.details)
}
