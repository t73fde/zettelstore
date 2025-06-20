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

package webui

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"zettelstore.de/z/internal/web/adapter"
)

// MakeFaviconHandler creates a HTTP handler to retrieve the favicon.
func (wui *WebUI) MakeFaviconHandler(baseDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		filename := filepath.Join(baseDir, "favicon.ico")
		f, err := os.Open(filename)
		if err != nil {
			wui.logger.Debug("Favicon not found", "err", err)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		defer func() { _ = f.Close() }()

		data, err := io.ReadAll(f)
		if err != nil {
			wui.logger.Error("Unable to read favicon data", "err", err)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		if err = adapter.WriteData(w, data, ""); err != nil {
			wui.logger.Error("Write favicon", "err", err)
		}
	})
}
