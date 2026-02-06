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

package webui

import (
	"net/http"

	"t73f.de/r/zsc/webapi"
)

func (wui *WebUI) redirectFound(w http.ResponseWriter, r *http.Request, ub *webapi.URLBuilder) {
	us := ub.String()
	wui.logger.Debug("redirect", "uri", us)
	http.Redirect(w, r, us, http.StatusFound)
}
