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

package logger_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"zettelstore.de/z/logger"
)

func TestParseLevel(t *testing.T) {
	testcases := []struct {
		text string
		exp  logger.Level
	}{
		{"tra", logger.TraceLevel},
		{"deb", logger.DebugLevel},
		{"info", logger.InfoLevel},
		{"err", logger.ErrorLevel},
		{"manda", logger.MandatoryLevel},
		{"dis", logger.NeverLevel},
		{"d", logger.Level(0)},
	}
	for i, tc := range testcases {
		got := logger.ParseLevel(tc.text)
		if got != tc.exp {
			t.Errorf("%d: ParseLevel(%q) == %q, but got %q", i, tc.text, tc.exp, got)
		}
	}
}

func BenchmarkDisabled(b *testing.B) {
	log := logger.New(&stderrLogWriter{}, "").SetLevel(logger.NeverLevel)
	for b.Loop() {
		log.Info().Str("key", "val").Msg("Benchmark")
	}
}

type stderrLogWriter struct{}

func (*stderrLogWriter) WriteMessage(level logger.Level, ts time.Time, prefix, msg string, details []byte) error {
	fmt.Fprintf(os.Stderr, "%v %v %v %v %v\n", level.Format(), ts, prefix, msg, string(details))
	return nil
}

type testLogWriter struct{}

func (*testLogWriter) WriteMessage(logger.Level, time.Time, string, string, []byte) error {
	return nil
}

func BenchmarkStrMessage(b *testing.B) {
	log := logger.New(&testLogWriter{}, "")
	for b.Loop() {
		log.Info().Str("key", "val").Msg("Benchmark")
	}
}

func BenchmarkMessage(b *testing.B) {
	log := logger.New(&testLogWriter{}, "")
	for b.Loop() {
		log.Info().Msg("Benchmark")
	}
}

func BenchmarkCloneStrMessage(b *testing.B) {
	log := logger.New(&testLogWriter{}, "").Clone().Str("sss", "ttt").Child()
	for b.Loop() {
		log.Info().Msg("123456789")
	}
}
