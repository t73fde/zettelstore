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
	"iter"
	"math"
	"slices"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/webapi"
)

// ContextSpec contains all specification values for calculating a context.
type ContextSpec struct {
	directionSpec
	maxCost  int
	maxCount int
	minCount int
	full     bool
}

// ContextPort is the collection of box methods needed by this directive.
type ContextPort interface {
	GetMeta(ctx context.Context, zid id.Zid) (*meta.Meta, error)
	SelectMeta(ctx context.Context, metaSeq []*meta.Meta, q *Query) ([]*meta.Meta, error)
}

// Print the spec on the given print environment.
func (spec *ContextSpec) Print(pe *PrintEnv) {
	pe.printSpace()
	pe.writeString(webapi.ContextDirective)
	if spec.full {
		pe.printSpace()
		pe.writeString(webapi.FullDirective)
	}
	spec.directionSpec.print(pe)
	pe.printPosInt(webapi.CostDirective, spec.maxCost)
	pe.printPosInt(webapi.MaxDirective, spec.maxCount)
	pe.printPosInt(webapi.MinDirective, spec.minCount)
}

// Execute the specification.
func (spec *ContextSpec) Execute(ctx context.Context, startSeq []*meta.Meta, port ContextPort) []*meta.Meta {
	maxCost := float64(spec.maxCost)
	if maxCost <= 0 {
		maxCost = 17
	}
	maxCount := spec.maxCount
	if maxCount <= 0 {
		maxCount = 200
	}
	tasks := newContextQueue(startSeq, maxCost, maxCount, spec.minCount, port)
	result := make([]*meta.Meta, 0, max(spec.minCount, 16))
	for {
		m, cost, level, dir := tasks.next()
		if m == nil {
			break
		}
		if level == 1 {
			cost = min(cost, 4.0)
		}
		result = append(result, m)

		for key, val := range m.ComputedRest() {
			tasks.addPair(ctx, key, val, cost, level, dir, spec)
		}
		if spec.full {
			newDir := 0
			if spec.isDirected {
				newDir = 1
			}
			tasks.addTags(ctx, m.GetFields(meta.KeyTags), cost, level, newDir)
		}
	}
	return result
}

type ztlCtxItem struct {
	cost  float64
	meta  *meta.Meta
	level uint
	dir   int8 // <0: backward, >0: forward, =0: not directed
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

func newContextQueue(startSeq []*meta.Meta, maxCost float64, maxCount, minCount int, port ContextPort) *contextTask {
	result := &contextTask{
		port:     port,
		seen:     idset.New(),
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

func (ct *contextTask) addPair(ctx context.Context, key string, value meta.Value, curCost float64, level uint, dir int, spec *ContextSpec) {
	if key == meta.KeyBack {
		return
	}
	newDir := 0
	newCost := curCost + contextCost(key)
	if key == meta.KeyBackward {
		if spec.isBackward {
			if spec.isDirected {
				newDir = -1
			}
			ct.addIDSet(ctx, newCost, level, newDir, value)
		}
		return
	}
	if key == meta.KeyForward {
		if spec.isForward {
			if spec.isDirected {
				newDir = -1
			}
			ct.addIDSet(ctx, newCost, level, newDir, value)
		}
		return
	}
	if meta.Inverse(key) != "" {
		// Backward reference
		if !spec.isBackward || dir > 0 {
			return
		}
		newDir = -1
	} else {
		// Forward reference
		if !spec.isForward || dir < 0 {
			return
		}
		newDir = 1
	}
	if !spec.isDirected {
		newDir = 0
	}
	if t := meta.Type(key); t == meta.TypeID {
		ct.addID(ctx, newCost, level, newDir, value)
	} else if t == meta.TypeIDSet {
		ct.addIDSet(ctx, newCost, level, newDir, value)
	}
}

func contextCost(key string) float64 {
	switch key {
	case meta.KeyFolge, meta.KeyPrecursor:
		return 0.2
	case meta.KeySequel, meta.KeyPrequel:
		return 1.0
	}
	return 2
}

func (ct *contextTask) addID(ctx context.Context, newCost float64, level uint, dir int, value meta.Value) {
	if zid, errParse := id.Parse(string(value)); errParse == nil {
		if m, errGetMeta := ct.port.GetMeta(ctx, zid); errGetMeta == nil {
			ct.addMeta(m, newCost, level, dir)
		}
	}
}

func (ct *contextTask) addMeta(m *meta.Meta, newCost float64, level uint, dir int) {
	if !ct.seen.Contains(m.Zid) {
		heap.Push(&ct.queue, ztlCtxItem{cost: newCost, meta: m, level: level + 1, dir: int8(dir)})
	}
}

func (ct *contextTask) addIDSet(ctx context.Context, newCost float64, level uint, dir int, value meta.Value) {
	elems := value.AsSlice()
	refCost := referenceCost(newCost, len(elems))
	for _, val := range elems {
		ct.addID(ctx, refCost, level, dir, meta.Value(val))
	}
}

func referenceCost(baseCost float64, numReferences int) float64 {
	nRefs := float64(numReferences)
	return nRefs*math.Log2(nRefs+1) + baseCost - 1
}

func (ct *contextTask) addTags(ctx context.Context, tagiter iter.Seq[string], baseCost float64, level uint, dir int) {
	tags := slices.Collect(tagiter)
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
		ct.addMeta(ct.metaZid[zid], minCost*costFactor, level, dir)
	})
}

func (ct *contextTask) updateTagData(ctx context.Context, tag string) *idset.Set {
	if _, found := ct.tagMetas[tag]; found {
		return ct.tagZids[tag]
	}
	q := Parse(meta.KeyTags + webapi.SearchOperatorHas + tag + " ORDER REVERSE " + meta.KeyID)
	ml, err := ct.port.SelectMeta(ctx, nil, q)
	if err != nil {
		ml = nil
	}
	ct.tagMetas[tag] = ml
	zids := idset.NewCap(len(ml))
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

func (ct *contextTask) next() (*meta.Meta, float64, uint, int) {
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
		return m, cost, item.level, int(item.dir)
	}
	return nil, -1, 0, 0
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
