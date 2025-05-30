//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package logger

import (
	"context"
	"strconv"
	"sync"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
)

// DMessage presents a message to log.
type DMessage struct {
	dlogger *DLogger
	level   DLevel
	buf     []byte
}

func dnewDMessage(logger *DLogger, level DLevel) *DMessage {
	if logger != nil {
		if logger.topParent.Level() <= level {
			m := messagePool.Get().(*DMessage)
			m.dlogger = logger
			m.level = level
			m.buf = append(m.buf[:0], logger.context...)
			return m
		}
	}
	return nil
}

func drecycleDMessage(m *DMessage) {
	messagePool.Put(m)
}

var messagePool = &sync.Pool{
	New: func() any {
		return &DMessage{
			buf: make([]byte, 0, 500),
		}
	},
}

// Enabled returns whether the message will log or not.
func (m *DMessage) Enabled() bool {
	return m != nil && m.level != DNeverLevel
}

// Str adds a string value to the full message
func (m *DMessage) Str(text, val string) *DMessage {
	if m.Enabled() {
		buf := append(m.buf, ',', ' ')
		buf = append(buf, text...)
		buf = append(buf, '=')
		m.buf = append(buf, val...)
	}
	return m
}

// Bool adds a boolean value to the full message
func (m *DMessage) Bool(text string, val bool) *DMessage {
	if val {
		m.Str(text, "true")
	} else {
		m.Str(text, "false")
	}
	return m
}

// Bytes adds a byte slice value to the full message
func (m *DMessage) Bytes(text string, val []byte) *DMessage {
	if m.Enabled() {
		buf := append(m.buf, ',', ' ')
		buf = append(buf, text...)
		buf = append(buf, '=')
		m.buf = append(buf, val...)
	}
	return m
}

// Err adds an error value to the full message
func (m *DMessage) Err(err error) *DMessage {
	if err != nil {
		return m.Str("error", err.Error())
	}
	return m
}

// Int adds an integer to the full message
func (m *DMessage) Int(text string, i int64) *DMessage {
	return m.Str(text, strconv.FormatInt(i, 10))
}

// Uint adds an unsigned integer to the full message
func (m *DMessage) Uint(text string, u uint64) *DMessage {
	return m.Str(text, strconv.FormatUint(u, 10))
}

// User adds the user-id field of the given user to the message.
func (m *DMessage) User(ctx context.Context) *DMessage {
	if m.Enabled() {
		if up := m.dlogger.uProvider; up != nil {
			if user := up.GetUser(ctx); user != nil {
				m.buf = append(m.buf, ", user="...)
				if userID, found := user.Get(meta.KeyUserID); found {
					m.buf = append(m.buf, userID...)
				} else {
					m.buf = append(m.buf, user.Zid.Bytes()...)
				}
			}
		}
	}
	return m
}

// Zid adds a zettel identifier to the full message
func (m *DMessage) Zid(zid id.Zid) *DMessage {
	return m.Bytes("zid", zid.Bytes())
}

// Msg add the given text to the message and writes it to the log.
func (m *DMessage) Msg(text string) {
	if m.Enabled() {
		_ = m.dlogger.dwriteDMessage(m.level, text, m.buf)
		drecycleDMessage(m)
	}
}

// Child creates a child logger with context of this message.
func (m *DMessage) Child() *DLogger {
	return dnewFromDMessage(m)
}
