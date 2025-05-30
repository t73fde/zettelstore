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

	"zettelstore.de/z/internal/logger"
)

func TestDParseLevel(t *testing.T) {
	testcases := []struct {
		text string
		exp  logger.DLevel
	}{
		{"tra", logger.DTraceLevel},
		{"deb", logger.DDebugLevel},
		{"info", logger.DInfoLevel},
		{"err", logger.DErrorLevel},
		{"manda", logger.DMandatoryLevel},
		{"dis", logger.DNeverLevel},
		{"d", logger.DLevel(0)},
	}
	for i, tc := range testcases {
		got := logger.DParseLevel(tc.text)
		if got != tc.exp {
			t.Errorf("%d: ParseLevel(%q) == %q, but got %q", i, tc.text, tc.exp, got)
		}
	}
}

func BenchmarkDDisabled(b *testing.B) {
	log := logger.DNew(&stderrLogWriter{}, "").SetLevel(logger.DNeverLevel)
	for b.Loop() {
		log.Info().Str("key", "val").Msg("Benchmark")
	}
}

type stderrLogWriter struct{}

func (*stderrLogWriter) DWriteMessage(level logger.DLevel, ts time.Time, prefix, msg string, details []byte) error {
	fmt.Fprintf(os.Stderr, "%v %v %v %v %v\n", level.Format(), ts, prefix, msg, string(details))
	return nil
}

type testLogWriter struct{}

func (*testLogWriter) DWriteMessage(logger.DLevel, time.Time, string, string, []byte) error {
	return nil
}

func BenchmarkDStrMessage(b *testing.B) {
	log := logger.DNew(&testLogWriter{}, "")
	for b.Loop() {
		log.Info().Str("key", "val").Msg("Benchmark")
	}
}

func BenchmarkDMessage(b *testing.B) {
	log := logger.DNew(&testLogWriter{}, "")
	for b.Loop() {
		log.Info().Msg("Benchmark")
	}
}

func BenchmarkDCloneStrMessage(b *testing.B) {
	log := logger.DNew(&testLogWriter{}, "").Clone().Str("sss", "ttt").Child()
	for b.Loop() {
		log.Info().Msg("123456789")
	}
}
