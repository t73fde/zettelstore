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
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"

	"t73f.de/r/zsc/domain/id"
)

type parseFunc func(string) (any, error)
type configDescription struct {
	text    string
	parse   parseFunc
	canList bool
}
type descriptionMap map[string]configDescription
type interfaceMap map[string]any

func (m interfaceMap) Clone() interfaceMap { return maps.Clone(m) }

type srvConfig struct {
	logLevelVar *slog.LevelVar
	logger      *slog.Logger
	mxConfig    sync.RWMutex
	frozen      bool
	descr       descriptionMap
	cur         interfaceMap
	next        interfaceMap
}

func (cfg *srvConfig) ConfigDescriptions() []serviceConfigDescription {
	cfg.mxConfig.RLock()
	defer cfg.mxConfig.RUnlock()
	keys := slices.Sorted(maps.Keys(cfg.descr))
	result := make([]serviceConfigDescription, 0, len(keys))
	for _, k := range keys {
		text := cfg.descr[k].text
		if strings.HasSuffix(k, "-") {
			text = text + " (list)"
		}
		result = append(result, serviceConfigDescription{Key: k, Descr: text})
	}
	return result
}

var errAlreadyFrozen = errors.New("value frozen")

func (cfg *srvConfig) noFrozen(parse parseFunc) parseFunc {
	return func(val string) (any, error) {
		if cfg.frozen {
			return nil, errAlreadyFrozen
		}
		return parse(val)
	}
}

var errListKeyNotFound = errors.New("no list key found")

func (cfg *srvConfig) SetConfig(key, value string) error {
	cfg.mxConfig.Lock()
	defer cfg.mxConfig.Unlock()
	descr, ok := cfg.descr[key]
	if !ok {
		d, baseKey, num := cfg.getListDescription(key)
		if num < 0 {
			return errListKeyNotFound
		}
		for i := num + 1; ; i++ {
			k := baseKey + strconv.Itoa(i)
			if _, ok = cfg.next[k]; !ok {
				break
			}
			delete(cfg.next, k)
		}
		if num == 0 {
			return nil
		}
		descr = d
	}
	parse := descr.parse
	if parse == nil {
		if cfg.frozen {
			return errAlreadyFrozen
		}
		cfg.next[key] = value
		return nil
	}
	iVal, err := parse(value)
	if err != nil {
		return err
	}
	cfg.next[key] = iVal
	return nil
}

func (cfg *srvConfig) getListDescription(key string) (configDescription, string, int) {
	for k, d := range cfg.descr {
		if !strings.HasSuffix(k, "-") {
			continue
		}
		if !strings.HasPrefix(key, k) {
			continue
		}
		num, err := strconv.Atoi(key[len(k):])
		if err != nil || num < 0 {
			continue
		}
		return d, k, num
	}
	return configDescription{}, "", -1
}

func (cfg *srvConfig) GetCurConfig(key string) any {
	cfg.mxConfig.RLock()
	defer cfg.mxConfig.RUnlock()
	if cfg.cur == nil {
		return cfg.next[key]
	}
	return cfg.cur[key]
}

func (cfg *srvConfig) GetNextConfig(key string) any {
	cfg.mxConfig.RLock()
	defer cfg.mxConfig.RUnlock()
	return cfg.next[key]
}

func (cfg *srvConfig) GetCurConfigList(all bool) []KeyDescrValue {
	return cfg.getOneConfigList(all, cfg.GetCurConfig)
}
func (cfg *srvConfig) GetNextConfigList() []KeyDescrValue {
	return cfg.getOneConfigList(true, cfg.GetNextConfig)
}
func (cfg *srvConfig) getOneConfigList(all bool, getConfig func(string) any) []KeyDescrValue {
	if len(cfg.descr) == 0 {
		return nil
	}
	keys := cfg.getSortedConfigKeys(all, getConfig)
	result := make([]KeyDescrValue, 0, len(keys))
	for _, k := range keys {
		val := getConfig(k)
		if val == nil {
			continue
		}
		descr, ok := cfg.descr[k]
		if !ok {
			descr, _, _ = cfg.getListDescription(k)
		}
		result = append(result, KeyDescrValue{
			Key:   k,
			Descr: descr.text,
			Value: fmt.Sprintf("%v", val),
		})
	}
	return result
}

func (cfg *srvConfig) getSortedConfigKeys(all bool, getConfig func(string) any) []string {
	keys := make([]string, 0, len(cfg.descr))
	for k, descr := range cfg.descr {
		if all || descr.canList {
			if !strings.HasSuffix(k, "-") {
				keys = append(keys, k)
				continue
			}
			for i := 1; ; i++ {
				key := k + strconv.Itoa(i)
				val := getConfig(key)
				if val == nil {
					break
				}
				keys = append(keys, key)
			}
		}
	}
	slices.Sort(keys)
	return keys
}

func (cfg *srvConfig) Freeze() {
	cfg.mxConfig.Lock()
	cfg.frozen = true
	cfg.mxConfig.Unlock()
}

func (cfg *srvConfig) SwitchNextToCur() {
	cfg.mxConfig.Lock()
	defer cfg.mxConfig.Unlock()
	cfg.cur = cfg.next.Clone()
}

var errNoBoolean = errors.New("no boolean value")

func parseBool(val string) (any, error) {
	if val == "" {
		return false, errNoBoolean
	}
	switch val[0] {
	case '0', 'f', 'F', 'n', 'N':
		return false, nil
	}
	return true, nil
}

func parseString(val string) (any, error) { return val, nil }
func parseInt(val string) (any, error)    { return strconv.Atoi(val) }
func parseInt64(val string) (any, error)  { return strconv.ParseInt(val, 10, 64) }
func parseZid(val string) (any, error)    { return id.Parse(val) }
func parseInvalidZid(val string) (any, error) {
	zid, _ := id.Parse(val)
	return zid, nil
}
