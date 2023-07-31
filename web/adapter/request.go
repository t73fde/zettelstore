//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package adapter

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"zettelstore.de/client.fossil/api"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/query"
)

// GetCredentialsViaForm retrieves the authentication credentions from a form.
func GetCredentialsViaForm(r *http.Request) (ident, cred string, ok bool) {
	err := r.ParseForm()
	if err != nil {
		kernel.Main.GetLogger(kernel.WebService).Info().Err(err).Msg("Unable to parse form")
		return "", "", false
	}

	ident = strings.TrimSpace(r.PostFormValue("username"))
	cred = r.PostFormValue("password")
	if ident == "" {
		return "", "", false
	}
	return ident, cred, true
}

// GetQuery retrieves the specified options from a query.
func GetQuery(vals url.Values) (result *query.Query) {
	if exprs, found := vals[api.QueryKeyQuery]; found {
		result = query.Parse(strings.Join(exprs, " "))
	}
	if seeds, found := vals[api.QueryKeySeed]; found {
		for _, seed := range seeds {
			if si, err := strconv.ParseInt(seed, 10, 31); err == nil {
				result = result.SetSeed(int(si))
				break
			}
		}
	}
	return result
}
