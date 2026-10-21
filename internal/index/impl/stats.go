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
	mx      sync.RWMutex
	updates uint64
	removes uint64
}

func (st *stats) finishUpdateZettel() {
	st.mx.Lock()
	defer st.mx.Unlock()
	st.updates++
}

func (st *stats) finishRemoveZettel() {
	st.mx.Lock()
	defer st.mx.Unlock()
	st.removes++
}

func (st *stats) read() (updates, removes uint64) {
	st.mx.RLock()
	defer st.mx.RUnlock()
	return st.updates, st.removes
}
