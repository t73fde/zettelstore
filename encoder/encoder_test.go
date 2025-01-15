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
	"t73f.de/r/zsc/input"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/config"
	"zettelstore.de/z/encoder"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/zettel/meta"

	_ "zettelstore.de/z/encoder/htmlenc"   // Allow to use HTML encoder.
	_ "zettelstore.de/z/encoder/mdenc"     // Allow to use markdown encoder.
	_ "zettelstore.de/z/encoder/shtmlenc"  // Allow to use SHTML encoder.
	_ "zettelstore.de/z/encoder/szenc"     // Allow to use sz encoder.
	_ "zettelstore.de/z/encoder/textenc"   // Allow to use text encoder.
	_ "zettelstore.de/z/encoder/zmkenc"    // Allow to use zmk encoder.
	_ "zettelstore.de/z/parser/zettelmark" // Allow to use zettelmark parser.
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
		bs := parser.ParseBlocks(inp, nil, meta.SyntaxZmk, config.NoHTML)
		var pe parserEncoder
		if tc.inline {
			var is ast.InlineSlice
			if len(bs) > 0 {
				if pn, ok := bs[0].(*ast.ParaNode); ok {
					is = pn.Inlines
				}
			}
			pe = &peInlines{is: is}
		} else {
			pe = &peBlocks{bs: bs}
		}
		checkEncodings(t, testNum, pe, tc.descr, tc.expect, tc.zmk)
		checkSz(t, testNum, pe, tc.descr)
	}
}

func checkEncodings(t *testing.T, testNum int, pe parserEncoder, descr string, expected expectMap, zmkDefault string) {
	for enc, exp := range expected {
		encdr := encoder.Create(enc, &encoder.CreateParameter{Lang: api.ValueLangEN})
		got, err := pe.encode(encdr)
		if err != nil {
			prefix := fmt.Sprintf("Test #%d", testNum)
			if d := descr; d != "" {
				prefix += "\nReason:   " + d
			}
			prefix += "\nMode:     " + pe.mode()
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
			prefix += "\nMode:     " + pe.mode()
			t.Errorf("%s\nEncoder:  %s\nExpected: %q\nGot:      %q", prefix, enc, exp, got)
		}
	}
}

func checkSz(t *testing.T, testNum int, pe parserEncoder, descr string) {
	t.Helper()
	encdr := encoder.Create(encoderSz, nil)
	exp, err := pe.encode(encdr)
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
		prefix += "\nMode:     " + pe.mode()
		t.Errorf("%s\n\nExpected: %q\nGot:      %q", prefix, exp, got)
	}
}

type parserEncoder interface {
	encode(encoder.Encoder) (string, error)
	mode() string
}

type peInlines struct {
	is ast.InlineSlice
}

func (in peInlines) encode(encdr encoder.Encoder) (string, error) {
	var sb strings.Builder
	if _, err := encdr.WriteInlines(&sb, &in.is); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func (peInlines) mode() string { return "inline" }

type peBlocks struct {
	bs ast.BlockSlice
}

func (bl peBlocks) encode(encdr encoder.Encoder) (string, error) {
	var sb strings.Builder
	if _, err := encdr.WriteBlocks(&sb, &bl.bs); err != nil {
		return "", err
	}
	return sb.String(), nil

}
func (peBlocks) mode() string { return "block" }
