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
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strconv"
	"sync"

	"zettelstore.de/z/internal/box"
)

type boxService struct {
	srvConfig
	mxService     sync.RWMutex
	manager       box.Manager
	createManager CreateBoxManagerFunc
}

var errInvalidDirType = errors.New("invalid directory type")

func (ps *boxService) Initialize(levelVar *slog.LevelVar, logger *slog.Logger) {
	ps.logLevelVar = levelVar
	ps.logger = logger
	ps.descr = descriptionMap{
		BoxDefaultDirType: {
			"Default directory box type",
			ps.noFrozen(func(val string) (any, error) {
				switch val {
				case BoxDirTypeNotify, BoxDirTypeSimple:
					return val, nil
				}
				return nil, errInvalidDirType
			}),
			true,
		},
		BoxURIs: {
			"Box URI",
			func(val string) (any, error) {
				uVal, err := url.Parse(val)
				if err != nil {
					return nil, err
				}
				if uVal.Scheme == "" {
					uVal.Scheme = "dir"
				}
				return uVal, nil
			},
			true,
		},
	}
	ps.next = interfaceMap{
		BoxDefaultDirType: BoxDirTypeNotify,
	}
}

func (ps *boxService) GetLogger() *slog.Logger { return ps.logger }
func (ps *boxService) GetLevel() slog.Level    { return ps.logLevelVar.Level() }
func (ps *boxService) SetLevel(l slog.Level)   { ps.logLevelVar.Set(l) }

func (ps *boxService) Start(kern *Kernel) error {
	boxURIs := make([]*url.URL, 0, 4)
	for i := 1; ; i++ {
		u := ps.GetNextConfig(BoxURIs + strconv.Itoa(i))
		if u == nil {
			break
		}
		boxURIs = append(boxURIs, u.(*url.URL))
	}
	ps.mxService.Lock()
	defer ps.mxService.Unlock()
	mgr, err := ps.createManager(boxURIs, kern.auth.manager, &kern.cfg)
	if err != nil {
		ps.logger.Error("Unable to create manager", "err", err)
		return err
	}
	ps.logger.Info("Start Manager", "location", mgr.Location())
	if err = mgr.Start(context.Background()); err != nil {
		ps.logger.Error("Unable to start manager", "err", err)
		return err
	}
	kern.cfg.setBox(mgr)
	ps.manager = mgr
	return nil
}

func (ps *boxService) IsStarted() bool {
	ps.mxService.RLock()
	defer ps.mxService.RUnlock()
	return ps.manager != nil
}

func (ps *boxService) Stop(*Kernel) {
	ps.logger.Info("Stop Manager")
	ps.mxService.RLock()
	mgr := ps.manager
	ps.mxService.RUnlock()
	mgr.Stop(context.Background())
	ps.mxService.Lock()
	ps.manager = nil
	ps.mxService.Unlock()
}

func (ps *boxService) GetStatistics() []KeyValue {
	var st box.Stats
	ps.mxService.RLock()
	ps.manager.ReadStats(&st)
	ps.mxService.RUnlock()
	return []KeyValue{
		{Key: "Read-only", Value: strconv.FormatBool(st.ReadOnly)},
		{Key: "Managed boxes", Value: strconv.Itoa(st.NumManagedBoxes)},
		{Key: "Zettel (total)", Value: strconv.Itoa(st.ZettelTotal)},
		{Key: "Zettel (indexed)", Value: strconv.Itoa(st.ZettelIndexed)},
		{Key: "Last re-index", Value: st.LastReload.Format("2006-01-02 15:04:05 -0700 MST")},
		{Key: "Duration last re-index", Value: fmt.Sprintf("%vms", st.DurLastReload.Milliseconds())},
		{Key: "Indexes since last re-index", Value: strconv.FormatUint(st.IndexesSinceReload, 10)},
		{Key: "Indexed words", Value: strconv.FormatUint(st.IndexedWords, 10)},
		{Key: "Indexed URLs", Value: strconv.FormatUint(st.IndexedUrls, 10)},
		{Key: "Zettel enrichments", Value: strconv.FormatUint(st.IndexUpdates, 10)},
	}
}

func (ps *boxService) dumpIndex(w io.Writer) {
	ps.manager.Dump(w)
}

func (ps *boxService) Refresh() error {
	ps.mxService.RLock()
	defer ps.mxService.RUnlock()
	if ps.manager != nil {
		return ps.manager.Refresh(context.Background())
	}
	return nil
}
