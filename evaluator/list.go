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
	"sort"
	"strconv"
	"strings"

	"zettelstore.de/client.fossil/api"
	"zettelstore.de/client.fossil/attrs"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/config"
	"zettelstore.de/z/encoding/atom"
	"zettelstore.de/z/encoding/rss"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel/meta"
)

// QueryAction transforms a list of metadata according to query actions into a AST nested list.
func QueryAction(ctx context.Context, q *query.Query, ml []*meta.Meta, rtConfig config.Config) (ast.BlockNode, int) {
	ap := actionPara{
		ctx:   ctx,
		q:     q,
		ml:    ml,
		kind:  ast.NestedListUnordered,
		min:   -1,
		max:   -1,
		title: rtConfig.GetSiteName(),
	}
	actions := q.Actions()
	if len(actions) == 0 {
		return ap.createBlockNodeMeta("")
	}

	acts := make([]string, 0, len(actions))
	for i, act := range actions {
		if strings.HasPrefix(act, "N") {
			ap.kind = ast.NestedListOrdered
			continue
		}
		if strings.HasPrefix(act, "MIN") {
			if num, err := strconv.Atoi(act[3:]); err == nil && num > 0 {
				ap.min = num
				continue
			}
		}
		if strings.HasPrefix(act, "MAX") {
			if num, err := strconv.Atoi(act[3:]); err == nil && num > 0 {
				ap.max = num
				continue
			}
		}
		if act == "TITLE" && i+1 < len(actions) {
			ap.title = strings.Join(actions[i+1:], " ")
			break
		}
		if act == "REINDEX" {
			continue
		}
		acts = append(acts, act)
	}
	var firstUnknownKey string
	for _, act := range acts {
		switch act {
		case "ATOM":
			return ap.createBlockNodeAtom(rtConfig)
		case "RSS":
			return ap.createBlockNodeRSS(rtConfig)
		case "KEYS":
			return ap.createBlockNodeMetaKeys()
		}
		key := strings.ToLower(act)
		switch meta.Type(key) {
		case meta.TypeWord:
			return ap.createBlockNodeWord(key)
		case meta.TypeTagSet:
			return ap.createBlockNodeTagSet(key)
		}
		if firstUnknownKey == "" {
			firstUnknownKey = key
		}
	}
	return ap.createBlockNodeMeta(firstUnknownKey)
}

type actionPara struct {
	ctx   context.Context
	q     *query.Query
	ml    []*meta.Meta
	kind  ast.NestedListKind
	min   int
	max   int
	title string
}

func (ap *actionPara) createBlockNodeWord(key string) (ast.BlockNode, int) {
	var buf bytes.Buffer
	ccs, bufLen := ap.prepareCatAction(key, &buf)
	if len(ccs) == 0 {
		return nil, 0
	}
	items := make([]ast.ItemSlice, 0, len(ccs))
	ccs.SortByName()
	for _, cat := range ccs {
		buf.WriteString(cat.Name)
		items = append(items, ast.ItemSlice{ast.CreateParaNode(&ast.LinkNode{
			Attrs:   nil,
			Ref:     ast.ParseReference(buf.String()),
			Inlines: ast.InlineSlice{&ast.TextNode{Text: cat.Name}},
		})})
		buf.Truncate(bufLen)
	}
	return &ast.NestedListNode{
		Kind:  ap.kind,
		Items: items,
		Attrs: nil,
	}, len(items)
}

func (ap *actionPara) createBlockNodeTagSet(key string) (ast.BlockNode, int) {
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
			para = append(para, &ast.SpaceNode{Lexeme: " "})
		}
		buf.WriteString(cat.Name)
		para = append(para,
			&ast.LinkNode{
				Attrs: countMap[cat.Count],
				Ref:   ast.ParseReference(buf.String()),
				Inlines: ast.InlineSlice{
					&ast.TextNode{Text: cat.Name},
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

func (ap *actionPara) limitTags(ccs meta.CountedCategories) meta.CountedCategories {
	if min, max := ap.min, ap.max; min > 0 || max > 0 {
		if min < 0 {
			min = ccs[len(ccs)-1].Count
		}
		if max < 0 {
			max = ccs[0].Count
		}
		if ccs[len(ccs)-1].Count < min || max < ccs[0].Count {
			temp := make(meta.CountedCategories, 0, len(ccs))
			for _, cat := range ccs {
				if min <= cat.Count && cat.Count <= max {
					temp = append(temp, cat)
				}
			}
			return temp
		}
	}
	return ccs
}

func (ap *actionPara) createBlockNodeMetaKeys() (ast.BlockNode, int) {
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
		buf.WriteString(cat.Name)
		buf.WriteString(api.ExistOperator)
		q1 := buf.String()
		buf.Truncate(bufLen)
		buf.WriteString(api.ActionSeparator)
		buf.WriteString(cat.Name)
		q2 := buf.String()
		buf.Truncate(bufLen)

		items = append(items, ast.ItemSlice{ast.CreateParaNode(
			&ast.LinkNode{
				Attrs:   nil,
				Ref:     ast.ParseReference(q1),
				Inlines: ast.InlineSlice{&ast.TextNode{Text: cat.Name}},
			},
			&ast.SpaceNode{Lexeme: " "},
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

func (ap *actionPara) createBlockNodeMeta(key string) (ast.BlockNode, int) {
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
			Inlines: parser.ParseSpacedText(m.GetTitle()),
		})})
	}
	return &ast.NestedListNode{
		Kind:  ap.kind,
		Items: items,
		Attrs: nil,
	}, len(items)
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

func (*actionPara) calcFontSizes(ccs meta.CountedCategories) map[int]attrs.Attributes {
	var fsAttrs [fontSizes]attrs.Attributes
	var a attrs.Attributes
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
	sort.Ints(countList)

	result := make(map[int]attrs.Attributes, len(countList))
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

func (ap *actionPara) createBlockNodeRSS(cfg config.Config) (ast.BlockNode, int) {
	var rssConfig rss.Configuration
	rssConfig.Setup(ap.ctx, cfg)
	rssConfig.Title = ap.title
	data := rssConfig.Marshal(ap.q, ap.ml)

	return &ast.VerbatimNode{
		Kind:    ast.VerbatimProg,
		Attrs:   attrs.Attributes{"lang": "xml"},
		Content: data,
	}, len(ap.ml)
}

func (ap *actionPara) createBlockNodeAtom(cfg config.Config) (ast.BlockNode, int) {
	var atomConfig atom.Configuration
	atomConfig.Setup(cfg)
	atomConfig.Title = ap.title
	data := atomConfig.Marshal(ap.q, ap.ml)

	return &ast.VerbatimNode{
		Kind:    ast.VerbatimProg,
		Attrs:   attrs.Attributes{"lang": "xml"},
		Content: data,
	}, len(ap.ml)
}
