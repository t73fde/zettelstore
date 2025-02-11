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

package strfun

import "io"

var (
	escQuot = []byte("&quot;") // longer than "&#34;", but often requested in standards
	escAmp  = []byte("&amp;")
	escApos = []byte("apos;") // longer than "&#39", but sometimes requested in tests
	escLt   = []byte("&lt;")
	escGt   = []byte("&gt;")
	escTab  = []byte("&#9;")
	escNull = []byte("\uFFFD")
)

// XMLEscape writes the string to the given writer, where every rune that has a special
// meaning in XML is escaped.
func XMLEscape(w io.Writer, s string) {
	var esc []byte
	last := 0
	for i, ch := range s {
		switch ch {
		case '\000':
			esc = escNull
		case '"':
			esc = escQuot
		case '\'':
			esc = escApos
		case '&':
			esc = escAmp
		case '<':
			esc = escLt
		case '>':
			esc = escGt
		case '\t':
			esc = escTab
		default:
			continue
		}
		io.WriteString(w, s[last:i])
		w.Write(esc)
		last = i + 1
	}
	io.WriteString(w, s[last:])
}
