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
	"bufio"
	"io"
	"os"
	"path/filepath"
	"testing"

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/parser"

	_ "zettelstore.de/z/cmd"
)

// Test all parser / encoder with a list of "naughty strings", i.e. unusual strings
// that often crash software.

func getNaughtyStrings() (result []string, err error) {
	fpath := filepath.Join("..", "testdata", "naughty", "blns.txt")
	file, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if text := scanner.Text(); text != "" && text[0] != '#' {
			result = append(result, text)
		}
	}
	return result, scanner.Err()
}

func getAllParser() (result []*parser.Info) {
	for _, pname := range parser.GetSyntaxes() {
		pinfo := parser.Get(pname)
		if pname == pinfo.Name {
			result = append(result, pinfo)
		}
	}
	return result
}

func getAllEncoder() (result []encoder.Encoder) {
	for _, enc := range encoder.GetEncodings() {
		e := encoder.Create(enc, &encoder.CreateParameter{Lang: meta.ValueLangEN})
		result = append(result, e)
	}
	return result
}

func TestNaughtyStringParser(t *testing.T) {
	blns, err := getNaughtyStrings()
	if err != nil {
		t.Fatal(err)
	}
	if len(blns) == 0 {
		t.Fatal("no naughty strings found")
	}
	pinfos := getAllParser()
	if len(pinfos) == 0 {
		t.Fatal("no parser found")
	}
	encs := getAllEncoder()
	if len(encs) == 0 {
		t.Fatal("no encoder found")
	}
	for _, s := range blns {
		for _, pinfo := range pinfos {
			node, bs := parser.Parse(input.NewInput([]byte(s)), &meta.Meta{}, pinfo.Name, config.NoHTML)
			for _, enc := range encs {
				if err = enc.WriteSz(io.Discard, node); err != nil {
					t.Error(err)
				}
				if err = enc.WriteBlocks(io.Discard, &bs); err != nil {
					t.Error(err)
				}
			}
		}
	}
}
