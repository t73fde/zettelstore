//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package usecase

import "t73f.de/r/zero/semver"

// Version is the data for this use case.
type Version struct {
	v semver.SemVer
}

// NewVersion creates a new use case.
func NewVersion(version semver.SemVer) Version {
	return Version{version}
}

// Run executes the use case.
func (uc Version) Run() semver.SemVer { return uc.v }
