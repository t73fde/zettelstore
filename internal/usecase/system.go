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

package usecase

// ----- Refresh all zettel ----------

// IsReadyPort is the interface used by this use case.
type IsReadyPort interface {
	IsReady() bool
}

// IsReady is the data for this use case.
type IsReady struct {
	port IsReadyPort
}

// NewIsReady creates a new use case.
func NewIsReady(port IsReadyPort) IsReady {
	return IsReady{port: port}
}

// Run executes the use case.
func (uc IsReady) Run() bool {
	return uc.port.IsReady()
}
