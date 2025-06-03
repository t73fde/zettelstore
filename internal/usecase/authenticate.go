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

package usecase

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"t73f.de/r/webs/ip"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/auth/cred"
)

// Authenticate is the data for this use case.
type Authenticate struct {
	log       *slog.Logger
	token     auth.TokenManager
	ucGetUser *GetUser
}

// NewAuthenticate creates a new use case.
func NewAuthenticate(log *slog.Logger, token auth.TokenManager, ucGetUser *GetUser) Authenticate {
	return Authenticate{
		log:       log,
		token:     token,
		ucGetUser: ucGetUser,
	}
}

// Run executes the use case.
//
// Parameter "r" is just included to produce better logging messages. It may be nil. Do not use it
// for other purposes.
func (uc *Authenticate) Run(ctx context.Context, r *http.Request, ident, credential string, d time.Duration, k auth.TokenKind) ([]byte, error) {
	identMeta, err := uc.ucGetUser.Run(ctx, ident)
	defer addDelay(time.Now(), 500*time.Millisecond, 100*time.Millisecond)

	if identMeta == nil || err != nil {
		uc.log.Info("No user with given ident found", "ident", ident, "err", err, "remote", ip.GetRemoteAddr(r))
		compensateCompare()
		return nil, err
	}

	if hashCred, ok := identMeta.Get(meta.KeyCredential); ok {
		ok, err = cred.CompareHashAndCredential(string(hashCred), identMeta.Zid, ident, credential)
		if err != nil {
			uc.log.Info("Error while comparing credentials", "ident", ident, "err", err, "remote", ip.GetRemoteAddr(r))
			return nil, err
		}
		if ok {
			token, err2 := uc.token.GetToken(identMeta, d, k)
			if err2 != nil {
				uc.log.Info("Unable to produce authentication token", "ident", ident, "err", err2)
				return nil, err2
			}
			uc.log.Info("Successful", "user", ident)
			return token, nil
		}
		uc.log.Info("Credentials don't match", "ident", ident, "remote", ip.GetRemoteAddr(r))
		return nil, nil
	}
	uc.log.Info("No credential stored", "ident", ident)
	compensateCompare()
	return nil, nil
}

// compensateCompare if normal comapare is not possible, to avoid timing hints.
func compensateCompare() {
	_, _ = cred.CompareHashAndCredential(
		"$2a$10$WHcSO3G9afJ3zlOYQR1suuf83bCXED2jmzjti/MH4YH4l2mivDuze", id.Invalid, "", "")
}

// addDelay after credential checking to allow some CPU time for other tasks.
// durDelay is the normal delay, if time spend for checking is smaller than
// the minimum delay minDelay. In addition some jitter (+/- 50 ms) is added.
func addDelay(start time.Time, durDelay, minDelay time.Duration) {
	jitter := time.Duration(rand.IntN(100)-50) * time.Millisecond
	if elapsed := time.Since(start); elapsed+minDelay < durDelay {
		time.Sleep(durDelay - elapsed + jitter)
	} else {
		time.Sleep(minDelay + jitter)
	}
}

// IsAuthenticatedPort contains method for this usecase.
type IsAuthenticatedPort interface {
	GetCurrentUser(context.Context) *meta.Meta
}

// IsAuthenticated cheks if the caller is already authenticated.
type IsAuthenticated struct {
	logger *slog.Logger
	port   IsAuthenticatedPort
	authz  auth.AuthzManager
}

// NewIsAuthenticated creates a new use case object.
func NewIsAuthenticated(logger *slog.Logger, port IsAuthenticatedPort, authz auth.AuthzManager) IsAuthenticated {
	return IsAuthenticated{
		logger: logger,
		port:   port,
		authz:  authz,
	}
}

// IsAuthenticatedResult is an enumeration.
type IsAuthenticatedResult uint8

// Values for IsAuthenticatedResult.
const (
	_ IsAuthenticatedResult = iota
	IsAuthenticatedDisabled
	IsAuthenticatedAndValid
	IsAuthenticatedAndInvalid
)

// Run executes the use case.
func (uc *IsAuthenticated) Run(ctx context.Context) IsAuthenticatedResult {
	if !uc.authz.WithAuth() {
		uc.logger.Info("IsAuthenticated", "auth", "disabled")
		return IsAuthenticatedDisabled
	}
	if uc.port.GetCurrentUser(ctx) == nil {
		uc.logger.Info("IsAuthenticated is false")
		return IsAuthenticatedAndInvalid
	}
	uc.logger.Info("IsAuthenticated is true")
	return IsAuthenticatedAndValid
}
