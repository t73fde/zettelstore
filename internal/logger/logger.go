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

// Package logger implements a logging package for use in the Zettelstore.
package logger

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"t73f.de/r/zsc/domain/meta"
)

// Level defines the possible log levels
type Level uint8

// Constants for Level
const (
	NoLevel        Level = iota // the absent log level
	TraceLevel                  // Log most internal activities
	DebugLevel                  // Log most data updates
	InfoLevel                   // Log normal activities
	ErrorLevel                  // Log (persistent) errors
	MandatoryLevel              // Log only mandatory events
	NeverLevel                  // Logging is disabled
)

var logLevel = [...]string{
	"     ",
	"TRACE",
	"DEBUG",
	"INFO ",
	"ERROR",
	">>>>>",
	"NEVER",
}

var strLevel = [...]string{
	"",
	"trace",
	"debug",
	"info",
	"error",
	"mandatory",
	"disabled",
}

// IsValid returns true, if the level is a valid level
func (l Level) IsValid() bool { return TraceLevel <= l && l <= NeverLevel }

func (l Level) String() string {
	if l.IsValid() {
		return strLevel[l]
	}
	return strconv.Itoa(int(l))
}

// Format returns a string representation suitable for logging.
func (l Level) Format() string {
	if l.IsValid() {
		return logLevel[l]
	}
	return strconv.Itoa(int(l))
}

// ParseLevel returns the recognized level.
func ParseLevel(text string) Level {
	for lv := TraceLevel; lv <= NeverLevel; lv++ {
		if len(text) > 2 && strings.HasPrefix(strLevel[lv], text) {
			return lv
		}
	}
	return NoLevel
}

// Logger represents an objects that emits logging messages.
type Logger struct {
	lw        LogWriter
	levelVal  uint32
	prefix    string
	context   []byte
	topParent *Logger
	uProvider UserProvider
}

// LogWriter writes log messages to their specified destinations.
type LogWriter interface {
	WriteMessage(level Level, ts time.Time, prefix, msg string, details []byte) error
}

// New creates a new logger for the given service.
//
// This function must only be called from a kernel implementation, not from
// code that tries to log something.
func New(lw LogWriter, prefix string) *Logger {
	if prefix != "" && len(prefix) < 6 {
		prefix = (prefix + "     ")[:6]
	}
	result := &Logger{
		lw:        lw,
		levelVal:  uint32(InfoLevel),
		prefix:    prefix,
		context:   nil,
		uProvider: nil,
	}
	result.topParent = result
	return result
}

func newFromMessage(msg *Message) *Logger {
	if msg == nil {
		return nil
	}
	logger := msg.logger
	context := make([]byte, 0, len(msg.buf))
	context = append(context, msg.buf...)
	return &Logger{
		lw:        nil,
		levelVal:  0,
		prefix:    logger.prefix,
		context:   context,
		topParent: logger.topParent,
		uProvider: nil,
	}
}

// SetLevel sets the level of the logger.
func (l *Logger) SetLevel(newLevel Level) *Logger {
	if l != nil {
		if l.topParent != l {
			panic("try to set level for child logger")
		}
		atomic.StoreUint32(&l.levelVal, uint32(newLevel))
	}
	return l
}

// Level returns the current level of the given logger
func (l *Logger) Level() Level {
	if l != nil {
		return Level(atomic.LoadUint32(&l.levelVal))
	}
	return NeverLevel
}

// Trace creates a tracing message.
func (l *Logger) Trace() *Message { return newMessage(l, TraceLevel) }

// Debug creates a debug message.
func (l *Logger) Debug() *Message { return newMessage(l, DebugLevel) }

// Info creates a message suitable for information data.
func (l *Logger) Info() *Message { return newMessage(l, InfoLevel) }

// Error creates a message suitable for errors.
func (l *Logger) Error() *Message { return newMessage(l, ErrorLevel) }

// Mandatory creates a message that will always logged, except when logging
// is disabled.
func (l *Logger) Mandatory() *Message { return newMessage(l, MandatoryLevel) }

// Clone creates a message to clone the logger.
func (l *Logger) Clone() *Message {
	msg := newMessage(l, NeverLevel)
	if msg != nil {
		msg.level = NoLevel
	}
	return msg
}

// UserProvider allows to retrieve an user metadata from a context.
type UserProvider interface {
	GetUser(ctx context.Context) *meta.Meta
}

// WithUser creates a derivied logger that allows to retrieve and log user identifer.
func (l *Logger) WithUser(up UserProvider) *Logger {
	return &Logger{
		lw:        nil,
		levelVal:  0,
		prefix:    l.prefix,
		context:   l.context,
		topParent: l.topParent,
		uProvider: up,
	}
}

func (l *Logger) writeMessage(level Level, msg string, details []byte) error {
	return l.topParent.lw.WriteMessage(level, time.Now().Local(), l.prefix, msg, details)
}
