//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

// Package user provides services for working with user data.
package user

import (
	"context"
	"time"

	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth"
)

// AuthData stores all relevant authentication data for a context.
type AuthData struct {
	User    *meta.Meta
	Token   []byte
	Now     time.Time
	Issued  time.Time
	Expires time.Time
}

// GetAuthData returns the full authentication data from the context.
func GetAuthData(ctx context.Context) *AuthData {
	if ctx != nil {
		if data, ok := ctx.Value(ctxKeyTypeSession{}).(*AuthData); ok {
			return data
		}
	}
	return nil
}

// GetCurrentUser returns the metadata of the current user, or nil if there is no one.
func GetCurrentUser(ctx context.Context) *meta.Meta {
	if data := GetAuthData(ctx); data != nil {
		return data.User
	}
	return nil
}

// ctxKeyTypeSession is just an additional type to make context value retrieval unambiguous.
type ctxKeyTypeSession struct{}

// UpdateContext enriches the given context with some data of the current user.
func UpdateContext(ctx context.Context, user *meta.Meta, data *auth.TokenData) context.Context {
	if data == nil {
		return context.WithValue(ctx, ctxKeyTypeSession{}, &AuthData{User: user})
	}
	return context.WithValue(
		ctx,
		ctxKeyTypeSession{},
		&AuthData{
			User:    user,
			Token:   data.Token,
			Now:     data.Now,
			Issued:  data.Issued,
			Expires: data.Expires,
		})
}
