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

// Package encoder provides a generic interface to encode the abstract syntax
// tree into some text form.
package encoder

import (
	"fmt"
	"io"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsc/webapi"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/zettel"
)

// Encoder is an interface that allows to encode different parts of a zettel.
type Encoder interface {
	// WriteZettel encodes a whole zettel and writes it to the Writer.
	WriteZettel(io.Writer, *zettel.ParsedZettel) error

	// WriteMeta encodes just the metadata.
	WriteMeta(io.Writer, *meta.Meta) error

	// WriteSz encodes  SZ represented zettel content.
	WriteSz(io.Writer, *sx.Pair) error
}

// Create builds a new encoder with the given options.
func Create(enc webapi.EncodingEnum, params *CreateParameter) Encoder {
	switch enc {
	case webapi.EncoderHTML:
		// We need a new transformer every time, because tx.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &htmlEncoder{
			th:   shtml.NewEvaluator(1),
			lang: params.Lang,
		}
	case webapi.EncoderMD:
		return &mdEncoder{lang: params.Lang}
	case webapi.EncoderSHTML:
		// We need a new transformer every time, because tx.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &shtmlEncoder{
			th:   shtml.NewEvaluator(1),
			lang: params.Lang,
		}
	case webapi.EncoderSz:
		// We need a new transformer every time, because trans.inVerse must be unique.
		// If we can refactor it out, the transformer can be created only once.
		return &szEncoder{}
	case webapi.EncoderText:
		return (*TextEncoder)(nil)
	case webapi.EncoderZmk:
		return (*zmkEncoder)(nil)
	}
	return nil
}

// CreateParameter contains values that are needed to create some encoder.
type CreateParameter struct {
	Lang string // default language
}

// GetEncodings returns all registered encodings, ordered by encoding value.
func GetEncodings() []webapi.EncodingEnum {
	return []webapi.EncodingEnum{
		webapi.EncoderHTML, webapi.EncoderMD, webapi.EncoderSHTML,
		webapi.EncoderSz, webapi.EncoderText, webapi.EncoderZmk,
	}
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

// Function / Data types to retrieve table information, esp. proposed column alignment.

// getTableAlignments returns an alignment value for each column.
//
// If nodeHasPrio is true, a single noneAlignment will be stored, when there is at least
// one noneAlignment in the column. This is useful for zettelmarkup tables.
// Otherwise, the alignment value which is used the most, is stored in the
// column alignments.
func getTableAlignments(table *sx.Pair, noneHasPrio bool) columnAlignment {
	var tabAlign tableAlignment
	numColumns := 0
	_, headerRow, rows := zsx.GetTable(table)
	if headerRow != nil {
		rowAlignments := getRowAlignments(headerRow)
		numColumns = max(numColumns, len(rowAlignments))
		tabAlign = tableAlignment{rowAlignments}
	} else {
		tabAlign = tableAlignment{rowAlignment{}}
	}
	for row := range rows.Pairs() {
		rowAlignments := getRowAlignments(row.Head())
		numColumns = max(numColumns, len(rowAlignments))
		tabAlign = append(tabAlign, rowAlignments)
	}
	if numColumns == 0 {
		return nil
	}
	for i, row := range tabAlign {
		for len(row) < numColumns {
			row = append(row, extraAlignment)
		}
		tabAlign[i] = row
	}
	result := make(columnAlignment, numColumns)
	for col := range numColumns {
		align, countNone := tabAlign.calcColAlignment(col)
		if noneHasPrio && countNone > 0 {
			result[col] = noneAlignment
		} else {
			result[col] = align
		}
	}
	return result
}
func getRowAlignments(row *sx.Pair) rowAlignment {
	var result rowAlignment
	for cell := range row.Pairs() {
		attrs, _ := zsx.GetCell(cell.Head())
		result = append(result, getCellAlignment(attrs))
	}
	return result
}
func getCellAlignment(attrs *sx.Pair) cellAlignment {
	if alignPair := attrs.Assoc(zsx.SymAttrAlign); alignPair != nil {
		if alignValue := alignPair.Cdr(); zsx.AttrAlignCenter.IsEqual(alignValue) {
			return centerAlignment
		} else if zsx.AttrAlignLeft.IsEqual(alignValue) {
			return leftAlignment
		} else if zsx.AttrAlignRight.IsEqual(alignValue) {
			return rightAlignment
		}
	}
	return noneAlignment
}

type cellAlignment uint8

const (
	noneAlignment cellAlignment = iota
	leftAlignment
	centerAlignment
	rightAlignment
	extraAlignment // like noneAlignment, but added later
)

type rowAlignment []cellAlignment
type columnAlignment []cellAlignment
type tableAlignment []rowAlignment

func (ta tableAlignment) calcColAlignment(col int) (cellAlignment, int) {
	var freq [extraAlignment]int
	for _, row := range ta {
		if align := row[col]; align < extraAlignment {
			freq[align]++
		}
	}
	val, maxCount := noneAlignment, 0
	for i, c := range freq {
		if c > maxCount {
			val = cellAlignment(i)
			maxCount = c
		}
	}
	return val, freq[noneAlignment]
}
