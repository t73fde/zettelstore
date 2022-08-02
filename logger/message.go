//-----------------------------------------------------------------------------
// Copyright (c) 2022 Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package logger

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	"zettelstore.de/c/api"
	"zettelstore.de/z/domain/id"
)

// Message presents a message to log.
type Message struct {
	logger *Logger
	level  Level
	buf    []byte
}

func newMessage(logger *Logger, level Level) *Message {
	if logger != nil {
		if logger.topParent.Level() <= level {
			m := messagePool.Get().(*Message)
			m.logger = logger
			m.level = level
			m.buf = append(m.buf[:0], logger.context...)
			return m
		}
	}
	return nil
}

func recycleMessage(m *Message) {
	messagePool.Put(m)
}

var messagePool = &sync.Pool{
	New: func() interface{} {
		return &Message{
			buf: make([]byte, 0, 500),
		}
	},
}

// Enabled returns whether the message will log or not.
func (m *Message) Enabled() bool {
	return m != nil && m.level != NeverLevel
}

// Str adds a string value to the full message
func (m *Message) Str(text, val string) *Message {
	if m.Enabled() {
		buf := append(m.buf, ',', ' ')
		buf = append(buf, text...)
		buf = append(buf, '=')
		m.buf = append(buf, val...)
	}
	return m
}

// Bool adds a boolean value to the full message
func (m *Message) Bool(text string, val bool) {
	if val {
		m.Str(text, "true")
	} else {
		m.Str(text, "false")
	}
}

// Bytes adds a byte slice value to the full message
func (m *Message) Bytes(text string, val []byte) *Message {
	if m.Enabled() {
		buf := append(m.buf, ',', ' ')
		buf = append(buf, text...)
		buf = append(buf, '=')
		m.buf = append(buf, val...)
	}
	return m
}

// Err adds an error value to the full message
func (m *Message) Err(err error) *Message {
	if err != nil {
		return m.Str("error", err.Error())
	}
	return m
}

// Int adds an integer to the full message
func (m *Message) Int(text string, i int64) *Message {
	return m.Str(text, strconv.FormatInt(i, 10))
}

// Uint adds an unsigned integer to the full message
func (m *Message) Uint(text string, u uint64) *Message {
	return m.Str(text, strconv.FormatUint(u, 10))
}

// User adds the user-id field of the given user to the message.
func (m *Message) User(ctx context.Context) *Message {
	if m.Enabled() {
		if up := m.logger.uProvider; up != nil {
			if user := up.GetUser(ctx); user != nil {
				m.buf = append(m.buf, ", user="...)
				if userID, found := user.Get(api.KeyUserID); found {
					m.buf = append(m.buf, userID...)
				} else {
					m.buf = append(m.buf, user.Zid.Bytes()...)
				}
			}
		}
	}
	return m
}

// HTTPIP adds the IP address of a HTTP request to the message.
func (m *Message) HTTPIP(r *http.Request) *Message {
	if r == nil {
		return m
	}
	if from := r.Header.Get("X-Forwarded-For"); from != "" {
		return m.Str("ip", from)
	}
	return m.Str("IP", r.RemoteAddr)
}

// Zid adds a zettel identifier to the full message
func (m *Message) Zid(zid id.Zid) *Message {
	return m.Bytes("zid", zid.Bytes())
}

// Msg add the given text to the message and writes it to the log.
func (m *Message) Msg(text string) {
	if m.Enabled() {
		m.logger.writeMessage(m.level, text, m.buf)
		recycleMessage(m)
	}
}

// Child creates a child logger with context of this message.
func (m *Message) Child() *Logger {
	return newFromMessage(m)
}
