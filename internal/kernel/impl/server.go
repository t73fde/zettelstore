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

package impl

import (
	"bufio"
	"net"
)

func startLineServer(kern *myKernel, listenAddr string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		kern.logger.Error().Err(err).Msg("Unable to start administration console")
		return err
	}
	kern.logger.Mandatory().Str("listen", listenAddr).Msg("Start administration console")
	go func() { lineServer(ln, kern) }()
	return nil
}

func lineServer(ln net.Listener, kern *myKernel) {
	// Something may panic. Ensure a running line service.
	defer func() {
		if ri := recover(); ri != nil {
			kern.doLogRecover("Line", ri)
			go lineServer(ln, kern)
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
			kern.logger.Error().Err(err).Msg("Unable to accept connection")
			break
		}
		go handleLineConnection(conn, kern)
	}
	ln.Close()
}

func handleLineConnection(conn net.Conn, kern *myKernel) {
	// Something may panic. Ensure a running connection.
	defer func() {
		if ri := recover(); ri != nil {
			kern.doLogRecover("LineConn", ri)
			go handleLineConnection(conn, kern)
		}
	}()

	kern.logger.Mandatory().Str("from", conn.RemoteAddr().String()).Msg("Start session on administration console")
	cmds := cmdSession{}
	cmds.initialize(conn, kern)
	s := bufio.NewScanner(conn)
	for s.Scan() {
		line := s.Text()
		if !cmds.executeLine(line) {
			break
		}
	}
	conn.Close()
}
