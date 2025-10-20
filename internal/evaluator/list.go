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

package evaluator

import (
	"bytes"
	"context"
	"math"
	"slices"
	"strconv"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/ast/sztrans"
	"zettelstore.de/z/internal/query"
)

// QueryActionAST transforms a list of metadata according to query actions into an AST nested list.
func QueryActionAST(ctx context.Context, q *query.Query, ml []*meta.Meta) ast.BlockNode {
	bn, _ := QueryAction(ctx, q, ml)
	return sztrans.MustGetBlock(bn)
}

// QueryAction transforms a list of metadata according to query actions into a SZ nested list.
func QueryAction(ctx context.Context, q *query.Query, ml []*meta.Meta) (*sx.Pair, int) {
	ap := actionPara{
		ctx:    ctx,
		q:      q,
		ml:     ml,
		kind:   zsx.SymListUnordered,
		minVal: -1,
		maxVal: -1,
	}
	actions := q.Actions()
	if len(actions) == 0 {
		return ap.createBlockNodeMeta("")
	}

	acts := make([]string, 0, len(actions))
	for _, act := range actions {
		if strings.HasPrefix(act, api.NumberedAction[0:1]) {
			ap.kind = zsx.SymListOrdered
			continue
		}
		if strings.HasPrefix(act, api.MinAction) {
			if num, err := strconv.Atoi(act[3:]); err == nil && num > 0 {
				ap.minVal = num
				continue
			}
		}
		if strings.HasPrefix(act, api.MaxAction) {
			if num, err := strconv.Atoi(act[3:]); err == nil && num > 0 {
				ap.maxVal = num
				continue
			}
		}
		if act == api.ReIndexAction {
			continue
		}
		acts = append(acts, act)
	}
	var firstUnknowAct string
	for _, act := range acts {
		switch act {
		case api.KeysAction:
			return ap.createBlockNodeMetaKeys()
		}
		key := strings.ToLower(act)
		switch meta.Type(key) {
		case meta.TypeWord:
			return ap.createBlockNodeWord(key)
		case meta.TypeTagSet:
			return ap.createBlockNodeTagSet(key)
		}
		if firstUnknowAct == "" {
			firstUnknowAct = act
		}
	}

	bn, numItems := ap.createBlockNodeMeta(strings.ToLower(firstUnknowAct))
	if bn != nil && numItems == 0 && firstUnknowAct == strings.ToUpper(firstUnknowAct) {
		bn, numItems = ap.createBlockNodeMeta("")
	}
	return bn, numItems
}

type actionPara struct {
	ctx    context.Context
	q      *query.Query
	ml     []*meta.Meta
	kind   *sx.Symbol
	minVal int
	maxVal int
}

func (ap *actionPara) createBlockNodeWord(key string) (*sx.Pair, int) {
	var buf bytes.Buffer
	ccs, bufLen := ap.prepareCatAction(key, &buf)
	if len(ccs) == 0 {
		return nil, 0
	}

	ccs.SortByName()
	var items sx.ListBuilder
	count := 0
	for _, cat := range ccs {
		buf.WriteString(string(cat.Name))
		items.Add(zsx.MakeBlock(zsx.MakePara(
			zsx.MakeLink(nil,
				sz.ScanReference(buf.String()),
				sx.MakeList(zsx.MakeText(string(cat.Name)))),
		)))
		count++
		buf.Truncate(bufLen)
	}
	return zsx.MakeList(ap.kind, nil, items.List()), count
}

func (ap *actionPara) createBlockNodeTagSet(key string) (*sx.Pair, int) {
	var buf bytes.Buffer
	ccs, bufLen := ap.prepareCatAction(key, &buf)
	if len(ccs) == 0 {
		return nil, 0
	}
	ccs.SortByCount()
	ccs = ap.limitTags(ccs)
	countClassAttrs := ap.calcFontSizes(ccs)

	ccs.SortByName()
	var tags sx.ListBuilder
	count := 0
	for i, cat := range ccs {
		if i > 0 {
			tags.Add(zsx.MakeText(" "))
		}
		buf.WriteString(string(cat.Name))
		tags.AddN(
			zsx.MakeLink(
				countClassAttrs[cat.Count],
				sz.ScanReference(buf.String()),
				sx.MakeList(zsx.MakeText(string(cat.Name)))),
			zsx.MakeFormat(zsx.SymFormatSuper,
				nil,
				sx.MakeList(zsx.MakeText(strconv.Itoa(cat.Count)))),
		)
		count++
		buf.Truncate(bufLen)
	}
	return zsx.MakeParaList(tags.List()), count
}

func (ap *actionPara) limitTags(ccs meta.CountedCategories) meta.CountedCategories {
	if minVal, maxVal := ap.minVal, ap.maxVal; minVal > 0 || maxVal > 0 {
		if minVal < 0 {
			minVal = ccs[len(ccs)-1].Count
		}
		if maxVal < 0 {
			maxVal = ccs[0].Count
		}
		if ccs[len(ccs)-1].Count < minVal || maxVal < ccs[0].Count {
			temp := make(meta.CountedCategories, 0, len(ccs))
			for _, cat := range ccs {
				if minVal <= cat.Count && cat.Count <= maxVal {
					temp = append(temp, cat)
				}
			}
			return temp
		}
	}
	return ccs
}

func (ap *actionPara) createBlockNodeMetaKeys() (*sx.Pair, int) {
	arr := make(meta.Arrangement, 128)
	for _, m := range ap.ml {
		for k := range m.Map() {
			arr[k] = append(arr[k], m)
		}
	}
	if len(arr) == 0 {
		return nil, 0
	}
	ccs := arr.Counted()
	ccs.SortByName()

	var buf bytes.Buffer
	bufLen := ap.prepareSimpleQuery(&buf)
	var items sx.ListBuilder
	count := 0
	for _, cat := range ccs {
		buf.WriteString(string(cat.Name))
		buf.WriteString(api.ExistOperator)
		q1 := buf.String()
		buf.Truncate(bufLen)
		buf.WriteString(api.ActionSeparator)
		buf.WriteString(string(cat.Name))
		q2 := buf.String()
		buf.Truncate(bufLen)

		items.Add(zsx.MakeBlock(zsx.MakePara(
			zsx.MakeLink(nil,
				sz.ScanReference(q1),
				sx.MakeList(zsx.MakeText(string(cat.Name)))),
			zsx.MakeText(" "),
			zsx.MakeText("("+strconv.Itoa(cat.Count)+", "),
			zsx.MakeLink(nil,
				sz.ScanReference(q2),
				sx.MakeList(zsx.MakeText("values"))),
			zsx.MakeText(")"),
		)))
		count++
	}
	return zsx.MakeList(ap.kind, nil, items.List()), count
}

func (ap *actionPara) createBlockNodeMeta(key string) (*sx.Pair, int) {
	if len(ap.ml) == 0 {
		return nil, 0
	}
	var items sx.ListBuilder
	count := 0
	for _, m := range ap.ml {
		if key != "" {
			if _, found := m.Get(key); !found {
				continue
			}
		}
		items.Add(zsx.MakeBlock(zsx.MakePara(
			zsx.MakeLink(nil,
				sz.ScanReference(m.Zid.String()),
				sx.MakeList(zsx.MakeText(sz.NormalizedSpacedText(m.GetTitle())))),
		)))
		count++
	}
	return zsx.MakeList(ap.kind, nil, items.List()), count
}

func (ap *actionPara) prepareCatAction(key string, buf *bytes.Buffer) (meta.CountedCategories, int) {
	if len(ap.ml) == 0 {
		return nil, 0
	}
	ccs := meta.CreateArrangement(ap.ml, key).Counted()
	if len(ccs) == 0 {
		return nil, 0
	}

	ap.prepareSimpleQuery(buf)
	buf.WriteString(key)
	buf.WriteString(api.SearchOperatorHas)
	bufLen := buf.Len()

	return ccs, bufLen
}

func (ap *actionPara) prepareSimpleQuery(buf *bytes.Buffer) int {
	sea := ap.q.Clone()
	sea.RemoveActions()
	buf.WriteString(ast.QueryPrefix)
	sea.Print(buf)
	if buf.Len() > len(ast.QueryPrefix) {
		buf.WriteByte(' ')
	}
	return buf.Len()
}

const fontSizes = 6 // Must be the number of CSS classes zs-font-size-* in base.css
const fontSizes64 = float64(fontSizes)

func (*actionPara) calcFontSizes(ccs meta.CountedCategories) map[int]*sx.Pair {
	var fsAttrs [fontSizes]*sx.Pair
	for i := range fontSizes {
		fsAttrs[i] = sx.MakeList(sx.Cons(sx.MakeString("class"), sx.MakeString("zs-font-size-"+strconv.Itoa(i))))
	}

	countMap := make(map[int]int, len(ccs))
	for _, cat := range ccs {
		countMap[cat.Count]++
	}

	countList := make([]int, 0, len(countMap))
	for count := range countMap {
		countList = append(countList, count)
	}
	slices.Sort(countList)

	result := make(map[int]*sx.Pair, len(countList))
	if len(countList) <= fontSizes {
		// If we have less different counts, center them inside the fsAttrs vector.
		curSize := (fontSizes - len(countList)) / 2
		for _, count := range countList {
			result[count] = fsAttrs[curSize]
			curSize++
		}
		return result
	}

	// Idea: the number of occurences for a specific count is substracted from a budget.
	total := float64(len(ccs))
	curSize := 0
	budget := calcBudget(total, 0.0)
	for _, count := range countList {
		result[count] = fsAttrs[curSize]
		cc := float64(countMap[count])
		total -= cc
		budget -= cc
		if budget < 1 {
			curSize++
			if curSize >= fontSizes {
				curSize = fontSizes
				budget = 0.0
			} else {
				budget = calcBudget(total, float64(curSize))
			}
		}
	}
	return result
}

func calcBudget(total, curSize float64) float64 { return math.Round(total / (fontSizes64 - curSize)) }
