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

	"t73f.de/r/zsc/webapi"

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
		h.Set(webapi.HeaderContentType, contentType)
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
	if eznf, isErr := errors.AsType[box.ErrZettelNotFound](err); isErr {
		return http.StatusNotFound, "Zettel not found: " + eznf.Zid.String()
	}
	if ena, isErr := errors.AsType[*box.ErrNotAllowed](err); isErr {
		msg := ena.Error()
		return http.StatusForbidden, strings.ToUpper(msg[:1]) + msg[1:]
	}
	if eiz, isErr := errors.AsType[box.ErrInvalidZid](err); isErr {
		return http.StatusBadRequest, fmt.Sprintf("Zettel-ID %q not appropriate in this context", eiz.Zid)
	}
	if etznf, isErr := errors.AsType[usecase.ErrTagZettelNotFound](err); isErr {
		return http.StatusNotFound, "Tag zettel not found: " + string(etznf.Tag)
	}
	if erznf, isErr := errors.AsType[usecase.ErrRoleZettelNotFound](err); isErr {
		return http.StatusNotFound, "Role zettel not found: " + string(erznf.Role)
	}
	if ebr, isErr := errors.AsType[ErrBadRequest](err); isErr {
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
	if ernf, isErr := errors.AsType[ErrResourceNotFound](err); isErr {
		return http.StatusNotFound, "Resource not found: " + ernf.Path
	}
	return http.StatusInternalServerError, err.Error()
}
