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

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/query"
)

// QueryActionAST transforms a list of metadata according to query actions into a AST nested list.
func QueryActionAST(ctx context.Context, q *query.Query, ml []*meta.Meta) (ast.BlockNode, int) {
	ap := actionParaAST{
		ctx:    ctx,
		q:      q,
		ml:     ml,
		kind:   ast.NestedListUnordered,
		minVal: -1,
		maxVal: -1,
	}
	actions := q.Actions()
	if len(actions) == 0 {
		return ap.createBlockNodeMetaAST("")
	}

	acts := make([]string, 0, len(actions))
	for _, act := range actions {
		if strings.HasPrefix(act, api.NumberedAction[0:1]) {
			ap.kind = ast.NestedListOrdered
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
			return ap.createBlockNodeMetaKeysAST()
		}
		key := strings.ToLower(act)
		switch meta.Type(key) {
		case meta.TypeWord:
			return ap.createBlockNodeWordAST(key)
		case meta.TypeTagSet:
			return ap.createBlockNodeTagSetAST(key)
		}
		if firstUnknowAct == "" {
			firstUnknowAct = act
		}
	}
	bn, numItems := ap.createBlockNodeMetaAST(strings.ToLower(firstUnknowAct))
	if bn != nil && numItems == 0 && firstUnknowAct == strings.ToUpper(firstUnknowAct) {
		bn, numItems = ap.createBlockNodeMetaAST("")
	}
	return bn, numItems
}

type actionParaAST struct {
	ctx    context.Context
	q      *query.Query
	ml     []*meta.Meta
	kind   ast.NestedListKind
	minVal int
	maxVal int
}

func (ap *actionParaAST) createBlockNodeWordAST(key string) (ast.BlockNode, int) {
	var buf bytes.Buffer
	ccs, bufLen := ap.prepareCatAction(key, &buf)
	if len(ccs) == 0 {
		return nil, 0
	}
	items := make([]ast.ItemSlice, 0, len(ccs))
	ccs.SortByName()
	for _, cat := range ccs {
		buf.WriteString(string(cat.Name))
		items = append(items, ast.ItemSlice{ast.CreateParaNode(&ast.LinkNode{
			Attrs:   nil,
			Ref:     ast.ParseReference(buf.String()),
			Inlines: ast.InlineSlice{&ast.TextNode{Text: string(cat.Name)}},
		})})
		buf.Truncate(bufLen)
	}
	return &ast.NestedListNode{
		Kind:  ap.kind,
		Items: items,
		Attrs: nil,
	}, len(items)
}

func (ap *actionParaAST) createBlockNodeTagSetAST(key string) (ast.BlockNode, int) {
	var buf bytes.Buffer
	ccs, bufLen := ap.prepareCatAction(key, &buf)
	if len(ccs) == 0 {
		return nil, 0
	}
	ccs.SortByCount()
	ccs = ap.limitTags(ccs)
	countMap := ap.calcFontSizes(ccs)

	para := make(ast.InlineSlice, 0, len(ccs))
	ccs.SortByName()
	for i, cat := range ccs {
		if i > 0 {
			para = append(para, &ast.TextNode{Text: " "})
		}
		buf.WriteString(string(cat.Name))
		para = append(para,
			&ast.LinkNode{
				Attrs: countMap[cat.Count],
				Ref:   ast.ParseReference(buf.String()),
				Inlines: ast.InlineSlice{
					&ast.TextNode{Text: string(cat.Name)},
				},
			},
			&ast.FormatNode{
				Kind:    ast.FormatSuper,
				Attrs:   nil,
				Inlines: ast.InlineSlice{&ast.TextNode{Text: strconv.Itoa(cat.Count)}},
			},
		)
		buf.Truncate(bufLen)
	}
	return &ast.ParaNode{Inlines: para}, len(ccs)
}

func (ap *actionParaAST) limitTags(ccs meta.CountedCategories) meta.CountedCategories {
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

func (ap *actionParaAST) createBlockNodeMetaKeysAST() (ast.BlockNode, int) {
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
	items := make([]ast.ItemSlice, 0, len(ccs))
	for _, cat := range ccs {
		buf.WriteString(string(cat.Name))
		buf.WriteString(api.ExistOperator)
		q1 := buf.String()
		buf.Truncate(bufLen)
		buf.WriteString(api.ActionSeparator)
		buf.WriteString(string(cat.Name))
		q2 := buf.String()
		buf.Truncate(bufLen)

		items = append(items, ast.ItemSlice{ast.CreateParaNode(
			&ast.LinkNode{
				Attrs:   nil,
				Ref:     ast.ParseReference(q1),
				Inlines: ast.InlineSlice{&ast.TextNode{Text: string(cat.Name)}},
			},
			&ast.TextNode{Text: " "},
			&ast.TextNode{Text: "(" + strconv.Itoa(cat.Count) + ", "},
			&ast.LinkNode{
				Attrs:   nil,
				Ref:     ast.ParseReference(q2),
				Inlines: ast.InlineSlice{&ast.TextNode{Text: "values"}},
			},
			&ast.TextNode{Text: ")"},
		)})
	}
	return &ast.NestedListNode{
		Kind:  ap.kind,
		Items: items,
		Attrs: nil,
	}, len(items)
}

func (ap *actionParaAST) createBlockNodeMetaAST(key string) (ast.BlockNode, int) {
	if len(ap.ml) == 0 {
		return nil, 0
	}
	items := make([]ast.ItemSlice, 0, len(ap.ml))
	for _, m := range ap.ml {
		if key != "" {
			if _, found := m.Get(key); !found {
				continue
			}
		}
		items = append(items, ast.ItemSlice{ast.CreateParaNode(&ast.LinkNode{
			Attrs:   nil,
			Ref:     ast.ParseReference(m.Zid.String()),
			Inlines: ast.ParseSpacedText(m.GetTitle()),
		})})
	}
	return &ast.NestedListNode{
		Kind:  ap.kind,
		Items: items,
		Attrs: nil,
	}, len(items)
}

func (ap *actionParaAST) prepareCatAction(key string, buf *bytes.Buffer) (meta.CountedCategories, int) {
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

func (ap *actionParaAST) prepareSimpleQuery(buf *bytes.Buffer) int {
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

func (*actionParaAST) calcFontSizes(ccs meta.CountedCategories) map[int]zsx.Attributes {
	var fsAttrs [fontSizes]zsx.Attributes
	var a zsx.Attributes
	for i := range fontSizes {
		fsAttrs[i] = a.AddClass("zs-font-size-" + strconv.Itoa(i))
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

	result := make(map[int]zsx.Attributes, len(countList))
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
