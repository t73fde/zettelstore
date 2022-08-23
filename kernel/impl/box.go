//-----------------------------------------------------------------------------
// Copyright (c) 2021-2022 Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package impl

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"

	"zettelstore.de/z/box"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
)

type boxService struct {
	srvConfig
	mxService     sync.RWMutex
	manager       box.Manager
	createManager kernel.CreateBoxManagerFunc
}

func (ps *boxService) Initialize(logger *logger.Logger) {
	ps.logger = logger
	ps.descr = descriptionMap{
		kernel.BoxDefaultDirType: {
			"Default directory box type",
			ps.noFrozen(func(val string) interface{} {
				switch val {
				case kernel.BoxDirTypeNotify, kernel.BoxDirTypeSimple:
					return val
				}
				return nil
			}),
			true,
		},
		kernel.BoxURIs: {
			"Box URI",
			func(val string) interface{} {
				uVal, err := url.Parse(val)
				if err != nil {
					return nil
				}
				if uVal.Scheme == "" {
					uVal.Scheme = "dir"
				}
				return uVal
			},
			true,
		},
	}
	ps.next = interfaceMap{
		kernel.BoxDefaultDirType: kernel.BoxDirTypeNotify,
	}
}

func (ps *boxService) GetLogger() *logger.Logger { return ps.logger }

func (ps *boxService) Start(kern *myKernel) error {
	boxURIs := make([]*url.URL, 0, 4)
	for i := 1; ; i++ {
		u := ps.GetNextConfig(kernel.BoxURIs + strconv.Itoa(i))
		if u == nil {
			break
		}
		boxURIs = append(boxURIs, u.(*url.URL))
	}
	ps.mxService.Lock()
	defer ps.mxService.Unlock()
	mgr, err := ps.createManager(boxURIs, kern.auth.manager, &kern.cfg)
	if err != nil {
		ps.logger.Fatal().Err(err).Msg("Unable to create manager")
		return err
	}
	ps.logger.Info().Str("location", mgr.Location()).Msg("Start Manager")
	if err = mgr.Start(context.Background()); err != nil {
		ps.logger.Fatal().Err(err).Msg("Unable to start manager")
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

func (ps *boxService) Stop(*myKernel) {
	ps.logger.Info().Msg("Stop Manager")
	ps.mxService.RLock()
	mgr := ps.manager
	ps.mxService.RUnlock()
	mgr.Stop(context.Background())
	ps.mxService.Lock()
	ps.manager = nil
	ps.mxService.Unlock()
}

func (ps *boxService) GetStatistics() []kernel.KeyValue {
	var st box.Stats
	ps.mxService.RLock()
	ps.manager.ReadStats(&st)
	ps.mxService.RUnlock()
	return []kernel.KeyValue{
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

func (ps *boxService) DumpIndex(w io.Writer) {
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
