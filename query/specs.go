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

import "t73f.de/r/zsc/api"

// IdentSpec contains all specification values to calculate the ident directive.
type IdentSpec struct{}

// Print the spec on the given print environment.
func (spec *IdentSpec) Print(pe *PrintEnv) {
	pe.printSpace()
	pe.writeString(api.IdentDirective)
}

// ItemsSpec contains all specification values to calculate items.
type ItemsSpec struct{}

// Print the spec on the given print environment.
func (spec *ItemsSpec) Print(pe *PrintEnv) {
	pe.printSpace()
	pe.writeString(api.ItemsDirective)
}
