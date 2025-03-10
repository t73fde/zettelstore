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

package webui

import (
	"net/http"

	"zettelstore.de/z/internal/usecase"
)

// MakeGetGoActionHandler creates a new HTTP handler to execute certain commands.
func (wui *WebUI) MakeGetGoActionHandler(ucRefresh *usecase.Refresh) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Currently, command "refresh" is the only command to be executed.
		err := ucRefresh.Run(ctx)
		if err != nil {
			wui.reportError(ctx, w, err)
			return
		}
		wui.redirectFound(w, r, wui.NewURLBuilder('/'))
	})
}
