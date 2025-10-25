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

// Package collect provides functions to collect items from a syntax tree.
package collect

import (
	"t73f.de/r/sx"
	"t73f.de/r/zsx"
)

// Order returns links in the items of the first list found in the given block node.
func Order(block *sx.Pair) *sx.Pair {
	var lb sx.ListBuilder
	blocks := zsx.GetBlock(block)
	for bn := range blocks.Pairs() {
		blk := bn.Head()
		if sym, isSymbol := sx.GetSymbol(blk.Car()); isSymbol {
			if zsx.SymListUnordered.IsEqualSymbol(sym) || zsx.SymListOrdered.IsEqualSymbol(sym) {
				_, _, items := zsx.GetList(blk)
				for item := range items.Pairs() {
					if ln := firstItemZettelLink(item.Head()); ln != nil {
						lb.Add(ln)
					}
				}
			}
		}
	}
	return lb.List()
}

func firstItemZettelLink(item *sx.Pair) *sx.Pair {
	blocks := zsx.GetBlock(item)
	for bn := range blocks.Pairs() {
		blk := bn.Head()
		if sym, isSymbol := sx.GetSymbol(blk.Car()); isSymbol && zsx.SymPara.IsEqualSymbol(sym) {
			inlines := zsx.GetPara(blk)
			if ln := firstInlineZettelLink(inlines); ln != nil {
				return ln
			}
		}
	}
	return nil
}

func firstInlineZettelLink(inlines *sx.Pair) (result *sx.Pair) {
	for inode := range inlines.Pairs() {
		inl := inode.Head()
		if sym, isSymbol := sx.GetSymbol(inl.Car()); isSymbol {
			switch sym {
			case zsx.SymLink:
				return inl
			case zsx.SymFormatDelete, zsx.SymFormatEmph, zsx.SymFormatInsert, zsx.SymFormatMark,
				zsx.SymFormatQuote, zsx.SymFormatSpan, zsx.SymFormatStrong, zsx.SymFormatSub,
				zsx.SymFormatSuper:
				_, _, finlines := zsx.GetFormat(inl)
				result = firstInlineZettelLink(finlines)
			case zsx.SymCite:
				_, _, cinlines := zsx.GetCite(inl)
				result = firstInlineZettelLink(cinlines)
			case zsx.SymEmbed:
				_, _, _, einlines := zsx.GetEmbed(inl)
				result = firstInlineZettelLink(einlines)
			case zsx.SymEmbedBLOB:
				_, _, _, binlines := zsx.GetEmbedBLOBuncode(inl)
				result = firstInlineZettelLink(binlines)
			}
			if result != nil {
				return result
			}
		}
	}
	return nil
}
