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
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/maps"
	"t73f.de/r/zsc/strfun"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
)

type coreService struct {
	srvConfig
	started bool

	mxRecover  sync.RWMutex
	mapRecover map[string]recoverInfo
}
type recoverInfo struct {
	count uint64
	ts    time.Time
	info  interface{}
	stack []byte
}

func (cs *coreService) Initialize(logger *logger.Logger) {
	cs.logger = logger
	cs.mapRecover = make(map[string]recoverInfo)
	cs.descr = descriptionMap{
		kernel.CoreDebug:     {"Debug mode", parseBool, false},
		kernel.CoreGoArch:    {"Go processor architecture", nil, false},
		kernel.CoreGoOS:      {"Go Operating System", nil, false},
		kernel.CoreGoVersion: {"Go Version", nil, false},
		kernel.CoreHostname:  {"Host name", nil, false},
		kernel.CorePort: {
			"Port of command line server",
			cs.noFrozen(func(val string) (any, error) {
				port, err := net.LookupPort("tcp", val)
				if err != nil {
					return nil, err
				}
				return port, nil
			}),
			true,
		},
		kernel.CoreProgname: {"Program name", nil, false},
		kernel.CoreStarted:  {"Start time", nil, false},
		kernel.CoreVerbose:  {"Verbose output", parseBool, true},
		kernel.CoreVersion: {
			"Version",
			cs.noFrozen(func(val string) (any, error) {
				if val == "" {
					return kernel.CoreDefaultVersion, nil
				}
				return val, nil
			}),
			false,
		},
		kernel.CoreVTime: {"Version time", nil, false},
	}
	cs.next = interfaceMap{
		kernel.CoreDebug:     false,
		kernel.CoreGoArch:    runtime.GOARCH,
		kernel.CoreGoOS:      runtime.GOOS,
		kernel.CoreGoVersion: runtime.Version(),
		kernel.CoreHostname:  "*unknown host*",
		kernel.CorePort:      0,
		kernel.CoreStarted:   time.Now().Local().Format(id.TimestampLayout),
		kernel.CoreVerbose:   false,
	}
	if hn, err := os.Hostname(); err == nil {
		cs.next[kernel.CoreHostname] = hn
	}
}

func (cs *coreService) GetLogger() *logger.Logger { return cs.logger }

func (cs *coreService) Start(*myKernel) error {
	cs.started = true
	return nil
}
func (cs *coreService) IsStarted() bool { return cs.started }
func (cs *coreService) Stop(*myKernel) {
	cs.started = false
}

func (cs *coreService) GetStatistics() []kernel.KeyValue {
	cs.mxRecover.RLock()
	defer cs.mxRecover.RUnlock()
	names := maps.Keys(cs.mapRecover)
	result := make([]kernel.KeyValue, 0, 3*len(names))
	for _, n := range names {
		ri := cs.mapRecover[n]
		result = append(
			result,
			kernel.KeyValue{
				Key:   fmt.Sprintf("Recover %q / Count", n),
				Value: fmt.Sprintf("%d", ri.count),
			},
			kernel.KeyValue{
				Key:   fmt.Sprintf("Recover %q / Last ", n),
				Value: fmt.Sprintf("%v", ri.ts),
			},
			kernel.KeyValue{
				Key:   fmt.Sprintf("Recover %q / Info ", n),
				Value: fmt.Sprintf("%v", ri.info),
			},
		)
	}
	return result
}

func (cs *coreService) RecoverLines(name string) []string {
	cs.mxRecover.RLock()
	ri := cs.mapRecover[name]
	cs.mxRecover.RUnlock()
	if ri.stack == nil {
		return nil
	}
	return append(
		[]string{
			fmt.Sprintf("Count: %d", ri.count),
			fmt.Sprintf("Time: %v", ri.ts),
			fmt.Sprintf("Reason: %v", ri.info),
		},
		strfun.SplitLines(string(ri.stack))...,
	)
}

func (cs *coreService) updateRecoverInfo(name string, recoverInfo interface{}, stack []byte) {
	cs.mxRecover.Lock()
	ri := cs.mapRecover[name]
	ri.count++
	ri.ts = time.Now().Local()
	ri.info = recoverInfo
	ri.stack = stack
	cs.mapRecover[name] = ri
	cs.mxRecover.Unlock()
}
