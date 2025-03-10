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

package query

import (
	"cmp"
	"strconv"

	"t73f.de/r/zsc/domain/meta"
)

type sortFunc func(i, j *meta.Meta) int

func buildSortFunc(order []sortOrder) sortFunc {
	hasID := false
	sortFuncs := make([]sortFunc, 0, len(order)+1)
	for _, o := range order {
		sortFuncs = append(sortFuncs, o.buildSortfunc())
		if o.key == meta.KeyID {
			hasID = true
			break
		}
	}
	if !hasID {
		sortFuncs = append(sortFuncs, defaultMetaSort)
	}
	if len(sortFuncs) == 1 {
		return sortFuncs[0]
	}
	return func(i, j *meta.Meta) int {
		for _, sf := range sortFuncs {
			if result := sf(i, j); result != 0 {
				return result
			}
		}
		return 0
	}
}

func (so *sortOrder) buildSortfunc() sortFunc {
	key := so.key
	keyType := meta.Type(key)
	if key == meta.KeyID || keyType == meta.TypeCredential {
		if so.descending {
			return defaultMetaSort
		}
		return func(i, j *meta.Meta) int { return cmp.Compare(i.Zid, j.Zid) }
	}
	if keyType == meta.TypeTimestamp {
		return createSortTimestampFunc(key, so.descending)
	}
	if keyType == meta.TypeNumber {
		return createSortNumberFunc(key, so.descending)
	}
	return createSortStringFunc(key, so.descending)
}

func defaultMetaSort(i, j *meta.Meta) int { return cmp.Compare(j.Zid, i.Zid) }

func createSortTimestampFunc(key string, descending bool) sortFunc {
	if descending {
		return func(i, j *meta.Meta) int {
			iVal, iOk := i.Get(key)
			jVal, jOk := j.Get(key)
			if result := compareFound(jOk, iOk); result != 0 {
				return result
			}
			return cmp.Compare(meta.ExpandTimestamp(jVal), meta.ExpandTimestamp(iVal))
		}
	}
	return func(i, j *meta.Meta) int {
		iVal, iOk := i.Get(key)
		jVal, jOk := j.Get(key)
		if result := compareFound(iOk, jOk); result != 0 {
			return result
		}
		return cmp.Compare(meta.ExpandTimestamp(iVal), meta.ExpandTimestamp(jVal))
	}
}

func createSortNumberFunc(key string, descending bool) sortFunc {
	if descending {
		return func(i, j *meta.Meta) int {
			iVal, iOk := getNum(i, key)
			jVal, jOk := getNum(j, key)
			if result := compareFound(jOk, iOk); result != 0 {
				return result
			}
			return cmp.Compare(jVal, iVal)
		}
	}
	return func(i, j *meta.Meta) int {
		iVal, iOk := getNum(i, key)
		jVal, jOk := getNum(j, key)
		if result := compareFound(iOk, jOk); result != 0 {
			return result
		}
		return cmp.Compare(iVal, jVal)
	}
}

func getNum(m *meta.Meta, key string) (int64, bool) {
	if s, ok := m.Get(key); ok {
		if i, err := strconv.ParseInt(string(s), 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}

func createSortStringFunc(key string, descending bool) sortFunc {
	if descending {
		return func(i, j *meta.Meta) int {
			iVal, iOk := i.Get(key)
			jVal, jOk := j.Get(key)
			if result := compareFound(jOk, iOk); result != 0 {
				return result
			}
			return cmp.Compare(jVal, iVal)
		}
	}
	return func(i, j *meta.Meta) int {
		iVal, iOk := i.Get(key)
		jVal, jOk := j.Get(key)
		if result := compareFound(iOk, jOk); result != 0 {
			return result
		}
		return cmp.Compare(iVal, jVal)
	}
}

func compareFound(iOk, jOk bool) int {
	if iOk {
		if jOk {
			return 0
		}
		return 1
	}
	if jOk {
		return -1
	}
	return 0
}
