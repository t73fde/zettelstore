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
	"t73f.de/r/sx/sxreader"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/parser"
)

type zmkTestCase struct {
	descr  string
	zmk    string
	syntax string
	inline bool
	expect expectMap
}

type expectMap map[api.EncodingEnum]string

const useZmk = "\000"
const (
	encoderHTML  = api.EncoderHTML
	encoderMD    = api.EncoderMD
	encoderSz    = api.EncoderSz
	encoderSHTML = api.EncoderSHTML
	encoderText  = api.EncoderText
	encoderZmk   = api.EncoderZmk
)

func TestEncoder(t *testing.T) {
	for i := range tcsInline {
		tcsInline[i].inline = true
	}
	executeTestCases(t, append(append([]zmkTestCase{}, tcsBlock...), tcsInline...))
}

func executeTestCases(t *testing.T, testCases []zmkTestCase) {
	for testNum, tc := range testCases {
		inp := input.NewInput([]byte(tc.zmk))
		syntax := tc.syntax
		if syntax == "" {
			syntax = meta.ValueSyntaxZmk
		}
		node, bs := parser.Parse(inp, nil, syntax)
		parser.Clean(node, false)
		parser.CleanAST(&bs, false)
		checkEncodings(t, testNum, node, tc.inline, tc.descr, tc.expect, tc.zmk)
		checkEncodingsAST(t, testNum+1000, bs, tc.inline, tc.descr, tc.expect, tc.zmk)
		checkSz(t, testNum, bs, tc.inline, tc.descr)
	}
}

func checkEncodings(t *testing.T, testNum int, node *sx.Pair, isInline bool, descr string, expected expectMap, zmkDefault string) {
	for enc, exp := range expected {
		encdr := encoder.Create(enc, &encoder.CreateParameter{Lang: meta.ValueLangEN})
		got, err := encode(encdr, node)
		if err != nil {
			prefix := fmt.Sprintf("Test #%d", testNum)
			if d := descr; d != "" {
				prefix += "\nReason:   " + d
			}
			prefix += "\nMode:     " + mode(isInline)
			t.Errorf("%s\nEncoder:  %s\nError:    %v", prefix, enc, err)
			continue
		}
		if enc == api.EncoderZmk && exp == useZmk {
			exp = zmkDefault
		}
		if got != exp {
			prefix := fmt.Sprintf("Test #%d", testNum)
			if d := descr; d != "" {
				prefix += "\nReason:   " + d
			}
			prefix += "\nMode:     " + mode(isInline)
			t.Errorf("%s\nEncoder:  %s\nExpected: %q\nGot:      %q", prefix, enc, exp, got)
		}
	}
}
func checkEncodingsAST(t *testing.T, testNum int, bs ast.BlockSlice, isInline bool, descr string, expected expectMap, zmkDefault string) {
	for enc, exp := range expected {
		encdr := encoder.Create(enc, &encoder.CreateParameter{Lang: meta.ValueLangEN})
		got, err := encodeAST(encdr, bs)
		if err != nil {
			prefix := fmt.Sprintf("Test #%d", testNum)
			if d := descr; d != "" {
				prefix += "\nReason:   " + d
			}
			prefix += "\nMode:     " + mode(isInline)
			t.Errorf("%s\nEncoder:  %s\nError:    %v", prefix, enc, err)
			continue
		}
		if enc == api.EncoderZmk && exp == useZmk {
			exp = zmkDefault
		}
		if got != exp {
			prefix := fmt.Sprintf("Test #%d", testNum)
			if d := descr; d != "" {
				prefix += "\nReason:   " + d
			}
			prefix += "\nMode:     " + mode(isInline)
			t.Errorf("%s\nEncoder:  %s\nExpected: %q\nGot:      %q", prefix, enc, exp, got)
		}
	}
}

func checkSz(t *testing.T, testNum int, bs ast.BlockSlice, isInline bool, descr string) {
	t.Helper()
	encdr := encoder.Create(encoderSz, nil)
	exp, err := encodeAST(encdr, bs)
	if err != nil {
		t.Error(err)
		return
	}
	val, err := sxreader.MakeReader(strings.NewReader(exp)).Read()
	if err != nil {
		t.Error(err)
		return
	}
	if got := val.String(); exp != got {
		prefix := fmt.Sprintf("Test #%d", testNum)
		if d := descr; d != "" {
			prefix += "\nReason:   " + d
		}
		prefix += "\nMode:     " + mode(isInline)
		t.Errorf("%s\n\nExpected: %q\nGot:      %q", prefix, exp, got)
	}
}

func encode(e encoder.Encoder, node *sx.Pair) (string, error) {
	var sb strings.Builder
	err := e.WriteSz(&sb, node)
	return sb.String(), err
}
func encodeAST(e encoder.Encoder, bs ast.BlockSlice) (string, error) {
	var sb strings.Builder
	err := e.WriteBlocks(&sb, &bs)
	return sb.String(), err
}

func mode(isInline bool) string {
	if isInline {
		return "inline"
	}
	return "block"
}
