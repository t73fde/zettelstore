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

// Package auth provides services for authentification / authorization.
package auth

import (
	"time"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/box"
	"zettelstore.de/z/config"
)

// BaseManager allows to check some base auth modes.
type BaseManager interface {
	// IsReadonly returns true, if the systems is configured to run in read-only-mode.
	IsReadonly() bool
}

// TokenManager provides methods to create authentication
type TokenManager interface {

	// GetToken produces a authentication token.
	GetToken(ident *meta.Meta, d time.Duration, kind TokenKind) ([]byte, error)

	// CheckToken checks the validity of the token and returns relevant data.
	CheckToken(token []byte, k TokenKind) (TokenData, error)
}

// TokenKind specifies for which application / usage a token is/was requested.
type TokenKind int

// Allowed values of token kind
const (
	_ TokenKind = iota
	KindAPI
	KindwebUI
)

// TokenData contains some important elements from a token.
type TokenData struct {
	Token   []byte
	Now     time.Time
	Issued  time.Time
	Expires time.Time
	Ident   string
	Zid     id.Zid
}

// AuthzManager provides methods for authorization.
type AuthzManager interface {
	BaseManager

	// Owner returns the zettel identifier of the owner.
	Owner() id.Zid

	// IsOwner returns true, if the given zettel identifier is that of the owner.
	IsOwner(zid id.Zid) bool

	// Returns true if authentication is enabled.
	WithAuth() bool

	// GetUserRole role returns the user role of the given user zettel.
	GetUserRole(user *meta.Meta) meta.UserRole
}

// Manager is the main interface for providing the service.
type Manager interface {
	TokenManager
	AuthzManager

	BoxWithPolicy(unprotectedBox box.Box, rtConfig config.Config) (box.Box, Policy)
}

// Policy is an interface for checking access authorization.
type Policy interface {
	// User is allowed to create a new zettel.
	CanCreate(user, newMeta *meta.Meta) bool

	// User is allowed to read zettel
	CanRead(user, m *meta.Meta) bool

	// User is allowed to write zettel.
	CanWrite(user, oldMeta, newMeta *meta.Meta) bool

	// User is allowed to delete zettel.
	CanDelete(user, m *meta.Meta) bool

	// User is allowed to refresh box data.
	CanRefresh(user *meta.Meta) bool
}
