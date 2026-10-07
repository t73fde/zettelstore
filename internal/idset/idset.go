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

// Package idset implements sets of zettel identifier.
package idset

import (
	"t73f.de/r/zero/roster"
	"t73f.de/r/zsc/domain/id"
)

// ZidSet is a set of zettel identifier, stored as an sorted array.
type ZidSet = roster.Roster[id.Zid]
