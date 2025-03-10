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

package impl

import (
	"errors"
	"sync"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logger"
)

type authService struct {
	srvConfig
	mxService     sync.RWMutex
	manager       auth.Manager
	createManager kernel.CreateAuthManagerFunc
}

var errAlreadySetOwner = errors.New("changing an existing owner not allowed")
var errAlreadyROMode = errors.New("system in readonly mode cannot change this mode")

func (as *authService) Initialize(logger *logger.Logger) {
	as.logger = logger
	as.descr = descriptionMap{
		kernel.AuthOwner: {
			"Owner's zettel id",
			func(val string) (any, error) {
				if owner := as.cur[kernel.AuthOwner]; owner != nil && owner != id.Invalid {
					return nil, errAlreadySetOwner
				}
				if val == "" {
					return id.Invalid, nil
				}
				return parseZid(val)
			},
			false,
		},
		kernel.AuthReadonly: {
			"Readonly mode",
			func(val string) (any, error) {
				if ro := as.cur[kernel.AuthReadonly]; ro == true {
					return nil, errAlreadyROMode
				}
				return parseBool(val)
			},
			true,
		},
	}
	as.next = interfaceMap{
		kernel.AuthOwner:    id.Invalid,
		kernel.AuthReadonly: false,
	}
}

func (as *authService) GetLogger() *logger.Logger { return as.logger }

func (as *authService) Start(*myKernel) error {
	as.mxService.Lock()
	defer as.mxService.Unlock()
	readonlyMode := as.GetNextConfig(kernel.AuthReadonly).(bool)
	owner := as.GetNextConfig(kernel.AuthOwner).(id.Zid)
	authMgr, err := as.createManager(readonlyMode, owner)
	if err != nil {
		as.logger.Error().Err(err).Msg("Unable to create manager")
		return err
	}
	as.logger.Info().Msg("Start Manager")
	as.manager = authMgr
	return nil
}

func (as *authService) IsStarted() bool {
	as.mxService.RLock()
	defer as.mxService.RUnlock()
	return as.manager != nil
}

func (as *authService) Stop(*myKernel) {
	as.logger.Info().Msg("Stop Manager")
	as.mxService.Lock()
	as.manager = nil
	as.mxService.Unlock()
}

func (*authService) GetStatistics() []kernel.KeyValue { return nil }
