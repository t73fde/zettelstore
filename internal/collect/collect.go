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

// Package collect provides functions to collect items from a syntax tree.
package collect

import (
	"iter"

	"t73f.de/r/sx"
	"t73f.de/r/zsx"
)

type refYielder struct {
	yield func(*sx.Pair) bool
	stop  bool
}

// ReferenceSeq returns an iterator of all references mentioned in the given
// block slice. This also includes references to images.
func ReferenceSeq(block *sx.Pair) iter.Seq[*sx.Pair] {
	return func(yield func(*sx.Pair) bool) {
		yielder := refYielder{yield, false}
		zsx.WalkIt(&yielder, block, nil)
	}
}

// Visit all node to collect data for the summary.
func (y *refYielder) VisitBefore(node *sx.Pair, _ *sx.Pair) bool {
	if y.stop {
		return true
	}
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymLink:
			_, ref, _ := zsx.GetLink(node)
			y.stop = !y.yield(ref)

		case zsx.SymEmbed:
			_, ref, _, _ := zsx.GetEmbed(node)
			y.stop = !y.yield(ref)

		case zsx.SymTransclude:
			_, ref, _ := zsx.GetTransclusion(node)
			y.stop = !y.yield(ref)
		}
		if y.stop {
			return true
		}
	}
	return false
}
func (*refYielder) VisitAfter(*sx.Pair, *sx.Pair) {}
