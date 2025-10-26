//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package sztrans

import (
	"fmt"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
)

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
		symType := mapGetS(mapMetaTypeS, ty)
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
		lb.Add(sx.Nil().Cons(obj).Cons(sx.MakeSymbol(key)).Cons(symType))
	}
	return lb.List()
}

func mapGetS[T comparable](m map[T]*sx.Symbol, k T) *sx.Symbol {
	if result, found := m[k]; found {
		return result
	}
	return sx.MakeSymbol(fmt.Sprintf("**%v:NOT-FOUND**", k))
}
