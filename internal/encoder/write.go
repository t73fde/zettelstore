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
	"io"

	"t73f.de/r/zsc/domain/meta"
)

// encWriter is a specialized writer for encoding zettel.
type encWriter struct {
	w   io.Writer // The io.Writer to write to
	err error     // Collect error
}

// newEncWriter creates a new encWriter
func newEncWriter(w io.Writer) encWriter { return encWriter{w: w} }

// Write writes the content of p.
func (w *encWriter) Write(p []byte) (l int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	l, w.err = w.w.Write(p)
	return l, w.err
}

// WriteString writes the content of s.
func (w *encWriter) WriteString(s string) {
	if w.err != nil {
		return
	}
	_, w.err = io.WriteString(w.w, s)
}

// WriteStrings writes the content of sl.
func (w *encWriter) WriteStrings(sl ...string) {
	for _, s := range sl {
		w.WriteString(s)
	}
}

// WriteByte writes the content of b.
func (w *encWriter) WriteByte(b byte) error {
	if w.err == nil {
		_, w.err = w.Write([]byte{b})
	}
	return w.err
}

// WriteBytes writes the content of bs.
func (w *encWriter) WriteBytes(bs ...byte) { _, _ = w.Write(bs) }

// WriteLn writes a new line character.
func (w *encWriter) WriteLn() {
	if w.err == nil {
		w.err = w.WriteByte('\n')
	}
}

// WriteLn writes a space character.
func (w *encWriter) WriteSpace() {
	if w.err == nil {
		w.err = w.WriteByte(' ')
	}
}

// WriteMeta write the content of the meta data in a standard way:
//
//	key: val
func (w *encWriter) WriteMeta(m *meta.Meta) {
	for key, val := range m.Computed() {
		w.WriteStrings(key, ": ", string(val), "\n")
	}
}

// Flush returns the collected length and error.
func (w *encWriter) Flush() error { return w.err }
