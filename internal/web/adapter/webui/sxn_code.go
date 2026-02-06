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

package webui

import (
	"context"
	"fmt"
	"io"

	"t73f.de/r/sx/sxeval"
	"t73f.de/r/zero/graph"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"
)

func (wui *WebUI) loadAllSxnCodeZettel(ctx context.Context) (graph.Digraph[id.Zid], *sxeval.Binding, error) {
	// getMeta MUST currently use GetZettel, because GetMeta just uses the
	// Index, which might not be current.
	getMeta := func(ctx context.Context, zid id.Zid) (*meta.Meta, error) {
		z, err := wui.box.GetZettel(ctx, zid)
		if err != nil {
			return nil, err
		}
		return z.Meta, nil
	}
	dg := buildSxnCodeDigraph(ctx, id.ZidSxnStart, getMeta)
	if dg == nil {
		return nil, wui.rootBinding, nil
	}
	dg = dg.AddVertex(id.ZidSxnBase).AddEdge(id.ZidSxnStart, id.ZidSxnBase)
	dg = dg.TransitiveClosure(id.ZidSxnStart)

	if zid, isDAG := dg.IsDAG(); !isDAG {
		return nil, nil, fmt.Errorf("zettel %v is part of a dependency cycle", zid)
	}
	bind := wui.rootBinding.MakeChildBinding("zettel", 128)
	for _, zid := range dg.SortReverse() {
		if err := wui.loadSxnCodeZettel(ctx, zid, bind); err != nil {
			return nil, nil, err
		}
	}
	return dg, bind, nil
}

type getMetaFunc func(context.Context, id.Zid) (*meta.Meta, error)

func buildSxnCodeDigraph(ctx context.Context, startZid id.Zid, getMeta getMetaFunc) graph.Digraph[id.Zid] {
	m, err := getMeta(ctx, startZid)
	if err != nil {
		return nil
	}
	var marked *idset.Set
	stack := []*meta.Meta{m}
	dg := graph.Digraph[id.Zid](nil).AddVertex(startZid)
	for pos := len(stack) - 1; pos >= 0; pos = len(stack) - 1 {
		curr := stack[pos]
		stack = stack[:pos]
		if marked.Contains(curr.Zid) {
			continue
		}
		marked = marked.Add(curr.Zid)
		for pre := range curr.GetFields(meta.KeyPredecessor) {
			if preZid, errParse := id.Parse(pre); errParse == nil {
				m, err = getMeta(ctx, preZid)
				if err != nil {
					continue
				}
				stack = append(stack, m)
				dg.AddVertex(preZid)
				dg.AddEdge(curr.Zid, preZid)
			}
		}
	}
	return dg
}

func (wui *WebUI) loadSxnCodeZettel(ctx context.Context, zid id.Zid, bind *sxeval.Binding) error {
	rdr, err := wui.makeZettelReader(ctx, zid)
	if err != nil {
		return err
	}
	env := sxeval.MakeEnvironment(bind)
	for {
		form, err2 := rdr.Read()
		if err2 != nil {
			if err2 == io.EOF {
				return nil
			}
			return err2
		}
		wui.logger.Debug("Loaded sxn code", "zid", zid, "form", form)

		if _, err2 = env.Eval(form, nil); err2 != nil {
			return err2
		}
	}
}
