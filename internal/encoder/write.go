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

package encoder

import (
	"encoding/base64"
	"io"
)

// EncWriter is a specialized writer for encoding zettel.
type EncWriter struct {
	w      io.Writer // The io.Writer to write to
	err    error     // Collect error
	length int       // Collected length
}

// NewEncWriter creates a new EncWriter
func NewEncWriter(w io.Writer) EncWriter {
	return EncWriter{w: w}
}

// Write writes the content of p.
func (w *EncWriter) Write(p []byte) (l int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	l, w.err = w.w.Write(p)
	w.length += l
	return l, w.err
}

// WriteString writes the content of s.
func (w *EncWriter) WriteString(s string) {
	if w.err != nil {
		return
	}
	var l int
	l, w.err = io.WriteString(w.w, s)
	w.length += l
}

// WriteStrings writes the content of sl.
func (w *EncWriter) WriteStrings(sl ...string) {
	for _, s := range sl {
		w.WriteString(s)
	}
}

// WriteByte writes the content of b.
func (w *EncWriter) WriteByte(b byte) error {
	var l int
	l, w.err = w.Write([]byte{b})
	w.length += l
	return w.err
}

// WriteBytes writes the content of bs.
func (w *EncWriter) WriteBytes(bs ...byte) {
	w.Write(bs)
}

// WriteBase64 writes the content of p, encoded with base64.
func (w *EncWriter) WriteBase64(p []byte) {
	if w.err == nil {
		encoder := base64.NewEncoder(base64.StdEncoding, w.w)
		var l int
		l, w.err = encoder.Write(p)
		w.length += l
		err1 := encoder.Close()
		if w.err == nil {
			w.err = err1
		}
	}
}

// Flush returns the collected length and error.
func (w *EncWriter) Flush() (int, error) { return w.length, w.err }
