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

// Package domain provides some zettel domain specific definitions.
package domain

import (
	"fmt"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"

	"zettelstore.de/z/internal/zettel"
)

// Zettel is the root node of the abstract syntax tree.
type Zettel struct {
	Meta    *meta.Meta     // Original metadata
	Content zettel.Content // Original content
	Zid     id.Zid         // Zettel identification.
	InhMeta *meta.Meta     // Metadata of the zettel, with inherited values.
	Blocks  *sx.Pair       // Syntax tree, encodes as an sx.Object.
	Syntax  string         // Syntax / parser that produced the Ast
}

var mapMetaTypeS = map[*meta.DescriptionType]*sx.Symbol{
	meta.TypeCredential: sz.SymTypeCredential,
	meta.TypeEmpty:      sz.SymTypeEmpty,
	meta.TypeID:         sz.SymTypeID,
	meta.TypeIDSet:      sz.SymTypeIDSet,
	meta.TypeNumber:     sz.SymTypeNumber,
	meta.TypeString:     sz.SymTypeString,
	meta.TypeTagSet:     sz.SymTypeTagSet,
	meta.TypeTimestamp:  sz.SymTypeTimestamp,
	meta.TypeURL:        sz.SymTypeURL,
	meta.TypeWord:       sz.SymTypeWord,
}

// GetMetaSz transforms the given metadata into a sz list.
func GetMetaSz(m *meta.Meta) *sx.Pair {
	var lb sx.ListBuilder
	lb.Add(sz.SymMeta)
	for key, val := range m.Computed() {
		ty := m.Type(key)
		symType, found := mapMetaTypeS[ty]
		if !found {
			symType = sx.MakeSymbol(fmt.Sprintf("**%v:NOT-FOUND**", ty))
		}
		var obj sx.Object
		if ty.IsSet {
			var setObjs sx.ListBuilder
			for _, val := range val.AsSlice() {
				setObjs.Add(sx.MakeString(val))
			}
			obj = setObjs.List()
		} else {
			obj = sx.MakeString(string(val))
		}
		lb.Add(sx.MakeList(symType, sx.MakeSymbol(key), obj))
	}
	return lb.List()
}
