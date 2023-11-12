//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package tests

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"testing"

	"zettelstore.de/client.fossil/api"
	_ "zettelstore.de/z/cmd"
	"zettelstore.de/z/encoder"
	"zettelstore.de/z/input"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/zettel/meta"
)

// Test all parser / encoder with a list of "naughty strings", i.e. unusual strings
// that often crash software.

func getNaughtyStrings() (result []string, err error) {
	fpath := filepath.Join("..", "testdata", "naughty", "blns.txt")
	file, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
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
		e := encoder.Create(enc, &encoder.CreateParameter{Lang: api.ValueLangEN})
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
			bs := pinfo.ParseBlocks(input.NewInput([]byte(s)), &meta.Meta{}, pinfo.Name)
			is := pinfo.ParseInlines(input.NewInput([]byte(s)), pinfo.Name)
			for _, enc := range encs {
				_, err = enc.WriteBlocks(io.Discard, &bs)
				if err != nil {
					t.Error(err)
				}
				_, err = enc.WriteInlines(io.Discard, &is)
				if err != nil {
					t.Error(err)
				}
			}
		}
	}
}
