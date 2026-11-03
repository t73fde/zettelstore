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
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
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
	var info zsInfo
	needServer := !addressInUse(":23123")
	if needServer {
		if err := startZettelstore(&info); err != nil {
			return err
		}
	}
	err := tools.CheckGoTest("zettelstore.de/z/tests/client", "-base-url", "http://127.0.0.1:23123")
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
	if err := cmd.Start(); err != nil {
		return err
	}
	return zsIsReady()
}

func stopZettelstore(i *zsInfo) error {
	conn, err := net.Dial("tcp", i.adminAddress)
	if err != nil {
		fmt.Println("Unable to stop Zettelstore")
		return err
	}
	_, err = io.WriteString(conn, "shutdown\n")
	_ = conn.Close()
	if err == nil && i.cmd != nil {
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

func zsIsReady() error {
	ctx := context.Background()
	client := http.Client{Timeout: 5 * time.Second}
	maxTime := time.Now().Add(1 * time.Minute)
	for {
		if time.Now().After(maxTime) {
			return fmt.Errorf("No ready timeout")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:23123/x?ready", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK, http.StatusNoContent:
			return nil
		case http.StatusServiceUnavailable:
			time.Sleep(1 * time.Second)
		default:
			return fmt.Errorf("unexpected status: %s", resp.Status)
		}
	}
}
