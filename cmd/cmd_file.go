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

package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/encoder"
	"zettelstore.de/z/internal/parser"
	"zettelstore.de/z/internal/zettel"
)

// ---------- Subcommand: file -----------------------------------------------

func cmdFile(fs *flag.FlagSet) (int, error) {
	enc := fs.Lookup("t").Value.String()
	m, inp, err := getInput(fs.Args())
	if m == nil {
		return 2, err
	}
	z := parser.ParseZettel(
		context.Background(),
		zettel.Zettel{
			Meta:    m,
			Content: zettel.NewContent(inp.Src[inp.Pos:]),
		},
		string(m.GetDefault(meta.KeySyntax, meta.DefaultSyntax)),
		nil,
	)
	encdr := encoder.Create(
		api.Encoder(enc),
		&encoder.CreateParameter{Lang: string(m.GetDefault(meta.KeyLang, meta.ValueLangEN))})
	if encdr == nil {
		fmt.Fprintf(os.Stderr, "Unknown format %q\n", enc)
		return 2, nil
	}
	_, err = encdr.WriteZettel(os.Stdout, z)
	if err != nil {
		return 2, err
	}
	fmt.Println()

	return 0, nil
}

func getInput(args []string) (*meta.Meta, *input.Input, error) {
	if len(args) < 1 {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, nil, err
		}
		inp := input.NewInput(src)
		m := meta.NewFromInput(id.New(true), inp)
		return m, inp, nil
	}

	src, err := os.ReadFile(args[0])
	if err != nil {
		return nil, nil, err
	}
	inp := input.NewInput(src)
	m := meta.NewFromInput(id.New(true), inp)

	if len(args) > 1 {
		src, err = os.ReadFile(args[1])
		if err != nil {
			return nil, nil, err
		}
		inp = input.NewInput(src)
	}
	return m, inp, nil
}
