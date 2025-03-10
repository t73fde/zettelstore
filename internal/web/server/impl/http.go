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

package impl

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Server timeout values
const (
	shutdownTimeout = 5 * time.Second
)

// httpServer is a HTTP server.
type httpServer struct {
	http.Server
}

// initializeHTTPServer creates a new HTTP server object.
func (srv *httpServer) initializeHTTPServer(addr string, handler http.Handler) {
	if addr == "" {
		addr = ":http"
	}
	srv.Server = http.Server{
		Addr:    addr,
		Handler: handler,
	}
}

// Run starts the web server, but does not wait for its completion.
func (srv *httpServer) Run() error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}

	go func() { srv.Serve(ln) }()
	return nil
}

// Stop the web server.
func (srv *httpServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	srv.Shutdown(ctx)
}
