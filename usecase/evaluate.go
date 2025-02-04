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

package usecase

import (
	"context"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/config"
	"zettelstore.de/z/evaluator"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel"
)

// Evaluate is the data for this use case.
type Evaluate struct {
	rtConfig    config.Config
	ucGetZettel *GetZettel
	ucQuery     *Query
}

// NewEvaluate creates a new use case.
func NewEvaluate(rtConfig config.Config, ucGetZettel *GetZettel, ucQuery *Query) Evaluate {
	return Evaluate{
		rtConfig:    rtConfig,
		ucGetZettel: ucGetZettel,
		ucQuery:     ucQuery,
	}
}

// Run executes the use case.
func (uc *Evaluate) Run(ctx context.Context, zid id.Zid, syntax string) (*ast.ZettelNode, error) {
	zettel, err := uc.ucGetZettel.Run(ctx, zid)
	if err != nil {
		return nil, err
	}
	return uc.RunZettel(ctx, zettel, syntax), nil
}

// RunZettel executes the use case for a given zettel.
func (uc *Evaluate) RunZettel(ctx context.Context, zettel zettel.Zettel, syntax string) *ast.ZettelNode {
	zn := parser.ParseZettel(ctx, zettel, syntax, uc.rtConfig)
	evaluator.EvaluateZettel(ctx, uc, uc.rtConfig, zn)
	return zn
}

// RunBlockNode executes the use case for a metadata list.
func (uc *Evaluate) RunBlockNode(ctx context.Context, bn ast.BlockNode) ast.BlockSlice {
	if bn == nil {
		return nil
	}
	bns := ast.BlockSlice{bn}
	evaluator.EvaluateBlock(ctx, uc, uc.rtConfig, &bns)
	return bns
}

// GetZettel retrieves the full zettel of a given zettel identifier.
func (uc *Evaluate) GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error) {
	return uc.ucGetZettel.Run(ctx, zid)
}

// QueryMeta returns a list of metadata that comply to the given selection criteria.
func (uc *Evaluate) QueryMeta(ctx context.Context, q *query.Query) ([]*meta.Meta, error) {
	return uc.ucQuery.Run(ctx, q)
}
