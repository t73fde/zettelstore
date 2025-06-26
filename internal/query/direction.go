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

import "t73f.de/r/zsc/api"

type directionSpec struct {
	isDirected bool
	isForward  bool
	isBackward bool
}

func (ds *directionSpec) cleanupAfterParse() {
	if !ds.isForward && !ds.isBackward {
		ds.isForward, ds.isBackward = true, true
	} else if ds.isForward && ds.isBackward {
		ds.isDirected = true
	}
}

func (ds directionSpec) print(pe *PrintEnv) {
	if ds.isDirected {
		pe.printSpace()
		pe.writeString(api.DirectedDirective)
	} else if ds.isForward {
		if !ds.isBackward {
			pe.printSpace()
			pe.writeString(api.ForwardDirective)
		}
	} else if ds.isBackward {
		pe.printSpace()
		pe.writeString(api.BackwardDirective)
	} else {
		panic("neither forward, backward, nor directed")
	}
}
