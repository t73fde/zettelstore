//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

// Package zettel provides specific types, constants, and functions for zettel.
package zettel

import "t73f.de/r/zsc/domain/meta"

// Zettel is the main data object of a zettelstore.
type Zettel struct {
	Meta    *meta.Meta // Some additional meta-data.
	Content Content    // The content of the zettel itself.
}

// Length returns the number of bytes to store the zettel (in a zettel view,
// not in a technical view).
func (z Zettel) Length() int { return z.Meta.Length() + z.Content.Length() }

// Equal compares two zettel for equality.
func (z Zettel) Equal(o Zettel, allowComputed bool) bool {
	return z.Meta.Equal(o.Meta, allowComputed) && z.Content.Equal(&o.Content)
}
