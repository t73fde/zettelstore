//-----------------------------------------------------------------------------
// Copyright (c) 2023-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2023-present Detlef Stern
//-----------------------------------------------------------------------------

package impl

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	"encoding/base64"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxreader"
)

var encoding = base64.RawURLEncoding

const digestAlg = crypto.SHA384

func sign(claim sx.Object, secret []byte) ([]byte, error) {
	var buf bytes.Buffer
	_, err := sx.Print(&buf, claim)
	if err != nil {
		return nil, err
	}
	token := make([]byte, encoding.EncodedLen(buf.Len()))
	encoding.Encode(token, buf.Bytes())

	digest := hmac.New(digestAlg.New, secret)
	_, err = digest.Write(buf.Bytes())
	if err != nil {
		return nil, err
	}
	dig := digest.Sum(nil)
	encDig := make([]byte, encoding.EncodedLen(len(dig)))
	encoding.Encode(encDig, dig)

	token = append(token, '.')
	token = append(token, encDig...)
	return token, nil
}

func check(token []byte, secret []byte) (sx.Object, error) {
	i := bytes.IndexByte(token, '.')
	if i <= 0 || 1024 < i {
		return nil, ErrMalformedToken
	}
	buf := make([]byte, len(token))
	n, err := encoding.Decode(buf, token[:i])
	if err != nil {
		return nil, err
	}
	rdr := sxreader.MakeReader(bytes.NewReader(buf[:n]))
	obj, err := rdr.Read()
	if err != nil {
		return nil, err
	}

	var objBuf bytes.Buffer
	_, err = sx.Print(&objBuf, obj)
	if err != nil {
		return nil, err
	}

	digest := hmac.New(digestAlg.New, secret)
	_, err = digest.Write(objBuf.Bytes())
	if err != nil {
		return nil, err
	}

	n, err = encoding.Decode(buf, token[i+1:])
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(buf[:n], digest.Sum(nil)) {
		return nil, ErrMalformedToken
	}
	return obj, nil
}
