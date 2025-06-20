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

// Package logging provides some definitions to adapt package slog to Zettelstore needs
package logging

import (
	"context"
	"log/slog"
	"strings"

	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth/user"
)

// Some additional log levels.
const (
	LevelMissing   slog.Level = -9999
	LevelTrace     slog.Level = -8
	LevelMandatory slog.Level = 9999
)

// LevelString returns a string naming the level.
func LevelString(level slog.Level) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case LevelMandatory:
		return ">>>>>"
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelError:
		return "ERROR"
	default:
		return level.String()
	}
}

// LevelStringPad returns a string naming the level. The string is a least 5 bytes long.
func LevelStringPad(level slog.Level) string {
	s := LevelString(level)
	if len(s) < 5 {
		s = s + "     "[0:5-len(s)]
	}
	return s
}

// LogTrace writes a trace log message.
func LogTrace(logger *slog.Logger, msg string, args ...any) {
	logger.Log(context.Background(), LevelTrace, msg, args...)
}

// LogMandatory writes a mandatory log message.
func LogMandatory(logger *slog.Logger, msg string, args ...any) {
	logger.Log(context.Background(), LevelMandatory, msg, args...)
}

// ParseLevel returns the recognized level.
func ParseLevel(text string) slog.Level {
	switch strings.ToUpper(text) {
	case "TR", "TRA", "TRAC", "TRACE":
		return LevelTrace
	case "DE", "DEB", "DEBU", "DEBUG":
		return slog.LevelDebug
	case "IN", "INF", "INFO":
		return slog.LevelInfo
	case "WA", "WAR", "WARN":
		return slog.LevelWarn
	case "ER", "ERR", "ERRO", "ERROR":
		return slog.LevelError
	}
	return LevelMissing
}

// Err returns a log attribute, if an error occurred.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.Any("err", err)
}

// User returns a log attribute indicating the currently user.
func User(ctx context.Context) slog.Attr {
	if um := user.GetCurrentUser(ctx); um != nil {
		if userID, found := um.Get(meta.KeyUserID); found {
			return slog.Any("user", userID)
		}
		return slog.String("user", um.Zid.String())
	}
	return slog.Attr{}
}
