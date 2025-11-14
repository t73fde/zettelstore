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

package auth

import (
	"bytes"

	"golang.org/x/crypto/bcrypt"

	"t73f.de/r/zsc/domain/id"
)

// HashCredential returns a hashed vesion of the given credential
func HashCredential(zid id.Zid, ident, credential string) (string, error) {
	fullCredential := createFullCredential(zid, ident, credential)
	res, err := bcrypt.GenerateFromPassword(fullCredential, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

// CompareHashAndCredential checks, whether the hashed credential is a possible
// value when hashing the credential.
func CompareHashAndCredential(hashed string, zid id.Zid, ident, credential string) (bool, error) {
	fullCredential := createFullCredential(zid, ident, credential)
	err := bcrypt.CompareHashAndPassword([]byte(hashed), fullCredential)
	if err == nil {
		return true, nil
	}
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	return false, err
}

func createFullCredential(zid id.Zid, ident, credential string) []byte {
	var buf bytes.Buffer
	buf.Write(zid.Bytes())
	buf.WriteByte(' ')
	buf.WriteString(ident)
	buf.WriteByte(' ')
	buf.WriteString(credential)
	return buf.Bytes()
}
