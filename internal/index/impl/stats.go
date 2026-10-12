//-----------------------------------------------------------------------------
// Copyright (c) 2026-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2026-present Detlef Stern
//-----------------------------------------------------------------------------

package impl

import "sync"

type stats struct {
	mx sync.RWMutex

	maxGen  uint64
	updates uint64
	removes uint64
}

func (st *stats) finishUpdateZettel(generation uint64) {
	st.mx.Lock()
	defer st.mx.Unlock()
	st.maxGen = max(st.maxGen, generation)
	st.updates++
}

func (st *stats) finishRemoveZettel(generation uint64) {
	st.mx.Lock()
	defer st.mx.Unlock()
	st.maxGen = max(st.maxGen, generation)
	st.removes++
}
