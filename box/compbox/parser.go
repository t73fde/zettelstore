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

package compbox

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/parser"
)

func genParserM(zid id.Zid) *meta.Meta {
	m := getTitledMeta(zid, "Zettelstore Supported Parser")
	m.Set(meta.KeyCreated, meta.Value(kernel.Main.GetConfig(kernel.CoreService, kernel.CoreVTime).(string)))
	m.Set(meta.KeyVisibility, meta.ValueVisibilityLogin)
	return m
}

func genParserC(context.Context, *compBox) []byte {
	var buf bytes.Buffer
	buf.WriteString("|=Syntax<|=Alt. Value(s):|=Text Parser?:|=Text Format?:|=Image Format?:\n")
	syntaxes := parser.GetSyntaxes()
	slices.Sort(syntaxes)
	for _, syntax := range syntaxes {
		info := parser.Get(syntax)
		if info.Name != syntax {
			continue
		}
		altNames := info.AltNames
		slices.Sort(altNames)
		fmt.Fprintf(
			&buf, "|%v|%v|%v|%v|%v\n",
			syntax, strings.Join(altNames, ", "), info.IsASTParser, info.IsTextFormat, info.IsImageFormat)
	}
	return buf.Bytes()
}
