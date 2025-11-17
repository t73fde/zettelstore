//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/parser"
)

type markdownTestCase struct {
	Markdown  string `json:"markdown"`
	HTML      string `json:"html"`
	Example   int    `json:"example"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Section   string `json:"section"`
}

func TestEncoderAvailability(t *testing.T) {
	t.Parallel()
	encoderMissing := false
	for _, enc := range encodings {
		enc := encoder.Create(enc, &encoder.CreateParameter{Lang: meta.ValueLangEN})
		if enc == nil {
			t.Errorf("No encoder for %q found", enc)
			encoderMissing = true
		}
	}
	if encoderMissing {
		panic("At least one encoder is missing. See test log")
	}
}

func TestMarkdownSpec(t *testing.T) {
	t.Parallel()
	content, err := os.ReadFile("../testdata/markdown/spec.json")
	if err != nil {
		panic(err)
	}
	var testcases []markdownTestCase
	if err = json.Unmarshal(content, &testcases); err != nil {
		panic(err)
	}

	for _, tc := range testcases {
		node := parseMD(tc.Markdown)
		testAllEncodings(t, tc, node)
		testZmkEncoding(t, tc, node)
	}
}

func parseMD(markdown string) *sx.Pair {
	return parser.Parse(input.NewInput([]byte(markdown)), nil, meta.ValueSyntaxMarkdown, nil)
}

func testAllEncodings(t *testing.T, tc markdownTestCase, node *sx.Pair) {
	var sb strings.Builder
	testID := tc.Example*100 + 1
	for _, enc := range encodings {
		t.Run(fmt.Sprintf("Encode %v %v", enc, testID), func(*testing.T) {
			_ = encoder.Create(enc, &encoder.CreateParameter{Lang: meta.ValueLangEN}).WriteSz(&sb, node)
			sb.Reset()
		})
	}
}

func testZmkEncoding(t *testing.T, tc markdownTestCase, node *sx.Pair) {
	zmkEncoder := encoder.Create(api.EncoderZmk, nil)
	var buf bytes.Buffer
	testID := tc.Example*100 + 1
	t.Run(fmt.Sprintf("Encode zmk %14d", testID), func(st *testing.T) {
		buf.Reset()
		_ = zmkEncoder.WriteSz(&buf, node)
		// gotFirst := buf.String()

		testID = tc.Example*100 + 2
		secondNode := parser.Parse(input.NewInput(buf.Bytes()), nil, meta.ValueSyntaxZmk, nil)
		buf.Reset()
		_ = zmkEncoder.WriteSz(&buf, secondNode)
		gotSecond := buf.String()

		// if gotFirst != gotSecond {
		// 	st.Errorf("\nCMD: %q\n1st: %q\n2nd: %q", tc.Markdown, gotFirst, gotSecond)
		// }

		testID = tc.Example*100 + 3
		thirdNode := parser.Parse(input.NewInput(buf.Bytes()), nil, meta.ValueSyntaxZmk, nil)
		buf.Reset()
		_ = zmkEncoder.WriteSz(&buf, thirdNode)
		gotThird := buf.String()

		if gotSecond != gotThird {
			st.Errorf("\ncmd: %q\nsz :%v\n1st: %q\n2nd: %q", tc.Markdown, node, gotSecond, gotThird)
		}
	})
}

func TestAdditionalMarkdown(t *testing.T) {
	testcases := []struct {
		md  string
		exp string
	}{
		{`abc<br>def`, "abc``<br>``{=\"html\"}def"},
	}
	zmkEncoder := encoder.Create(api.EncoderZmk, nil)
	var sb strings.Builder
	for i, tc := range testcases {
		node := parseMD(tc.md)
		sb.Reset()
		_ = zmkEncoder.WriteSz(&sb, node)
		if got := sb.String(); got != tc.exp {
			t.Errorf("%d: %q -> %q, but got %q", i, tc.md, tc.exp, got)
		}
	}
}
