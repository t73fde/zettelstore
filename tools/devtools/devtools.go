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

// Package main provides a command to install development tools.
package main

import (
	"flag"
	"fmt"
	"os"

	"zettelstore.de/z/tools"
)

func main() {
	flag.BoolVar(&tools.Verbose, "v", false, "Verbose output")
	flag.Parse()

	if err := cmdTools(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func cmdTools() error {
	tools := []struct{ name, pack string }{
		{"shadow", "golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest"},
		{"unparam", "mvdan.cc/unparam@latest"},
		{"staticcheck", "honnef.co/go/tools/cmd/staticcheck@latest"},
		{"govulncheck", "golang.org/x/vuln/cmd/govulncheck@latest"},
		{"deadcode", "golang.org/x/tools/cmd/deadcode@latest"},
		{"errcheck", "github.com/kisielk/errcheck@latest"},
		{"revive", "github.com/mgechev/revive@latest"},
	}
	for _, tool := range tools {
		err := doGoInstall(tool.pack)
		if err != nil {
			return err
		}
	}
	return nil
}
func doGoInstall(pack string) error {
	out, err := tools.ExecuteCommand(nil, "go", "install", pack)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to install package", pack)
		if len(out) > 0 {
			fmt.Fprintln(os.Stderr, out)
		}
	}
	return err
}
