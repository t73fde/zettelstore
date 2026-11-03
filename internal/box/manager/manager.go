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
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"t73f.de/r/zero/iter"
	"t73f.de/r/zero/roster"
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/index"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logging"
)

// ConnectData contains all administration related values.
type ConnectData struct {
	Config   config.Config
	Enricher box.Enricher
	Notify   box.UpdateNotifier
}

// Constants for query parameter
const (
	QueryName     = "name"
	QueryReadOnly = "readonly"
)

// Connect returns a handle to the specified box.
func Connect(u *url.URL, cdata *ConnectData) (box.ManagedBox, error) {
	if create, ok := registry[u.Scheme]; ok {
		return create(u, cdata)
	}
	return nil, fmt.Errorf("invalid scheme: %q", u.Scheme)
}

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
	mgrLogger        *slog.Logger
	stateMx          sync.RWMutex
	state            box.StartState
	mgrMx            sync.RWMutex
	rtConfig         config.Config
	boxes            []box.ManagedBox
	observers        []box.UpdateFunc
	mxObserver       sync.RWMutex
	done             chan struct{}
	infos            chan box.UpdateInfo
	propertyKeys     roster.Roster[string] // Set of property key names
	index            index.Index
	indexWaitTimeout time.Duration
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
func New(boxURIs []*url.URL, authManager auth.BaseManager, index index.Index, rtConfig config.Config) (*Manager, error) {
	var propertyKeys roster.Roster[string]
	for kd := range meta.KeyDescriptionSeq() {
		if kd.IsProperty() {
			propertyKeys.Insert(kd.Name)
		}
	}
	boxLogger := kernel.Main.GetLogger(kernel.BoxService)
	mgr := &Manager{
		mgrLogger:        boxLogger.With("box", "manager"),
		rtConfig:         rtConfig,
		infos:            make(chan box.UpdateInfo, len(boxURIs)*10),
		propertyKeys:     propertyKeys,
		index:            index,
		indexWaitTimeout: 4 * time.Second, // TODO: make configurable
	}

	if err := setupBoxURIs(boxURIs, authManager.IsReadonly()); err != nil {
		return nil, err
	}
	cdata := ConnectData{Config: rtConfig, Enricher: mgr, Notify: mgr.notifyChanged}
	boxes := make([]box.ManagedBox, 0, len(boxURIs)+2)
	for _, u := range boxURIs {
		b, err := Connect(u, &cdata)
		if err != nil {
			return nil, err
		}
		if b != nil {
			boxes = append(boxes, b)
		}
	}
	constbox, err := registry[box.SchemeConstBox](nil, &cdata)
	if err != nil {
		return nil, err
	}
	compbox, err := registry[box.SchemeCompBox](nil, &cdata)
	if err != nil {
		return nil, err
	}
	boxes = append(boxes, constbox, compbox)

	mgr.index.RegisterRetriever(mgr)
	mgr.boxes = boxes
	return mgr, nil
}
func setupBoxURIs(boxURIs []*url.URL, isReadonly bool) error {
	boxNames := roster.New(box.SchemeCompBox, box.SchemeConstBox)
	hasName := make([]bool, len(boxURIs))
	for i, u := range boxURIs {
		q := u.Query()
		if name := q.Get(QueryName); name != "" {
			if s := zerostrings.JoinSeq(
				iter.MapSeq(zerostrings.NormalizeWordSeq(name),
					func(b []byte) string { return string(b) }), ""); s != "" {
				if boxNames.Contains(s) {
					if name == s {
						return fmt.Errorf("name %q in box-uri-%d %v already used", s, i+1, u)
					}
					return fmt.Errorf("name %q (%q) in box-uri-%d %v already used", name, s, i+1, u)
				}
				boxNames.Insert(s)
				hasName[i] = true
			} else {
				q.Del(QueryName)
				u.RawQuery = q.Encode()
			}
		}
		if isReadonly {
			q.Set(QueryReadOnly, "")
			u.RawQuery = q.Encode()
		}
	}

	newName := make([]string, len(boxURIs))
	addedName := func(name string, pos int) bool {
		if !boxNames.Contains(name) {
			boxNames.Insert(name)
			newName[pos] = name
			return true
		}
		return false
	}
	for i, u := range boxURIs {
		if hasName[i] {
			continue
		}
		if name := nameFromPath(u.Path); name != "" && addedName(name, i) {
			continue
		}
		if name := nameFromPath(u.Opaque); name != "" && addedName(name, i) {
			continue
		}
		if scheme := u.Scheme; addedName(scheme, i) {
			continue
		}
		baseName := strconv.Itoa(i + 1)
		for cnt := 0; ; cnt++ {
			name := baseName
			if cnt > 0 {
				name = baseName + "b" + strconv.Itoa(cnt)
			}
			if addedName(name, i) {
				break
			}
		}
	}
	for i, u := range boxURIs {
		if hasName[i] || newName[i] == "" {
			continue
		}
		q := u.Query()
		q.Set(QueryName, newName[i])
		u.RawQuery = q.Encode()
	}
	return nil
}
func nameFromPath(path string) string {
	name := filepath.Base(path)
	if name[0] == '.' || name[0] == '/' {
		return ""
	}
	if ext := filepath.Ext(name); ext != "" {
		name = name[0 : len(name)-len(ext)]
	}
	return zerostrings.JoinSeq(
		iter.MapSeq(zerostrings.NormalizeWordSeq(name),
			func(b []byte) string { return string(b) }), "")
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
					logging.LogTrace(mgr.mgrLogger, "clean destutter cache")
					cache = destutterCache{}
				}
				tsLastEvent = now

				reason, zid := ci.Reason, ci.Zid
				mgr.mgrLogger.Debug("notifier", "reason", reason, "zid", zid)
				if ignoreUpdate(cache, now, reason, zid) {
					logging.LogTrace(mgr.mgrLogger, "notifier ignored", "reason", reason, "zid", zid)
					continue
				}

				if ci.Box == nil {
					ci.Box = mgr
				}
				if mgr.State() == box.StartStateStarted {
					switch reason {
					case box.OnReady:
						mgr.index.Enqueue(index.OnReady, zid, nil)
					case box.OnReload:
						mgr.index.Enqueue(index.OnReset, zid, nil)
					case box.OnZettel, box.OnDelete:
						mgr.index.Enqueue(index.OnZettel, zid, nil)
					default:
						mgr.mgrLogger.Error("unknown notification reason", "reason", reason)
					}
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

func (mgr *Manager) notifyObserver(ci *box.UpdateInfo) {
	mgr.mxObserver.RLock()
	observers := mgr.observers
	mgr.mxObserver.RUnlock()
	for _, ob := range observers {
		ob(*ci)
	}
}

func (mgr *Manager) enqueueAndWait(ctx context.Context, zid id.Zid) {
	if !mgr.index.IsReady() {
		mgr.index.Enqueue(index.OnZettel, zid, nil) // do not wait for a not ready index
		return
	}
	mgr.mgrLogger.Debug("enqueueAndWait/0", "zid", zid)
	done := make(chan error, 1) // buffered, never closed
	mgr.index.Enqueue(index.OnZettel, zid, done)
	mgr.mgrLogger.Debug("enqueueAndWait/1", "zid", zid)

	timer := time.NewTimer(mgr.indexWaitTimeout)
	defer timer.Stop()
	select {
	case err := <-done:
		if err == nil {
			mgr.mgrLogger.Debug("Index updated", "zid", zid)
		} else {
			mgr.mgrLogger.Info("Index failed", "zid", zid, "error", err)
		}
	case <-timer.C:
		mgr.mgrLogger.Info("Index wait timeout", "zid", zid)
	case <-ctx.Done():
		mgr.mgrLogger.Debug("Index wait cancelled", "zid", zid, "cause", context.Cause(ctx))
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
	mgr.done = make(chan struct{})
	go mgr.notifier()

	mgr.waitBoxesAreStarted()
	mgr.setState(box.StartStateStarted)

	mgr.notifyObserver(&box.UpdateInfo{Box: mgr, Reason: box.OnReady})
	mgr.index.Enqueue(index.OnReady, id.Invalid, nil) // Indexer can setup itself
	mgr.index.Enqueue(index.OnReset, id.Invalid, nil) // Ensure an initial index run
	return nil
}

func (mgr *Manager) waitBoxesAreStarted() {
	const waitTime = 10 * time.Millisecond
	const waitLoop = int(1 * time.Second / waitTime)
	for i := 1; !mgr.allBoxesStarted(); i++ {
		if i%waitLoop == 0 {
			if time.Duration(i)*waitTime > time.Minute {
				mgr.mgrLogger.Info("Waiting for more than one minute to start")
			} else {
				logging.LogTrace(mgr.mgrLogger, "Wait for boxes to start")
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

// IsReady returns true if the Manager is not just started, but also all
// zettel are indexed.
func (mgr *Manager) IsReady() bool {
	return mgr.State() == box.StartStateStarted && mgr.index.IsReady()
}

// Refresh internal box data.
func (mgr *Manager) Refresh(ctx context.Context) error {
	mgr.mgrLogger.Debug("Refresh")
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
	mgr.mgrLogger.Debug("ReIndex")
	if err := mgr.checkContinue(ctx); err != nil {
		return err
	}
	mgr.infos <- box.UpdateInfo{Box: mgr, Reason: box.OnZettel, Zid: zid}
	return nil
}

// ReadStats populates st with box statistics.
func (mgr *Manager) ReadStats(st *box.Stats) {
	mgr.mgrLogger.Debug("ReadStats")
	mgr.mgrMx.RLock()
	defer mgr.mgrMx.RUnlock()
	subStats := make([]box.ManagedBoxStats, len(mgr.boxes))
	for i, p := range mgr.boxes {
		p.ReadStats(&subStats[i])
	}
	st.NumManagedBoxes = len(mgr.boxes)

	st.ReadOnly = true
	if len(subStats) > 0 {
		st.ReadOnly = subStats[0].ReadOnly
	}

	sumZettel := 0
	for _, sst := range subStats {
		sumZettel += sst.Zettel
	}
	st.ZettelTotal = sumZettel
}

// Dump internal data structures to a Writer.
func (mgr *Manager) Dump(w io.Writer) { mgr.index.Dump(w) }

func (mgr *Manager) checkContinue(ctx context.Context) error {
	if mgr.State() != box.StartStateStarted {
		return box.ErrStopped
	}
	return ctx.Err()
}

func (mgr *Manager) notifyChanged(bbox box.BaseBox, zid id.Zid, reason box.UpdateReason) {
	if infos := mgr.infos; infos != nil {
		mgr.infos <- box.UpdateInfo{Box: bbox, Reason: reason, Zid: zid}
	}
}
