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

// Package tools provides a collection of functions to build needed tools.
package tools

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"zettelstore.de/z/strfun"
)

var EnvDirectProxy = []string{"GOPROXY=direct"}
var EnvGoVCS = []string{"GOVCS=zettelstore.de:fossil,t73f.de:fossil"}
var Verbose bool

func ExecuteCommand(env []string, name string, arg ...string) (string, error) {
	LogCommand("EXEC", env, name, arg)
	var out strings.Builder
	cmd := PrepareCommand(env, name, arg, nil, &out, os.Stderr)
	err := cmd.Run()
	return out.String(), err
}

func ExecuteFilter(data []byte, env []string, name string, arg ...string) (string, string, error) {
	LogCommand("EXEC", env, name, arg)
	var stdout, stderr strings.Builder
	cmd := PrepareCommand(env, name, arg, bytes.NewReader(data), &stdout, &stderr)
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func PrepareCommand(env []string, name string, arg []string, in io.Reader, stdout, stderr io.Writer) *exec.Cmd {
	if len(env) > 0 {
		env = append(env, os.Environ()...)
	}
	cmd := exec.Command(name, arg...)
	cmd.Env = env
	cmd.Stdin = in
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd
}
func LogCommand(exec string, env []string, name string, arg []string) {
	if Verbose {
		if len(env) > 0 {
			for i, e := range env {
				fmt.Fprintf(os.Stderr, "ENV%d %v\n", i+1, e)
			}
		}
		fmt.Fprintln(os.Stderr, exec, name, arg)
	}
}

func Check(forRelease bool) error {
	if err := CheckGoTest("./..."); err != nil {
		return err
	}
	if err := checkGoVet(); err != nil {
		return err
	}
	if err := checkShadow(forRelease); err != nil {
		return err
	}
	if err := checkStaticcheck(); err != nil {
		return err
	}
	if err := checkUnparam(forRelease); err != nil {
		return err
	}
	if forRelease {
		if err := checkGoVulncheck(); err != nil {
			return err
		}
	}
	return checkFossilExtra()
}

func CheckGoTest(pkg string, testParams ...string) error {
	var env []string
	env = append(env, EnvDirectProxy...)
	env = append(env, EnvGoVCS...)
	args := []string{"test", pkg}
	args = append(args, testParams...)
	out, err := ExecuteCommand(env, "go", args...)
	if err != nil {
		for _, line := range strfun.SplitLines(out) {
			if strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "?") {
				continue
			}
			fmt.Fprintln(os.Stderr, line)
		}
	}
	return err
}
func checkGoVet() error {
	out, err := ExecuteCommand(EnvGoVCS, "go", "vet", "./...")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Some checks failed")
		if len(out) > 0 {
			fmt.Fprintln(os.Stderr, out)
		}
	}
	return err
}

func checkShadow(forRelease bool) error {
	path, err := findExecStrict("shadow", forRelease)
	if path == "" {
		return err
	}
	out, err := ExecuteCommand(EnvGoVCS, path, "-strict", "./...")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Some shadowed variables found")
		if len(out) > 0 {
			fmt.Fprintln(os.Stderr, out)
		}
	}
	return err
}

func checkStaticcheck() error {
	out, err := ExecuteCommand(EnvGoVCS, "staticcheck", "./...")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Some staticcheck problems found")
		if len(out) > 0 {
			fmt.Fprintln(os.Stderr, out)
		}
	}
	return err
}

func checkUnparam(forRelease bool) error {
	path, err := findExecStrict("unparam", forRelease)
	if path == "" {
		return err
	}
	out, err := ExecuteCommand(EnvGoVCS, path, "./...")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Some unparam problems found")
		if len(out) > 0 {
			fmt.Fprintln(os.Stderr, out)
		}
	}
	if forRelease {
		if out2, err2 := ExecuteCommand(nil, path, "-exported", "-tests", "./..."); err2 != nil {
			fmt.Fprintln(os.Stderr, "Some optional unparam problems found")
			if len(out2) > 0 {
				fmt.Fprintln(os.Stderr, out2)
			}
		}
	}
	return err
}

func checkGoVulncheck() error {
	out, err := ExecuteCommand(EnvGoVCS, "govulncheck", "./...")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Some checks failed")
		if len(out) > 0 {
			fmt.Fprintln(os.Stderr, out)
		}
	}
	return err
}
func findExec(cmd string) string {
	if path, err := ExecuteCommand(nil, "which", cmd); err == nil && path != "" {
		return strings.TrimSpace(path)
	}
	return ""
}

func findExecStrict(cmd string, forRelease bool) (string, error) {
	path := findExec(cmd)
	if path != "" || !forRelease {
		return path, nil
	}
	return "", errors.New("Command '" + cmd + "' not installed, but required for release")
}

func checkFossilExtra() error {
	out, err := ExecuteCommand(nil, "fossil", "extra")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to execute 'fossil extra'")
		return err
	}
	if len(out) > 0 {
		fmt.Fprint(os.Stderr, "Warning: unversioned file(s):")
		for i, extra := range strfun.SplitLines(out) {
			if i > 0 {
				fmt.Fprint(os.Stderr, ",")
			}
			fmt.Fprintf(os.Stderr, " %q", extra)
		}
		fmt.Fprintln(os.Stderr)
	}
	return nil
}
