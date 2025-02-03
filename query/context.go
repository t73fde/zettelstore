//-----------------------------------------------------------------------------
// Copyright (c) 2023-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2023-present Detlef Stern
//-----------------------------------------------------------------------------

package query

import (
	"container/heap"
	"context"
	"math"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"zettelstore.de/z/zettel/meta"
)

// ContextSpec contains all specification values for calculating a context.
type ContextSpec struct {
	Direction ContextDirection
	MaxCost   int
	MaxCount  int
	MinCount  int
	Full      bool
}

// ContextDirection specifies the direction a context should be calculated.
type ContextDirection uint8

// Constants for ContextDirection.
const (
	ContextDirBoth ContextDirection = iota
	ContextDirForward
	ContextDirBackward
)

// ContextPort is the collection of box methods needed by this directive.
type ContextPort interface {
	GetMeta(ctx context.Context, zid id.Zid) (*meta.Meta, error)
	SelectMeta(ctx context.Context, metaSeq []*meta.Meta, q *Query) ([]*meta.Meta, error)
}

// Print the spec on the given print environment.
func (spec *ContextSpec) Print(pe *PrintEnv) {
	pe.printSpace()
	pe.writeString(api.ContextDirective)
	if spec.Full {
		pe.printSpace()
		pe.writeString(api.FullDirective)
	}
	switch spec.Direction {
	case ContextDirBackward:
		pe.printSpace()
		pe.writeString(api.BackwardDirective)
	case ContextDirForward:
		pe.printSpace()
		pe.writeString(api.ForwardDirective)
	}
	pe.printPosInt(api.CostDirective, spec.MaxCost)
	pe.printPosInt(api.MaxDirective, spec.MaxCount)
	pe.printPosInt(api.MinDirective, spec.MinCount)
}

// Execute the specification.
func (spec *ContextSpec) Execute(ctx context.Context, startSeq []*meta.Meta, port ContextPort) []*meta.Meta {
	maxCost := float64(spec.MaxCost)
	if maxCost <= 0 {
		maxCost = 17
	}
	maxCount := spec.MaxCount
	if maxCount <= 0 {
		maxCount = 200
	}
	tasks := newQueue(startSeq, maxCost, maxCount, spec.MinCount, port)
	isBackward := spec.Direction == ContextDirBoth || spec.Direction == ContextDirBackward
	isForward := spec.Direction == ContextDirBoth || spec.Direction == ContextDirForward
	result := make([]*meta.Meta, 0, max(spec.MinCount, 16))
	for {
		m, cost, level := tasks.next()
		if m == nil {
			break
		}
		result = append(result, m)

		for _, p := range m.ComputedPairsRest() {
			tasks.addPair(ctx, p.Key, p.Value, cost, level, isBackward, isForward)
		}
		if !spec.Full {
			continue
		}
		if tags, found := m.GetList(api.KeyTags); found {
			tasks.addTags(ctx, tags, cost, level)
		}
	}
	return result
}

type ztlCtxItem struct {
	cost  float64
	meta  *meta.Meta
	level uint
}
type ztlCtxQueue []ztlCtxItem

func (q ztlCtxQueue) Len() int { return len(q) }
func (q ztlCtxQueue) Less(i, j int) bool {
	levelI, levelJ := q[i].level, q[j].level
	if levelI == 0 {
		if levelJ == 0 {
			return q[i].meta.Zid < q[j].meta.Zid
		}
		return true
	}
	if levelI == 1 {
		if levelJ == 0 {
			return false
		}
		if levelJ == 1 {
			costI, costJ := q[i].cost, q[j].cost
			if costI == costJ {
				return q[i].meta.Zid < q[j].meta.Zid
			}
			return costI < costJ
		}
		return true
	}
	if levelJ == 0 || levelJ == 1 {
		return false
	}
	costI, costJ := q[i].cost, q[j].cost
	if costI == costJ {
		return q[i].meta.Zid < q[j].meta.Zid
	}
	return costI < costJ
}
func (q ztlCtxQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *ztlCtxQueue) Push(x any)   { *q = append(*q, x.(ztlCtxItem)) }
func (q *ztlCtxQueue) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	old[n-1].meta = nil // avoid memory leak
	*q = old[0 : n-1]
	return item
}

type contextTask struct {
	port     ContextPort
	seen     *idset.Set
	queue    ztlCtxQueue
	maxCost  float64
	maxCount int
	minCount int
	tagMetas map[string][]*meta.Meta
	tagZids  map[string]*idset.Set // just the zids of tagMetas
	metaZid  map[id.Zid]*meta.Meta // maps zid to meta for all meta retrieved with tags
}

func newQueue(startSeq []*meta.Meta, maxCost float64, maxCount, minCount int, port ContextPort) *contextTask {
	result := &contextTask{
		port:     port,
		seen:     idset.NewSet(),
		maxCost:  maxCost,
		maxCount: max(maxCount, minCount),
		minCount: minCount,
		tagMetas: make(map[string][]*meta.Meta),
		tagZids:  make(map[string]*idset.Set),
		metaZid:  make(map[id.Zid]*meta.Meta),
	}

	queue := make(ztlCtxQueue, 0, len(startSeq))
	for _, m := range startSeq {
		queue = append(queue, ztlCtxItem{cost: 1, meta: m})
	}
	heap.Init(&queue)
	result.queue = queue
	return result
}

func (ct *contextTask) addPair(ctx context.Context, key, value string, curCost float64, level uint, isBackward, isForward bool) {
	if key == api.KeyBack {
		return
	}
	newCost := curCost + contextCost(key)
	if key == api.KeyBackward {
		if isBackward {
			ct.addIDSet(ctx, newCost, level, value)
		}
		return
	}
	if key == api.KeyForward {
		if isForward {
			ct.addIDSet(ctx, newCost, level, value)
		}
		return
	}
	hasInverse := meta.Inverse(key) != ""
	if (!hasInverse || !isBackward) && (hasInverse || !isForward) {
		return
	}
	if t := meta.Type(key); t == meta.TypeID {
		ct.addID(ctx, newCost, level, value)
	} else if t == meta.TypeIDSet {
		ct.addIDSet(ctx, newCost, level, value)
	}
}

func contextCost(key string) float64 {
	switch key {
	case api.KeyFolge, api.KeyPrecursor:
		return 0.1
	case api.KeySequel, api.KeyPrequel:
		return 1.0
	case api.KeySuccessors, api.KeyPredecessor:
		return 7
	}
	return 2
}

func (ct *contextTask) addID(ctx context.Context, newCost float64, level uint, value string) {
	if zid, errParse := id.Parse(value); errParse == nil {
		if m, errGetMeta := ct.port.GetMeta(ctx, zid); errGetMeta == nil {
			ct.addMeta(m, newCost, level)
		}
	}
}

func (ct *contextTask) addMeta(m *meta.Meta, newCost float64, level uint) {
	if !ct.seen.Contains(m.Zid) {
		heap.Push(&ct.queue, ztlCtxItem{cost: newCost, meta: m, level: level + 1})
	}
}

func (ct *contextTask) addIDSet(ctx context.Context, newCost float64, level uint, value string) {
	elems := meta.ListFromValue(value)
	refCost := referenceCost(newCost, len(elems))
	for _, val := range elems {
		ct.addID(ctx, refCost, level, val)
	}
}

func referenceCost(baseCost float64, numReferences int) float64 {
	nRefs := float64(numReferences)
	return nRefs*math.Log2(nRefs+1) + baseCost - 1
}

func (ct *contextTask) addTags(ctx context.Context, tags []string, baseCost float64, level uint) {
	var zidSet *idset.Set
	for _, tag := range tags {
		zs := ct.updateTagData(ctx, tag)
		zidSet = zidSet.IUnion(zs)
	}
	zidSet.ForEach(func(zid id.Zid) {
		minCost := math.MaxFloat64
		costFactor := 1.1
		for _, tag := range tags {
			tagZids := ct.tagZids[tag]
			if tagZids.Contains(zid) {
				cost := tagCost(baseCost, tagZids.Length())
				if cost < minCost {
					minCost = cost
				}
				costFactor /= 1.1
			}
		}
		ct.addMeta(ct.metaZid[zid], minCost*costFactor, level)
	})
}

func (ct *contextTask) updateTagData(ctx context.Context, tag string) *idset.Set {
	if _, found := ct.tagMetas[tag]; found {
		return ct.tagZids[tag]
	}
	q := Parse(api.KeyTags + api.SearchOperatorHas + tag + " ORDER REVERSE " + api.KeyID)
	ml, err := ct.port.SelectMeta(ctx, nil, q)
	if err != nil {
		ml = nil
	}
	ct.tagMetas[tag] = ml
	zids := idset.NewSetCap(len(ml))
	for _, m := range ml {
		zid := m.Zid
		zids = zids.Add(zid)
		if _, found := ct.metaZid[zid]; !found {
			ct.metaZid[zid] = m
		}
	}
	ct.tagZids[tag] = zids
	return zids
}

func tagCost(baseCost float64, numTags int) float64 {
	nTags := float64(numTags)
	return nTags*math.Log2(nTags+1) + baseCost - 1
}

func (ct *contextTask) next() (*meta.Meta, float64, uint) {
	for len(ct.queue) > 0 {
		item := heap.Pop(&ct.queue).(ztlCtxItem)
		m := item.meta
		zid := m.Zid
		if ct.seen.Contains(zid) {
			continue
		}
		cost, level := item.cost, item.level
		if ct.hasEnough(cost, level) {
			break
		}
		ct.seen.Add(zid)
		return m, cost, item.level
	}
	return nil, -1, 0
}

func (ct *contextTask) hasEnough(cost float64, level uint) bool {
	if level <= 1 {
		// Always add direct descendants of the initial zettel
		return false
	}
	length := ct.seen.Length()
	if minCount := ct.minCount; 0 < minCount && minCount > length {
		return false
	}
	if maxCount := ct.maxCount; 0 < maxCount && maxCount <= length {
		return true
	}
	maxCost := ct.maxCost
	return maxCost == 0.0 || maxCost <= cost
}
