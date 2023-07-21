//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package usecase

import (
	"context"
	"math/rand"
	"net/http"
	"time"

	"zettelstore.de/c/api"
	"zettelstore.de/z/auth"
	"zettelstore.de/z/auth/cred"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// AuthenticatePort is the interface used by this use case.
type AuthenticatePort interface {
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)
	SelectMeta(context.Context, *query.Query) ([]*meta.Meta, error)
}

// Authenticate is the data for this use case.
type Authenticate struct {
	log       *logger.Logger
	token     auth.TokenManager
	port      AuthenticatePort
	ucGetUser GetUser
}

// NewAuthenticate creates a new use case.
func NewAuthenticate(log *logger.Logger, token auth.TokenManager, authz auth.AuthzManager, port AuthenticatePort) Authenticate {
	return Authenticate{
		log:       log,
		token:     token,
		port:      port,
		ucGetUser: NewGetUser(authz, port),
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
		uc.log.Info().Str("ident", ident).Err(err).HTTPIP(r).Msg("No user with given ident found")
		compensateCompare()
		return nil, err
	}

	if hashCred, ok := identMeta.Get(api.KeyCredential); ok {
		ok, err = cred.CompareHashAndCredential(hashCred, identMeta.Zid, ident, credential)
		if err != nil {
			uc.log.Info().Str("ident", ident).Err(err).HTTPIP(r).Msg("Error while comparing credentials")
			return nil, err
		}
		if ok {
			token, err2 := uc.token.GetToken(identMeta, d, k)
			if err2 != nil {
				uc.log.Info().Str("ident", ident).Err(err).Msg("Unable to produce authentication token")
				return nil, err2
			}
			uc.log.Info().Str("user", ident).Msg("Successful")
			return token, nil
		}
		uc.log.Info().Str("ident", ident).HTTPIP(r).Msg("Credentials don't match")
		return nil, nil
	}
	uc.log.Info().Str("ident", ident).Msg("No credential stored")
	compensateCompare()
	return nil, nil
}

// compensateCompare if normal comapare is not possible, to avoid timing hints.
func compensateCompare() {
	cred.CompareHashAndCredential(
		"$2a$10$WHcSO3G9afJ3zlOYQR1suuf83bCXED2jmzjti/MH4YH4l2mivDuze", id.Invalid, "", "")
}

// addDelay after credential checking to allow some CPU time for other tasks.
// durDelay is the normal delay, if time spend for checking is smaller than
// the minimum delay minDelay. In addition some jitter (+/- 50 ms) is added.
func addDelay(start time.Time, durDelay, minDelay time.Duration) {
	jitter := time.Duration(rand.Intn(100)-50) * time.Millisecond
	if elapsed := time.Since(start); elapsed+minDelay < durDelay {
		time.Sleep(durDelay - elapsed + jitter)
	} else {
		time.Sleep(minDelay + jitter)
	}
}

// IsAuthenticatedPort contains method for this usecase.
type IsAuthenticatedPort interface {
	GetUser(context.Context) *meta.Meta
}

// IsAuthenticated cheks if the caller is alrwady authenticated.
type IsAuthenticated struct {
	log   *logger.Logger
	port  IsAuthenticatedPort
	authz auth.AuthzManager
}

// NewIsAuthenticated creates a new use case object.
func NewIsAuthenticated(log *logger.Logger, port IsAuthenticatedPort, authz auth.AuthzManager) IsAuthenticated {
	return IsAuthenticated{
		log:   log,
		port:  port,
		authz: authz,
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
		uc.log.Sense().Str("auth", "disabled").Msg("IsAuthenticated")
		return IsAuthenticatedDisabled
	}
	if uc.port.GetUser(ctx) == nil {
		uc.log.Sense().Msg("IsAuthenticated is false")
		return IsAuthenticatedAndInvalid
	}
	uc.log.Sense().Msg("IsAuthenticated is true")
	return IsAuthenticatedAndValid
}
