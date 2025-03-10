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

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"t73f.de/r/zsc/api"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/usecase"
)

// WriteData emits the given data to the response writer.
func WriteData(w http.ResponseWriter, data []byte, contentType string) error {
	if len(data) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	PrepareHeader(w, contentType)
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(data)
	return err
}

// PrepareHeader sets the HTTP header to defined values.
func PrepareHeader(w http.ResponseWriter, contentType string) http.Header {
	h := w.Header()
	if contentType != "" {
		h.Set(api.HeaderContentType, contentType)
	}
	return h
}

// ErrBadRequest is returned if the caller made an invalid HTTP request.
type ErrBadRequest struct {
	Text string
}

// NewErrBadRequest creates an new bad request error.
func NewErrBadRequest(text string) error { return ErrBadRequest{Text: text} }

func (err ErrBadRequest) Error() string { return err.Text }

// CodeMessageFromError returns an appropriate HTTP status code and text from a given error.
func CodeMessageFromError(err error) (int, string) {
	var eznf box.ErrZettelNotFound
	if errors.As(err, &eznf) {
		return http.StatusNotFound, "Zettel not found: " + eznf.Zid.String()
	}
	var ena *box.ErrNotAllowed
	if errors.As(err, &ena) {
		msg := ena.Error()
		return http.StatusForbidden, strings.ToUpper(msg[:1]) + msg[1:]
	}
	var eiz box.ErrInvalidZid
	if errors.As(err, &eiz) {
		return http.StatusBadRequest, fmt.Sprintf("Zettel-ID %q not appropriate in this context", eiz.Zid)
	}
	var etznf usecase.ErrTagZettelNotFound
	if errors.As(err, &etznf) {
		return http.StatusNotFound, "Tag zettel not found: " + string(etznf.Tag)
	}
	var erznf usecase.ErrRoleZettelNotFound
	if errors.As(err, &erznf) {
		return http.StatusNotFound, "Role zettel not found: " + string(erznf.Role)
	}
	var ebr ErrBadRequest
	if errors.As(err, &ebr) {
		return http.StatusBadRequest, ebr.Text
	}
	if errors.Is(err, box.ErrStopped) {
		return http.StatusInternalServerError, fmt.Sprintf("Zettelstore not operational: %v", err)
	}
	if errors.Is(err, box.ErrConflict) {
		return http.StatusConflict, "Zettelstore operations conflicted"
	}
	if errors.Is(err, box.ErrCapacity) {
		return http.StatusInsufficientStorage, "Zettelstore reached one of its storage limits"
	}
	var ernf ErrResourceNotFound
	if errors.As(err, &ernf) {
		return http.StatusNotFound, "Resource not found: " + ernf.Path
	}
	return http.StatusInternalServerError, err.Error()
}
