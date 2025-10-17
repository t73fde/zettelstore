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

package evaluator

import (
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxbuiltins"
	"t73f.de/r/sx/sxreader"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx"
)

func evaluateSxn(block *sx.Pair) *sx.Pair {
	if blocks := zsx.GetBlock(block); blocks != nil {
		blk := blocks.Head()
		if sym, isSymbol := sx.GetSymbol(blk.Car()); isSymbol && zsx.SymVerbatimCode.IsEqualSymbol(sym) {
			_, attrs, content := zsx.GetVerbatim(blk)
			if classAttr, hasClass := zsx.GetAttributes(attrs).Get(""); hasClass && classAttr == meta.ValueSyntaxSxn {
				rd := sxreader.MakeReader(strings.NewReader(content))
				if objs, err := rd.ReadAll(); err == nil {
					var lb sx.ListBuilder
					for _, obj := range objs {
						var sb strings.Builder
						_, _ = sxbuiltins.Print(&sb, obj)
						lb.Add(zsx.MakeVerbatim(zsx.SymVerbatimCode, attrs, sb.String()))
					}
					return zsx.MakeBlockList(lb.List())
				}
			}
		}
	}
	return block
}
