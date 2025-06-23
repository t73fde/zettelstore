//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

package query

import (
	"container/heap"
	"context"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"
)

// ThreadSpec contains all information for a thread directive.
type ThreadSpec struct {
	IsFolge    bool
	IsSequel   bool
	IsForward  bool
	IsBackward bool
	MaxCount   int
}

// Print the spec on the given print environment.
func (spec *ThreadSpec) Print(pe *PrintEnv) {
	pe.printSpace()
	if spec.IsFolge {
		if spec.IsSequel {
			pe.writeString(api.ThreadDirective)
		} else {
			pe.writeString(api.FolgeDirective)
		}
	} else if spec.IsSequel {
		pe.writeString(api.SequelDirective)
	} else {
		panic("neither folge nor sequel")
	}

	if spec.IsForward {
		if !spec.IsBackward {
			pe.printSpace()
			pe.writeString(api.ForwardDirective)
		}
	} else if spec.IsBackward {
		pe.printSpace()
		pe.writeString(api.BackwardDirective)
	} else {
		panic("neither forward nor backward")
	}

	pe.printPosInt(api.MaxDirective, spec.MaxCount)
}

// ThreadPort is the collection of box methods needed by this directive.
type ThreadPort interface {
	GetMeta(ctx context.Context, zid id.Zid) (*meta.Meta, error)
}

// Execute the specification.
func (spec *ThreadSpec) Execute(ctx context.Context, startSeq []*meta.Meta, port ThreadPort) []*meta.Meta {
	tasks := newThreadQueue(startSeq, spec.MaxCount, port)
	result := make([]*meta.Meta, 0, 16)
	for {
		m, level := tasks.next()
		if m == nil {
			break
		}
		result = append(result, m)

		for key, val := range m.ComputedRest() {
			tasks.addPair(ctx, key, val, level, spec)
		}
	}
	return result
}

type ztlThreadItem struct {
	meta  *meta.Meta
	level uint
}
type ztlThreadQueue []ztlThreadItem

func (q ztlThreadQueue) Len() int { return len(q) }
func (q ztlThreadQueue) Less(i, j int) bool {
	if levelI, levelJ := q[i].level, q[j].level; levelI < levelJ {
		return true
	} else if levelI == levelJ {
		return q[i].meta.Zid < q[j].meta.Zid
	}
	return false
}
func (q ztlThreadQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *ztlThreadQueue) Push(x any)   { *q = append(*q, x.(ztlThreadItem)) }
func (q *ztlThreadQueue) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	old[n-1].meta = nil // avoid memory leak
	*q = old[0 : n-1]
	return item
}

type threadTask struct {
	port     ThreadPort
	seen     *idset.Set
	queue    ztlThreadQueue
	maxCount int
}

func newThreadQueue(startSeq []*meta.Meta, maxCount int, port ThreadPort) *threadTask {
	result := &threadTask{
		port:     port,
		seen:     idset.New(),
		maxCount: maxCount,
	}

	queue := make(ztlThreadQueue, 0, len(startSeq))
	for _, m := range startSeq {
		queue = append(queue, ztlThreadItem{meta: m})
	}
	heap.Init(&queue)
	result.queue = queue
	return result
}

func (ct *threadTask) next() (*meta.Meta, uint) {
	for len(ct.queue) > 0 {
		item := heap.Pop(&ct.queue).(ztlThreadItem)
		m := item.meta
		zid := m.Zid
		if ct.seen.Contains(zid) {
			continue
		}
		level := item.level
		if ct.hasEnough(level) {
			break
		}
		ct.seen.Add(zid)
		return m, item.level
	}
	return nil, 0
}

func (ct *threadTask) hasEnough(level uint) bool {
	maxCount := ct.maxCount
	if level <= 1 || ct.maxCount <= 0 {
		// Always add direct descendants of the initial zettel
		return false
	}
	return maxCount <= ct.seen.Length()
}

func (ct *threadTask) addPair(ctx context.Context, key string, value meta.Value, level uint, spec *ThreadSpec) {
	isFolge, isSequel, isBackward, isForward := spec.IsFolge, spec.IsSequel, spec.IsBackward, spec.IsForward
	switch key {
	case meta.KeyPrecursor:
		if !isFolge || !isBackward {
			return
		}
	case meta.KeyFolge:
		if !isFolge || !isForward {
			return
		}
	case meta.KeyPrequel:
		if !isSequel || !isBackward {
			return
		}
	case meta.KeySequel:
		if !isSequel || !isForward {
			return
		}
	default:
		return
	}
	elems := value.AsSlice()
	for _, val := range elems {
		if zid, errParse := id.Parse(val); errParse == nil {
			if m, errGetMeta := ct.port.GetMeta(ctx, zid); errGetMeta == nil {
				if !ct.seen.Contains(m.Zid) {
					heap.Push(&ct.queue, ztlThreadItem{meta: m, level: level + 1})
				}
			}
		}
	}
}
