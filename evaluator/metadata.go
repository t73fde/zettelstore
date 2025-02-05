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
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/ast"
)

func evaluateMetadata(m *meta.Meta) ast.BlockSlice {
	descrlist := &ast.DescriptionListNode{}
	for _, p := range m.Pairs() {
		descrlist.Descriptions = append(
			descrlist.Descriptions, getMetadataDescription(p.Key, p.Value))
	}
	return ast.BlockSlice{descrlist}
}

func getMetadataDescription(key string, value meta.Value) ast.Description {
	is := convertMetavalueToInlineSlice(value, meta.Type(key))
	return ast.Description{
		Term:         ast.InlineSlice{&ast.TextNode{Text: key}},
		Descriptions: []ast.DescriptionSlice{{&ast.ParaNode{Inlines: is}}},
	}
}

func convertMetavalueToInlineSlice(value meta.Value, dt *meta.DescriptionType) ast.InlineSlice {
	var sliceData []string
	if dt.IsSet {
		sliceData = value.ListFromValue()
		if len(sliceData) == 0 {
			return nil
		}
	} else {
		sliceData = []string{string(value)}
	}
	makeLink := dt == meta.TypeID || dt == meta.TypeIDSet

	result := make(ast.InlineSlice, 0, 2*len(sliceData)-1)
	for i, val := range sliceData {
		if i > 0 {
			result = append(result, &ast.TextNode{Text: " "})
		}
		tn := &ast.TextNode{Text: val}
		if makeLink {
			result = append(result, &ast.LinkNode{
				Ref:     ast.ParseReference(val),
				Inlines: ast.InlineSlice{tn},
			})
		} else {
			result = append(result, tn)
		}
	}
	return result
}
