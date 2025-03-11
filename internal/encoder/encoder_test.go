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

	"t73f.de/r/sx/sxreader"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/parser"

	_ "zettelstore.de/z/internal/parser/zettelmark" // Allow to use zettelmark parser.
)

type zmkTestCase struct {
	descr  string
	zmk    string
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
	executeTestCases(t, append(tcsBlock, tcsInline...))
}

func executeTestCases(t *testing.T, testCases []zmkTestCase) {
	for testNum, tc := range testCases {
		inp := input.NewInput([]byte(tc.zmk))
		bs := parser.ParseBlocks(inp, nil, meta.ValueSyntaxZmk, config.NoHTML)
		checkEncodings(t, testNum, bs, tc.inline, tc.descr, tc.expect, tc.zmk)
		checkSz(t, testNum, bs, tc.inline, tc.descr)
	}
}

func checkEncodings(t *testing.T, testNum int, bs ast.BlockSlice, isInline bool, descr string, expected expectMap, zmkDefault string) {
	for enc, exp := range expected {
		encdr := encoder.Create(enc, &encoder.CreateParameter{Lang: meta.ValueLangEN})
		got, err := encode(encdr, bs, isInline)
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
	exp, err := encode(encdr, bs, isInline)
	if err != nil {
		t.Error(err)
		return
	}
	val, err := sxreader.MakeReader(strings.NewReader(exp)).Read()
	if err != nil {
		t.Error(err)
		return
	}
	got := val.String()
	if exp != got {
		prefix := fmt.Sprintf("Test #%d", testNum)
		if d := descr; d != "" {
			prefix += "\nReason:   " + d
		}
		prefix += "\nMode:     " + mode(isInline)
		t.Errorf("%s\n\nExpected: %q\nGot:      %q", prefix, exp, got)
	}
}

func encode(e encoder.Encoder, bs ast.BlockSlice, isInline bool) (string, error) {
	var sb strings.Builder
	if !isInline {
		_, err := e.WriteBlocks(&sb, &bs)
		return sb.String(), err
	}
	var is ast.InlineSlice
	if len(bs) > 0 {
		if pn, ok := bs[0].(*ast.ParaNode); ok {
			is = pn.Inlines
		}
	}
	_, err := e.WriteInlines(&sb, &is)
	return sb.String(), err
}

func mode(isInline bool) string {
	if isInline {
		return "inline"
	}
	return "block"
}
