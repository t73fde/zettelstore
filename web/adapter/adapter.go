//-----------------------------------------------------------------------------
// Copyright (c) 2024-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2024-present Detlef Stern
//-----------------------------------------------------------------------------

// Package adapter provides handlers for web requests, and some helper tools.
package adapter

import (
	"context"

	"zettelstore.de/z/usecase"
	"zettelstore.de/z/zettel/meta"
)

// TryReIndex executes a re-index if the appropriate query action is given.
func TryReIndex(ctx context.Context, actions []string, metaSeq []*meta.Meta, reIndex *usecase.ReIndex) ([]string, error) {
	if lenActions := len(actions); lenActions > 0 {
		tempActions := make([]string, 0, lenActions)
		hasReIndex := false
		for _, act := range actions {
			if !hasReIndex && act == "REINDEX" {
				hasReIndex = true
				var errAction error
				for _, m := range metaSeq {
					if err := reIndex.Run(ctx, m.Zid); err != nil {
						errAction = err
					}
				}
				if errAction != nil {
					return nil, errAction
				}
				continue
			}
			tempActions = append(tempActions, act)
		}
		return tempActions, nil
	}
	return nil, nil
}
