//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package kernel

import (
	"bufio"
	"net"

	"zettelstore.de/z/internal/logging"
)

func startLineServer(kern *Kernel, listenAddr string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		kern.logger.Error("Unable to start administration console", "err", err)
		return err
	}
	logging.LogMandatory(kern.logger, "Start administration console", "listen", listenAddr)
	go func() { lineServer(ln, kern) }()
	return nil
}

func lineServer(ln net.Listener, kern *Kernel) {
	// Something may panic. Ensure a running line service.
	defer func() {
		if ri := recover(); ri != nil {
			kern.LogRecover("Line", ri)
			go lineServer(ln, kern)
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
			kern.logger.Error("Unable to accept connection", "err", err)
			break
		}
		go handleLineConnection(conn, kern)
	}
	_ = ln.Close()
}

func handleLineConnection(conn net.Conn, kern *Kernel) {
	// Something may panic. Ensure a running connection.
	defer func() {
		if ri := recover(); ri != nil {
			kern.LogRecover("LineConn", ri)
			go handleLineConnection(conn, kern)
		}
	}()

	logging.LogMandatory(kern.logger, "Start session on administration console", "from", conn.RemoteAddr().String())
	cmds := cmdSession{}
	cmds.initialize(conn, kern)
	s := bufio.NewScanner(conn)
	for s.Scan() {
		line := s.Text()
		if !cmds.executeLine(line) {
			break
		}
	}
	_ = conn.Close()
}
