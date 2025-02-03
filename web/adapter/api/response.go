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

package api

import (
	"bytes"
	"net/http"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/web/content"
)

func (a *API) writeObject(w http.ResponseWriter, zid id.Zid, obj sx.Object) error {
	var buf bytes.Buffer
	if _, err := sx.Print(&buf, obj); err != nil {
		msg := a.log.Error().Err(err)
		if msg != nil {
			if zid.IsValid() {
				msg = msg.Zid(zid)
			}
			msg.Msg("Unable to store object in buffer")
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil
	}
	return writeBuffer(w, &buf, content.SXPF)
}
