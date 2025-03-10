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

package adapter

import "net/http"

// BadRequest signals HTTP status code 400.
func BadRequest(w http.ResponseWriter, text string) {
	http.Error(w, text, http.StatusBadRequest)
}

// ErrResourceNotFound is signalled when a web resource was not found.
type ErrResourceNotFound struct{ Path string }

func (ernf ErrResourceNotFound) Error() string { return "resource not found: " + ernf.Path }
