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

package query_test

import (
	"testing"

	"zettelstore.de/z/internal/query"
)

func TestParser(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		spec string
		exp  string
	}{
		{"1", "1"}, // Just a number will transform to search for that number in all zettel

		{"1 IDENT", "00000000000001 IDENT"},
		{"IDENT", "IDENT"},
		{"1 IDENT|REINDEX", "00000000000001 IDENT | REINDEX"},

		{"1 ITEMS", "00000000000001 ITEMS"},
		{"ITEMS", "ITEMS"},

		{"CONTEXT", "CONTEXT"}, {"CONTEXT a", "CONTEXT a"},
		{"0 CONTEXT", "0 CONTEXT"}, {"1 CONTEXT", "00000000000001 CONTEXT"},
		{"1 CONTEXT CONTEXT", "00000000000001 CONTEXT CONTEXT"},
		{"00000000000001 CONTEXT", "00000000000001 CONTEXT"},
		{"100000000000001 CONTEXT", "100000000000001 CONTEXT"},
		{"1 CONTEXT FULL", "00000000000001 CONTEXT FULL"},
		{"1 CONTEXT BACKWARD", "00000000000001 CONTEXT BACKWARD"},
		{"1 CONTEXT FORWARD", "00000000000001 CONTEXT FORWARD"},
		{"1 CONTEXT COST ", "00000000000001 CONTEXT COST"},
		{"1 CONTEXT COST 3", "00000000000001 CONTEXT COST 3"}, {"1 CONTEXT COST x", "00000000000001 CONTEXT COST x"},
		{"1 CONTEXT MAX 5", "00000000000001 CONTEXT MAX 5"}, {"1 CONTEXT MAX y", "00000000000001 CONTEXT MAX y"},
		{"1 CONTEXT MIN 7", "00000000000001 CONTEXT MIN 7"}, {"1 CONTEXT MIN y", "00000000000001 CONTEXT MIN y"},
		{"1 CONTEXT MAX 5 COST 7", "00000000000001 CONTEXT COST 7 MAX 5"},
		{"1 CONTEXT |  N", "00000000000001 CONTEXT | N"},
		{"1 1 CONTEXT", "00000000000001 CONTEXT"},
		{"1 2 CONTEXT", "00000000000001 00000000000002 CONTEXT"},
		{"2 1 CONTEXT", "00000000000002 00000000000001 CONTEXT"},
		{"1 CONTEXT|N", "00000000000001 CONTEXT | N"},

		{"CONTEXT 0", "CONTEXT 0"},

		{"FOLGE", "FOLGE"}, {"FOLGE a", "FOLGE a"},
		{"0 FOLGE", "0 FOLGE"}, {"1 FOLGE", "00000000000001 FOLGE"},
		{"1 FOLGE FOLGE", "00000000000001 FOLGE FOLGE"},
		{"00000000000001 FOLGE", "00000000000001 FOLGE"},
		{"100000000000001 FOLGE", "100000000000001 FOLGE"},
		{"1 FOLGE BACKWARD", "00000000000001 FOLGE BACKWARD"},
		{"1 FOLGE FORWARD", "00000000000001 FOLGE FORWARD"},
		{"1 FOLGE MAX 5", "00000000000001 FOLGE MAX 5"}, {"1 FOLGE MAX y", "00000000000001 FOLGE MAX y"},
		{"1 FOLGE |  N", "00000000000001 FOLGE | N"},
		{"1 1 FOLGE", "00000000000001 FOLGE"},
		{"1 2 FOLGE", "00000000000001 00000000000002 FOLGE"},
		{"2 1 FOLGE", "00000000000002 00000000000001 FOLGE"},
		{"1 FOLGE|N", "00000000000001 FOLGE | N"},

		{"FOLGE 0", "FOLGE 0"},

		{"SEQUEL", "SEQUEL"}, {"SEQUEL a", "SEQUEL a"},
		{"0 SEQUEL", "0 SEQUEL"}, {"1 SEQUEL", "00000000000001 SEQUEL"},
		{"1 SEQUEL SEQUEL", "00000000000001 SEQUEL SEQUEL"},
		{"00000000000001 SEQUEL", "00000000000001 SEQUEL"},
		{"100000000000001 SEQUEL", "100000000000001 SEQUEL"},
		{"1 SEQUEL BACKWARD", "00000000000001 SEQUEL BACKWARD"},
		{"1 SEQUEL FORWARD", "00000000000001 SEQUEL FORWARD"},
		{"1 SEQUEL MAX 5", "00000000000001 SEQUEL MAX 5"}, {"1 SEQUEL MAX y", "00000000000001 SEQUEL MAX y"},
		{"1 SEQUEL |  N", "00000000000001 SEQUEL | N"},
		{"1 1 SEQUEL", "00000000000001 SEQUEL"},
		{"1 2 SEQUEL", "00000000000001 00000000000002 SEQUEL"},
		{"2 1 SEQUEL", "00000000000002 00000000000001 SEQUEL"},
		{"1 SEQUEL|N", "00000000000001 SEQUEL | N"},

		{"SEQUEL 0", "SEQUEL 0"},

		{"THREAD", "THREAD"}, {"THREAD a", "THREAD a"},
		{"0 THREAD", "0 THREAD"}, {"1 THREAD", "00000000000001 THREAD"},
		{"1 THREAD THREAD", "00000000000001 THREAD THREAD"},
		{"00000000000001 THREAD", "00000000000001 THREAD"},
		{"100000000000001 THREAD", "100000000000001 THREAD"},
		{"1 THREAD FULL", "00000000000001 THREAD FULL"},
		{"1 THREAD BACKWARD", "00000000000001 THREAD BACKWARD"},
		{"1 THREAD FORWARD", "00000000000001 THREAD FORWARD"},
		{"1 THREAD MAX 5", "00000000000001 THREAD MAX 5"}, {"1 THREAD MAX y", "00000000000001 THREAD MAX y"},
		{"1 THREAD |  N", "00000000000001 THREAD | N"},
		{"1 1 THREAD", "00000000000001 THREAD"},
		{"1 2 THREAD", "00000000000001 00000000000002 THREAD"},
		{"2 1 THREAD", "00000000000002 00000000000001 THREAD"},
		{"1 THREAD|N", "00000000000001 THREAD | N"},

		{"THREAD 0", "THREAD 0"},

		{"1 UNLINKED", "00000000000001 UNLINKED"},
		{"UNLINKED", "UNLINKED"},
		{"1 UNLINKED PHRASE", "00000000000001 UNLINKED PHRASE"},
		{"1 UNLINKED PHRASE Zettel", "00000000000001 UNLINKED PHRASE Zettel"},

		{"?", "?"}, {"!?", "!?"}, {"?a", "?a"}, {"!?a", "!?a"},
		{"key?", "key?"}, {"key!?", "key!?"},
		{"b key?", "key? b"}, {"b key!?", "key!? b"},
		{"key?a", "key?a"}, {"key!?a", "key!?a"},
		{"", ""}, {"!", ""}, {":", ""}, {"!:", ""}, {"[", ""}, {"![", ""}, {"]", ""}, {"!]", ""}, {"~", ""}, {"!~", ""}, {"<", ""}, {"!<", ""}, {">", ""}, {"!>", ""},
		{`a`, `a`}, {`!a`, `!a`},
		{`=a`, `=a`}, {`!=a`, `!=a`},
		{`:a`, `:a`}, {`!:a`, `!:a`},
		{`[a`, `[a`}, {`![a`, `![a`},
		{`]a`, `]a`}, {`!]a`, `!]a`},
		{`~a`, `a`}, {`!~a`, `!a`},
		{`key=`, `key=`}, {`key!=`, `key!=`},
		{`key:`, `key:`}, {`key!:`, `key!:`},
		{`key[`, `key[`}, {`key![`, `key![`},
		{`key]`, `key]`}, {`key!]`, `key!]`},
		{`key~`, `key~`}, {`key!~`, `key!~`},
		{`key<`, `key<`}, {`key!<`, `key!<`},
		{`key>`, `key>`}, {`key!>`, `key!>`},
		{`key=a`, `key=a`}, {`key!=a`, `key!=a`},
		{`key:a`, `key:a`}, {`key!:a`, `key!:a`},
		{`key[a`, `key[a`}, {`key![a`, `key![a`},
		{`key]a`, `key]a`}, {`key!]a`, `key!]a`},
		{`key~a`, `key~a`}, {`key!~a`, `key!~a`},
		{`key<a`, `key<a`}, {`key!<a`, `key!<a`},
		{`key>a`, `key>a`}, {`key!>a`, `key!>a`},
		{`key1:a key2:b`, `key1:a key2:b`},
		{`key1: key2:b`, `key1: key2:b`},
		{"word key:a", "key:a word"},
		{`PICK 3`, `PICK 3`}, {`PICK 9 PICK 11`, `PICK 9`},
		{"PICK a", "PICK a"},
		{`RANDOM`, `RANDOM`}, {`RANDOM a`, `a RANDOM`}, {`a RANDOM`, `a RANDOM`},
		{`RANDOM RANDOM a`, `a RANDOM`},
		{`RANDOMRANDOM a`, `RANDOMRANDOM a`}, {`a RANDOMRANDOM`, `a RANDOMRANDOM`},
		{`ORDER`, `ORDER`}, {"ORDER a b", "b ORDER a"}, {"a ORDER", "a ORDER"}, {"ORDER %", "ORDER %"},
		{"ORDER a %", "% ORDER a"},
		{"ORDER REVERSE", "ORDER REVERSE"}, {"ORDER REVERSE a b", "b ORDER REVERSE a"},
		{"a RANDOM ORDER b", "a ORDER b"}, {"a ORDER b RANDOM", "a ORDER b"},
		{"OFFSET", "OFFSET"}, {"OFFSET a", "OFFSET a"}, {"OFFSET 10 a", "a OFFSET 10"},
		{"OFFSET 01 a", "a OFFSET 1"}, {"OFFSET 0 a", "a"}, {"a OFFSET 0", "a"},
		{"OFFSET 4 OFFSET 8", "OFFSET 8"}, {"OFFSET 8 OFFSET 4", "OFFSET 8"},
		{"LIMIT", "LIMIT"}, {"LIMIT a", "LIMIT a"}, {"LIMIT 10 a", "a LIMIT 10"},
		{"LIMIT 01 a", "a LIMIT 1"}, {"LIMIT 0 a", "a"}, {"a LIMIT 0", "a"},
		{"LIMIT 4 LIMIT 8", "LIMIT 4"}, {"LIMIT 8 LIMIT 4", "LIMIT 4"},
		{"OR", ""}, {"OR OR", ""}, {"a OR", "a"}, {"OR b", "b"}, {"OR a OR", "a"},
		{"a OR b", "a OR b"},
		{"|", ""}, {" | RANDOM", "| RANDOM"}, {"| RANDOM", "| RANDOM"}, {"a|a b ", "a | a b"},
	}
	for i, tc := range testcases {
		got := query.Parse(tc.spec).String()
		if tc.exp != got {
			t.Errorf("%d: Parse(%q) does not yield %q, but got %q", i, tc.spec, tc.exp, got)
			continue
		}

		gotReparse := query.Parse(got).String()
		if gotReparse != got {
			t.Errorf("%d: Parse(%q) does not yield itself, but %q", i, got, gotReparse)
		}

		gotPipe := query.Parse(got + "|").String()
		if gotPipe != got {
			t.Errorf("%d: Parse(%q) does not yield itself, but %q", i, got+"|", gotReparse)
		}
	}
}
