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

package kernel

import (
	"errors"
	"log/slog"
	"sync"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/auth"
)

type authService struct {
	srvConfig
	mxService     sync.RWMutex
	manager       auth.Manager
	createManager CreateAuthManagerFunc
}

var errAlreadySetOwner = errors.New("changing an existing owner not allowed")
var errAlreadyROMode = errors.New("system in readonly mode cannot change this mode")

func (as *authService) Initialize(levelVar *slog.LevelVar, logger *slog.Logger) {
	as.logLevelVar = levelVar
	as.logger = logger
	as.descr = descriptionMap{
		AuthOwner: {
			"Owner's zettel id",
			func(val string) (any, error) {
				if owner := as.cur[AuthOwner]; owner != nil && owner != id.Invalid {
					return nil, errAlreadySetOwner
				}
				if val == "" {
					return id.Invalid, nil
				}
				return parseZid(val)
			},
			false,
		},
		AuthReadonly: {
			"Readonly mode",
			func(val string) (any, error) {
				if ro := as.cur[AuthReadonly]; ro == true {
					return nil, errAlreadyROMode
				}
				return parseBool(val)
			},
			true,
		},
	}
	as.next = interfaceMap{
		AuthOwner:    id.Invalid,
		AuthReadonly: false,
	}
}

func (as *authService) GetLogger() *slog.Logger { return as.logger }
func (as *authService) GetLevel() slog.Level    { return as.logLevelVar.Level() }
func (as *authService) SetLevel(l slog.Level)   { as.logLevelVar.Set(l) }

func (as *authService) Start(*Kernel) error {
	as.mxService.Lock()
	defer as.mxService.Unlock()
	readonlyMode := as.GetNextConfig(AuthReadonly).(bool)
	owner := as.GetNextConfig(AuthOwner).(id.Zid)
	authMgr, err := as.createManager(readonlyMode, owner)
	if err != nil {
		as.logger.Error("Unable to create manager", "err", err)
		return err
	}
	as.logger.Info("Start Manager")
	as.manager = authMgr
	return nil
}

func (as *authService) IsStarted() bool {
	as.mxService.RLock()
	defer as.mxService.RUnlock()
	return as.manager != nil
}

func (as *authService) Stop(*Kernel) {
	as.logger.Info("Stop Manager")
	as.mxService.Lock()
	as.manager = nil
	as.mxService.Unlock()
}

func (*authService) GetStatistics() []KeyValue { return nil }
