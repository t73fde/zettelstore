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

package box

import (
	"net/url"
	"strconv"
	"time"

	"zettelstore.de/z/zettel/id"
)

// GetNewZid calculates a new and unused zettel identifier, based on the current date and time.
func GetNewZid(testZid func(id.Zid) (bool, error)) (id.Zid, error) {
	withSeconds := false
	for range 90 { // Must be completed within 9 seconds (less than web/server.writeTimeout)
		zid := id.New(withSeconds)
		found, err := testZid(zid)
		if err != nil {
			return id.Invalid, err
		}
		if found {
			return zid, nil
		}
		// TODO: do not wait here unconditionally.
		time.Sleep(100 * time.Millisecond)
		withSeconds = true
	}
	return id.Invalid, ErrConflict
}

// GetQueryBool is a helper function to extract bool values from a box URI.
func GetQueryBool(u *url.URL, key string) bool {
	_, ok := u.Query()[key]
	return ok
}

// GetQueryInt is a helper function to extract int values of a specified range from a box URI.
func GetQueryInt(u *url.URL, key string, minVal, defVal, maxVal int) int {
	sVal := u.Query().Get(key)
	if sVal == "" {
		return defVal
	}
	iVal, err := strconv.Atoi(sVal)
	if err != nil {
		return defVal
	}
	if iVal < minVal {
		return minVal
	}
	if iVal > maxVal {
		return maxVal
	}
	return iVal
}
