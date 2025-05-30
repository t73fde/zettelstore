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
)

const (
	LevelMissing   slog.Level = -9999
	LevelTrace     slog.Level = -8
	LevelMandatory slog.Level = 9999
)

func LevelString(level slog.Level) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case LevelMandatory:
		return ">>>>>"
	// case slog.LevelInfo:
	// 	return "INFO "
	default:
		s := level.String()
		if len(s) < 5 {
			s = s + "     "[0:5-len(s)]
		}
		return s
	}

}

func LogTrace(logger *slog.Logger, msg string, args ...any) {
	logger.Log(context.Background(), LevelTrace, msg, args...)
}

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
