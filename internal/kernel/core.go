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
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"runtime"
	"slices"
	"sync"
	"time"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/logger"
	"zettelstore.de/z/strfun"
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
	info  any
	stack []byte
}

func (cs *coreService) Initialize(logger *slog.Logger, dlogger *logger.DLogger) {
	cs.logger = logger
	cs.dlogger = dlogger
	cs.mapRecover = make(map[string]recoverInfo)
	cs.descr = descriptionMap{
		CoreDebug:     {"Debug mode", parseBool, false},
		CoreGoArch:    {"Go processor architecture", nil, false},
		CoreGoOS:      {"Go Operating System", nil, false},
		CoreGoVersion: {"Go Version", nil, false},
		CoreHostname:  {"Host name", nil, false},
		CorePort: {
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
		CoreProgname: {"Program name", nil, false},
		CoreStarted:  {"Start time", nil, false},
		CoreVerbose:  {"Verbose output", parseBool, true},
		CoreVersion: {
			"Version",
			cs.noFrozen(func(val string) (any, error) {
				if val == "" {
					return CoreDefaultVersion, nil
				}
				return val, nil
			}),
			false,
		},
		CoreVTime: {"Version time", nil, false},
	}
	cs.next = interfaceMap{
		CoreDebug:     false,
		CoreGoArch:    runtime.GOARCH,
		CoreGoOS:      runtime.GOOS,
		CoreGoVersion: runtime.Version(),
		CoreHostname:  "*unknown host*",
		CorePort:      0,
		CoreStarted:   time.Now().Local().Format(id.TimestampLayout),
		CoreVerbose:   false,
	}
	if hn, err := os.Hostname(); err == nil {
		cs.next[CoreHostname] = hn
	}
}

func (cs *coreService) GetLogger() *slog.Logger     { return cs.logger }
func (cs *coreService) GetDLogger() *logger.DLogger { return cs.dlogger }

func (cs *coreService) Start(*Kernel) error {
	cs.started = true
	return nil
}
func (cs *coreService) IsStarted() bool { return cs.started }
func (cs *coreService) Stop(*Kernel) {
	cs.started = false
}

func (cs *coreService) GetStatistics() []KeyValue {
	cs.mxRecover.RLock()
	defer cs.mxRecover.RUnlock()
	names := slices.Sorted(maps.Keys(cs.mapRecover))
	result := make([]KeyValue, 0, 3*len(names))
	for _, n := range names {
		ri := cs.mapRecover[n]
		result = append(
			result,
			KeyValue{
				Key:   fmt.Sprintf("Recover %q / Count", n),
				Value: fmt.Sprintf("%d", ri.count),
			},
			KeyValue{
				Key:   fmt.Sprintf("Recover %q / Last ", n),
				Value: fmt.Sprintf("%v", ri.ts),
			},
			KeyValue{
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

func (cs *coreService) updateRecoverInfo(name string, recoverInfo any, stack []byte) {
	cs.mxRecover.Lock()
	ri := cs.mapRecover[name]
	ri.count++
	ri.ts = time.Now().Local()
	ri.info = recoverInfo
	ri.stack = stack
	cs.mapRecover[name] = ri
	cs.mxRecover.Unlock()
}
