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

// DLevel defines the possible log levels
type DLevel uint8

// Constants for Level
const (
	DNoLevel        DLevel = iota // the absent log level
	DTraceLevel                   // Log most internal activities
	DDebugLevel                   // Log most data updates
	DInfoLevel                    // Log normal activities
	DErrorLevel                   // Log (persistent) errors
	DMandatoryLevel               // Log only mandatory events
	DNeverLevel                   // Logging is disabled
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
func (l DLevel) IsValid() bool { return DTraceLevel <= l && l <= DNeverLevel }

func (l DLevel) String() string {
	if l.IsValid() {
		return strLevel[l]
	}
	return strconv.Itoa(int(l))
}

// Format returns a string representation suitable for logging.
func (l DLevel) Format() string {
	if l.IsValid() {
		return logLevel[l]
	}
	return strconv.Itoa(int(l))
}

// DParseLevel returns the recognized level.
func DParseLevel(text string) DLevel {
	for lv := DTraceLevel; lv <= DNeverLevel; lv++ {
		if len(text) > 2 && strings.HasPrefix(strLevel[lv], text) {
			return lv
		}
	}
	return DNoLevel
}

// DLogger represents an objects that emits logging messages.
type DLogger struct {
	lw        DLogWriter
	levelVal  uint32
	prefix    string
	context   []byte
	topParent *DLogger
	uProvider DUserProvider
}

// DLogWriter writes log messages to their specified destinations.
type DLogWriter interface {
	DWriteMessage(level DLevel, ts time.Time, prefix, msg string, details []byte) error
}

// DNew creates a new logger for the given service.
//
// This function must only be called from a kernel implementation, not from
// code that tries to log something.
func DNew(lw DLogWriter, prefix string) *DLogger {
	if prefix != "" && len(prefix) < 6 {
		prefix = (prefix + "     ")[:6]
	}
	result := &DLogger{
		lw:        lw,
		levelVal:  uint32(DInfoLevel),
		prefix:    prefix,
		context:   nil,
		uProvider: nil,
	}
	result.topParent = result
	return result
}

func dnewFromDMessage(msg *DMessage) *DLogger {
	if msg == nil {
		return nil
	}
	logger := msg.logger
	context := make([]byte, 0, len(msg.buf))
	context = append(context, msg.buf...)
	return &DLogger{
		lw:        nil,
		levelVal:  0,
		prefix:    logger.prefix,
		context:   context,
		topParent: logger.topParent,
		uProvider: nil,
	}
}

// SetLevel sets the level of the logger.
func (l *DLogger) SetLevel(newLevel DLevel) *DLogger {
	if l != nil {
		if l.topParent != l {
			panic("try to set level for child logger")
		}
		atomic.StoreUint32(&l.levelVal, uint32(newLevel))
	}
	return l
}

// Level returns the current level of the given logger
func (l *DLogger) Level() DLevel {
	if l != nil {
		return DLevel(atomic.LoadUint32(&l.levelVal))
	}
	return DNeverLevel
}

// Trace creates a tracing message.
func (l *DLogger) Trace() *DMessage { return dnewDMessage(l, DTraceLevel) }

// Debug creates a debug message.
func (l *DLogger) Debug() *DMessage { return dnewDMessage(l, DDebugLevel) }

// Info creates a message suitable for information data.
func (l *DLogger) Info() *DMessage { return dnewDMessage(l, DInfoLevel) }

// Error creates a message suitable for errors.
func (l *DLogger) Error() *DMessage { return dnewDMessage(l, DErrorLevel) }

// Mandatory creates a message that will always logged, except when logging
// is disabled.
func (l *DLogger) Mandatory() *DMessage { return dnewDMessage(l, DMandatoryLevel) }

// Clone creates a message to clone the logger.
func (l *DLogger) Clone() *DMessage {
	msg := dnewDMessage(l, DNeverLevel)
	if msg != nil {
		msg.level = DNoLevel
	}
	return msg
}

// DUserProvider allows to retrieve an user metadata from a context.
type DUserProvider interface {
	GetUser(ctx context.Context) *meta.Meta
}

// WithUser creates a derivied logger that allows to retrieve and log user identifer.
func (l *DLogger) WithUser(up DUserProvider) *DLogger {
	return &DLogger{
		lw:        nil,
		levelVal:  0,
		prefix:    l.prefix,
		context:   l.context,
		topParent: l.topParent,
		uProvider: up,
	}
}

func (l *DLogger) dwriteDMessage(level DLevel, msg string, details []byte) error {
	return l.topParent.lw.DWriteMessage(level, time.Now().Local(), l.prefix, msg, details)
}
