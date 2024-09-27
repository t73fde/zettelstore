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

// Package manager coordinates the various boxes and indexes of a Zettelstore.
package manager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"zettelstore.de/z/auth"
	"zettelstore.de/z/box"
	"zettelstore.de/z/box/manager/mapstore"
	"zettelstore.de/z/box/manager/store"
	"zettelstore.de/z/config"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/strfun"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// ConnectData contains all administration related values.
type ConnectData struct {
	Number   int // number of the box, starting with 1.
	Config   config.Config
	Enricher box.Enricher
	Notify   box.UpdateNotifier
	Mapper   Mapper
}

// Mapper allows to inspect the mapping between old-style and new-style zettel identifier.
type Mapper interface {
	Warnings(context.Context) (*id.Set, error) // Fetch problematic zettel identifier

	FetchAsBytes(context.Context) ([]byte, error)
}

// Connect returns a handle to the specified box.
func Connect(u *url.URL, authManager auth.BaseManager, cdata *ConnectData) (box.ManagedBox, error) {
	if authManager.IsReadonly() {
		rawURL := u.String()
		// TODO: the following is wrong under some circumstances:
		// 1. fragment is set
		if q := u.Query(); len(q) == 0 {
			rawURL += "?readonly"
		} else if _, ok := q["readonly"]; !ok {
			rawURL += "&readonly"
		}
		var err error
		if u, err = url.Parse(rawURL); err != nil {
			return nil, err
		}
	}

	if create, ok := registry[u.Scheme]; ok {
		return create(u, cdata)
	}
	return nil, &ErrInvalidScheme{u.Scheme}
}

// ErrInvalidScheme is returned if there is no box with the given scheme.
type ErrInvalidScheme struct{ Scheme string }

func (err *ErrInvalidScheme) Error() string { return "Invalid scheme: " + err.Scheme }

type createFunc func(*url.URL, *ConnectData) (box.ManagedBox, error)

var registry = map[string]createFunc{}

// Register the encoder for later retrieval.
func Register(scheme string, create createFunc) {
	if _, ok := registry[scheme]; ok {
		panic(scheme)
	}
	registry[scheme] = create
}

// Manager is a coordinating box.
type Manager struct {
	mgrLog       *logger.Logger
	stateMx      sync.RWMutex
	state        box.StartState
	mgrMx        sync.RWMutex
	rtConfig     config.Config
	boxes        []box.ManagedBox
	observers    []box.UpdateFunc
	mxObserver   sync.RWMutex
	done         chan struct{}
	infos        chan box.UpdateInfo
	propertyKeys strfun.Set // Set of property key names
	zidMapper    *zidMapper
	mappingMx    sync.Mutex // protects updates to mapping zettel

	// Indexer data
	idxLog   *logger.Logger
	idxStore store.Store
	idxAr    *anteroomQueue
	idxReady chan struct{} // Signal a non-empty anteroom to background task

	// Indexer stats data
	idxMx          sync.RWMutex
	idxLastReload  time.Time
	idxDurReload   time.Duration
	idxSinceReload uint64
}

func (mgr *Manager) setState(newState box.StartState) {
	mgr.stateMx.Lock()
	mgr.state = newState
	mgr.stateMx.Unlock()
}

// State returns the box.StartState of the manager.
func (mgr *Manager) State() box.StartState {
	mgr.stateMx.RLock()
	state := mgr.state
	mgr.stateMx.RUnlock()
	return state
}

// New creates a new managing box.
func New(boxURIs []*url.URL, authManager auth.BaseManager, rtConfig config.Config) (*Manager, error) {
	descrs := meta.GetSortedKeyDescriptions()
	propertyKeys := make(strfun.Set, len(descrs))
	for _, kd := range descrs {
		if kd.IsProperty() {
			propertyKeys.Set(kd.Name)
		}
	}
	boxLog := kernel.Main.GetLogger(kernel.BoxService)
	mgr := &Manager{
		mgrLog:       boxLog.Clone().Str("box", "manager").Child(),
		rtConfig:     rtConfig,
		infos:        make(chan box.UpdateInfo, len(boxURIs)*10),
		propertyKeys: propertyKeys,

		idxLog:   boxLog.Clone().Str("box", "index").Child(),
		idxStore: createIdxStore(rtConfig),
		idxAr:    newAnteroomQueue(1000),
		idxReady: make(chan struct{}, 1),
	}
	mgr.zidMapper = newZidMapper(mgr)

	cdata := ConnectData{Number: 1, Config: rtConfig, Enricher: mgr, Notify: mgr.notifyChanged, Mapper: mgr.zidMapper}
	boxes := make([]box.ManagedBox, 0, len(boxURIs)+2)
	for _, uri := range boxURIs {
		p, err := Connect(uri, authManager, &cdata)
		if err != nil {
			return nil, err
		}
		if p != nil {
			boxes = append(boxes, p)
			cdata.Number++
		}
	}
	constbox, err := registry[" const"](nil, &cdata)
	if err != nil {
		return nil, err
	}
	cdata.Number++
	compbox, err := registry[" comp"](nil, &cdata)
	if err != nil {
		return nil, err
	}
	cdata.Number++
	boxes = append(boxes, constbox, compbox)
	mgr.boxes = boxes
	return mgr, nil
}

func createIdxStore(_ config.Config) store.Store {
	return mapstore.New()
}

// RegisterObserver registers an observer that will be notified
// if a zettel was found to be changed.
func (mgr *Manager) RegisterObserver(f box.UpdateFunc) {
	if f != nil {
		mgr.mxObserver.Lock()
		mgr.observers = append(mgr.observers, f)
		mgr.mxObserver.Unlock()
	}
}

func (mgr *Manager) notifier() {
	// The call to notify may panic. Ensure a running notifier.
	defer func() {
		if ri := recover(); ri != nil {
			kernel.Main.LogRecover("Notifier", ri)
			go mgr.notifier()
		}
	}()

	tsLastEvent := time.Now()
	cache := destutterCache{}
	for {
		select {
		case ci, ok := <-mgr.infos:
			if ok {
				now := time.Now()
				if len(cache) > 1 && tsLastEvent.Add(10*time.Second).Before(now) {
					// Cache contains entries and is definitely outdated
					mgr.mgrLog.Trace().Msg("clean destutter cache")
					cache = destutterCache{}
				}
				tsLastEvent = now

				reason, zid := ci.Reason, ci.Zid
				mgr.mgrLog.Debug().Uint("reason", uint64(reason)).Zid(zid).Msg("notifier")
				if ignoreUpdate(cache, now, reason, zid) {
					mgr.mgrLog.Trace().Uint("reason", uint64(reason)).Zid(zid).Msg("notifier ignored")
					continue
				}

				isStarted := mgr.State() == box.StartStateStarted
				mgr.idxEnqueue(reason, zid, isStarted)
				if ci.Box == nil {
					ci.Box = mgr
				}
				if isStarted {
					mgr.notifyObserver(&ci)
				}
			}
		case <-mgr.done:
			return
		}
	}
}

type destutterData struct {
	deadAt time.Time
	reason box.UpdateReason
}
type destutterCache = map[id.Zid]destutterData

func ignoreUpdate(cache destutterCache, now time.Time, reason box.UpdateReason, zid id.Zid) bool {
	if dsd, found := cache[zid]; found {
		if dsd.reason == reason && dsd.deadAt.After(now) {
			return true
		}
	}
	cache[zid] = destutterData{
		deadAt: now.Add(500 * time.Millisecond),
		reason: reason,
	}
	return false
}

func (mgr *Manager) idxEnqueue(reason box.UpdateReason, zidO id.Zid, isStarted bool) {
	switch reason {
	case box.OnReady:
		return
	case box.OnReload:
		mgr.idxAr.Reset()
	case box.OnZettel:
		if isStarted {
			if zidO > id.MappingZid {
				if _, found := mgr.zidMapper.LookupZidN(zidO); !found {
					mgr.createMapping(context.Background(), zidO)
				}
			} else if zidO == id.MappingZid {
				if _, err := mgr.getAndUpdateMapping(context.Background()); err != nil {
					mgr.mgrLog.Error().Err(err).Msg("ID mapping update problem")
				} else {
					mgr.mgrLog.Info().Msg("ID mapping updated")
				}
			}
		}
		mgr.idxAr.EnqueueZettel(zidO)
	case box.OnDelete:
		if isStarted && zidO > id.MappingZid {
			mgr.deleteMapping(context.Background(), zidO)
		}
		mgr.idxAr.EnqueueZettel(zidO)
	default:
		mgr.mgrLog.Error().Uint("reason", uint64(reason)).Zid(zidO).Msg("Unknown notification reason")
		return
	}
	select {
	case mgr.idxReady <- struct{}{}:
	default:
	}
}

func (mgr *Manager) notifyObserver(ci *box.UpdateInfo) {
	mgr.mxObserver.RLock()
	observers := mgr.observers
	mgr.mxObserver.RUnlock()
	for _, ob := range observers {
		ob(*ci)
	}
}

// Start the box. Now all other functions of the box are allowed.
// Starting an already started box is not allowed.
func (mgr *Manager) Start(ctx context.Context) error {
	mgr.mgrMx.Lock()
	defer mgr.mgrMx.Unlock()
	if mgr.State() != box.StartStateStopped {
		return box.ErrStarted
	}
	mgr.setState(box.StartStateStarting)
	for i := len(mgr.boxes) - 1; i >= 0; i-- {
		ssi, ok := mgr.boxes[i].(box.StartStopper)
		if !ok {
			continue
		}
		err := ssi.Start(ctx)
		if err == nil {
			continue
		}
		mgr.setState(box.StartStateStopping)
		for j := i + 1; j < len(mgr.boxes); j++ {
			if ssj, ok2 := mgr.boxes[j].(box.StartStopper); ok2 {
				ssj.Stop(ctx)
			}
		}
		mgr.setState(box.StartStateStopped)
		return err
	}
	mgr.idxAr.Reset() // Ensure an initial index run
	mgr.done = make(chan struct{})
	go mgr.notifier()

	mgr.waitBoxesAreStarted()
	mgr.setupIdentifierMapping()
	mgr.setState(box.StartStateStarted)

	mgr.notifyObserver(&box.UpdateInfo{Box: mgr, Reason: box.OnReady})

	go mgr.idxIndexer()
	return nil
}

func (mgr *Manager) waitBoxesAreStarted() {
	const waitTime = 10 * time.Millisecond
	const waitLoop = int(1 * time.Second / waitTime)
	for i := 1; !mgr.allBoxesStarted(); i++ {
		if i%waitLoop == 0 {
			if time.Duration(i)*waitTime > time.Minute {
				mgr.mgrLog.Info().Msg("Waiting for more than one minute to start")
			} else {
				mgr.mgrLog.Trace().Msg("Wait for boxes to start")
			}
		}
		time.Sleep(waitTime)
	}
}

func (mgr *Manager) allBoxesStarted() bool {
	for _, bx := range mgr.boxes {
		if b, ok := bx.(box.StartStopper); ok && b.State() != box.StartStateStarted {
			return false
		}
	}
	return true
}

func (mgr *Manager) setupIdentifierMapping() {
	ctx := context.Background()
	z, err := mgr.getAndUpdateMapping(ctx)
	if err != nil {
		mgr.mgrLog.Error().Err(err).Msg("error while reading and updating id mapping")
	}

	mapping, err := mgr.zidMapper.FetchAsBytes(ctx)
	if err != nil {
		mgr.mgrLog.Error().Err(err).Msg("Unable to get current identifier mapping")
		return
	}

	content := z.Content.AsBytes()
	if !bytes.Equal(content, mapping) {
		z.Content = zettel.NewContent(mapping)
		if err = mgr.updateZettel(ctx, z); err != nil {
			mgr.mgrLog.Error().Err(err).Msg("Unable to write identifier mapping zettel")
		} else {
			mgr.mgrLog.Info().Msg("Mapping was updated")
		}
	} else {
		mgr.mgrLog.Info().Msg("No mapping update")
	}
}

func (mgr *Manager) getAndUpdateMapping(ctx context.Context) (zettel.Zettel, error) {
	z, err := mgr.getZettel(ctx, id.MappingZid)
	if err != nil {
		return z, fmt.Errorf("get id mapping zettel: %w", err)
	}
	if z.Content.IsBinary() {
		return z, fmt.Errorf("id mapping zettel is binary")
	}
	z.Content.TrimSpace()
	content := z.Content.AsBytes()
	if err = mgr.zidMapper.parseAndUpdate(content); err != nil {
		err = fmt.Errorf("id mapping zettel parsing: %w", err)
	}
	return z, err
}

// Stop the started box. Now only the Start() function is allowed.
func (mgr *Manager) Stop(ctx context.Context) {
	mgr.mgrMx.Lock()
	defer mgr.mgrMx.Unlock()
	if err := mgr.checkContinue(ctx); err != nil {
		return
	}
	mgr.setState(box.StartStateStopping)
	close(mgr.done)
	for _, p := range mgr.boxes {
		if ss, ok := p.(box.StartStopper); ok {
			ss.Stop(ctx)
		}
	}
	mgr.setState(box.StartStateStopped)
}

// Refresh internal box data.
func (mgr *Manager) Refresh(ctx context.Context) error {
	mgr.mgrLog.Debug().Msg("Refresh")
	if err := mgr.checkContinue(ctx); err != nil {
		return err
	}
	mgr.infos <- box.UpdateInfo{Reason: box.OnReload, Zid: id.Invalid}
	mgr.mgrMx.Lock()
	defer mgr.mgrMx.Unlock()
	for _, bx := range mgr.boxes {
		if rb, ok := bx.(box.Refresher); ok {
			rb.Refresh(ctx)
		}
	}
	return nil
}

// ReIndex data of the given zettel.
func (mgr *Manager) ReIndex(ctx context.Context, zid id.Zid) error {
	mgr.mgrLog.Debug().Msg("ReIndex")
	if err := mgr.checkContinue(ctx); err != nil {
		return err
	}
	mgr.infos <- box.UpdateInfo{Box: mgr, Reason: box.OnZettel, Zid: zid}
	return nil
}

// ReadStats populates st with box statistics.
func (mgr *Manager) ReadStats(st *box.Stats) {
	mgr.mgrLog.Debug().Msg("ReadStats")
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	subStats := make([]box.ManagedBoxStats, len(mgr.boxes))
	for i, p := range mgr.boxes {
		p.ReadStats(&subStats[i])
	}

	st.ReadOnly = true
	sumZettel := 0
	for _, sst := range subStats {
		if !sst.ReadOnly {
			st.ReadOnly = false
		}
		sumZettel += sst.Zettel
	}
	st.NumManagedBoxes = len(mgr.boxes)
	st.ZettelTotal = sumZettel

	var storeSt store.Stats
	mgr.idxMx.RLock()
	defer mgr.idxMx.RUnlock()
	mgr.idxStore.ReadStats(&storeSt)

	st.LastReload = mgr.idxLastReload
	st.IndexesSinceReload = mgr.idxSinceReload
	st.DurLastReload = mgr.idxDurReload
	st.ZettelIndexed = storeSt.Zettel
	st.IndexUpdates = storeSt.Updates
	st.IndexedWords = storeSt.Words
	st.IndexedUrls = storeSt.Urls
}

// Dump internal data structures to a Writer.
func (mgr *Manager) Dump(w io.Writer) {
	mgr.idxStore.Dump(w)
}

func (mgr *Manager) checkContinue(ctx context.Context) error {
	if mgr.State() != box.StartStateStarted {
		return box.ErrStopped
	}
	return ctx.Err()
}

func (mgr *Manager) notifyChanged(bbox box.BaseBox, zid id.Zid, reason box.UpdateReason, force bool) {
	if infos := mgr.infos; infos != nil && (zid != id.MappingZid || force) {
		mgr.infos <- box.UpdateInfo{Box: bbox, Reason: reason, Zid: zid}
	}
}
