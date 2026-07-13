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

package encoder_test

import (
	"fmt"
	"strings"
	"testing"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/webapi"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/parser"
)

type zmkTestCase struct {
	descr     string
	src       string
	syntax    string
	allowHTML bool
	inline    bool
	expect    expectMap
}

type expectMap map[webapi.EncodingEnum]string

const useZmk = "\000"
const (
	encoderHTML  = webapi.EncoderHTML
	encoderMD    = webapi.EncoderMD
	encoderSz    = webapi.EncoderSz
	encoderSHTML = webapi.EncoderSHTML
	encoderText  = webapi.EncoderText
	encoderZmk   = webapi.EncoderZmk
)

func TestEncoder(t *testing.T) {
	for i := range tcsInline {
		tcsInline[i].inline = true
	}
	executeTestCases(t, append(append([]zmkTestCase{}, tcsBlock...), tcsInline...))
}

func executeTestCases(t *testing.T, testCases []zmkTestCase) {
	for testNum, tc := range testCases {
		syntax := tc.syntax
		if syntax == "" {
			syntax = meta.ValueSyntaxZmk
		}
		alst := sx.Nil()
		if tc.allowHTML {
			alst = alst.Cons(sx.Cons(parser.SymAllowHTML, nil))
		}
		pinfo := parser.Get(syntax)
		checkEncodings(t, testNum, tc, pinfo, nil, syntax, alst)
	}
}

func checkEncodings(t *testing.T, testNum int, tc zmkTestCase, pinfo *parser.Info, m *meta.Meta, syntax string, alst *sx.Pair) {
	for enc, exp := range tc.expect {
		inp := input.NewInput([]byte(tc.src))
		node := pinfo.Parse(inp, m, syntax, alst)

		encdr := encoder.Create(enc, &encoder.CreateParameter{Lang: meta.ValueLangEN})
		got, err := encode(encdr, node)
		if err != nil {
			prefix := fmt.Sprintf("Test #%d", testNum)
			if d := tc.descr; d != "" {
				prefix += "\nReason:   " + d
			}
			prefix += "\nMode:     " + mode(tc.inline)
			t.Errorf("%s\nEncoder:  %s\nError:    %v", prefix, enc, err)
			continue
		}
		if enc == encoderZmk && exp == useZmk {
			exp = tc.src
		}
		if got != exp {
			prefix := fmt.Sprintf("Test #%d", testNum)
			if d := tc.descr; d != "" {
				prefix += "\nReason:   " + d
			}
			prefix += "\nMode:     " + mode(tc.inline)
			t.Errorf("%s\nEncoder:  %s\nNode:     %v\nExpected: %q\nGot:      %q",
				prefix, enc, node, exp, got)
		}
	}
}

func encode(e encoder.Encoder, node *sx.Pair) (string, error) {
	var sb strings.Builder
	err := e.WriteSz(&sb, node)
	return sb.String(), err
}

func mode(isInline bool) string {
	if isInline {
		return "inline"
	}
	return "block"
}
