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

import (
	"regexp"
	"strconv"

	"zettelstore.de/z/internal/kernel"
)

// Version is the data for this use case.
type Version struct {
	vr VersionResult
}

// NewVersion creates a new use case.
func NewVersion(version string) Version {
	return Version{calculateVersionResult(version)}
}

// VersionResult is the data structure returned by this usecase.
type VersionResult struct {
	Major int
	Minor int
	Patch int
	Info  string
	Hash  string
}

var invalidVersion = VersionResult{
	Major: -1,
	Minor: -1,
	Patch: -1,
	Info:  kernel.CoreDefaultVersion,
	Hash:  "",
}

var reVersion = regexp.MustCompile(`^(\d+)\.(\d+)(\.(\d+))?(-(([[:alnum:]]|-)+))?(\+(([[:alnum:]])+(-[[:alnum:]]+)?))?`)

func calculateVersionResult(version string) VersionResult {
	match := reVersion.FindStringSubmatch(version)
	if len(match) < 12 {
		return invalidVersion
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return invalidVersion
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return invalidVersion
	}
	patch, err := strconv.Atoi(match[4])
	if err != nil {
		patch = 0
	}
	return VersionResult{
		Major: major,
		Minor: minor,
		Patch: patch,
		Info:  match[6],
		Hash:  match[9],
	}
}

// Run executes the use case.
func (uc Version) Run() VersionResult { return uc.vr }
