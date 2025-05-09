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

// Package main provides a command to test the API
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"zettelstore.de/z/tools"
)

func main() {
	flag.BoolVar(&tools.Verbose, "v", false, "Verbose output")
	flag.Parse()

	if err := cmdTestAPI(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

type zsInfo struct {
	cmd          *exec.Cmd
	out          strings.Builder
	adminAddress string
}

func cmdTestAPI() error {
	var err error
	var info zsInfo
	needServer := !addressInUse(":23123")
	if needServer {
		err = startZettelstore(&info)
	}
	if err != nil {
		return err
	}
	err = tools.CheckGoTest("zettelstore.de/z/tests/client", "-base-url", "http://127.0.0.1:23123")
	if needServer {
		err1 := stopZettelstore(&info)
		if err == nil {
			err = err1
		}
	}
	return err
}

func startZettelstore(info *zsInfo) error {
	info.adminAddress = ":2323"
	name, arg := "go", []string{
		"run", "cmd/zettelstore/main.go", "run",
		"-c", "./testdata/testbox/19700101000000.zettel", "-a", info.adminAddress[1:]}
	tools.LogCommand("FORK", nil, name, arg)
	cmd := tools.PrepareCommand(tools.EnvGoVCS, name, arg, nil, &info.out, os.Stderr)
	if !tools.Verbose {
		cmd.Stderr = nil
	}
	err := cmd.Start()
	time.Sleep(2 * time.Second)
	for range 100 {
		time.Sleep(time.Millisecond * 100)
		if addressInUse(info.adminAddress) {
			info.cmd = cmd
			return err
		}
	}
	time.Sleep(4 * time.Second) // Wait for all zettel to be indexed.
	return errors.New("zettelstore did not start")
}

func stopZettelstore(i *zsInfo) error {
	conn, err := net.Dial("tcp", i.adminAddress)
	if err != nil {
		fmt.Println("Unable to stop Zettelstore")
		return err
	}
	_, err = io.WriteString(conn, "shutdown\n")
	_ = conn.Close()
	if err == nil {
		err = i.cmd.Wait()
	}
	return err
}

func addressInUse(address string) bool {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
