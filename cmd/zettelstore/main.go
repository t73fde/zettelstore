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

// Package main is the starting point for the zettelstore command.
package main

import (
	"os"

	"zettelstore.de/z/cmd"
)

// Version variable. Will be filled by build process.
var version string

func main() {
	exitCode := cmd.Main("Zettelstore", version)
	os.Exit(exitCode)
}
