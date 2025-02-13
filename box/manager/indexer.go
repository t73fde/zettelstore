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

package manager

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/box"
	"zettelstore.de/z/box/manager/store"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/strfun"
	"zettelstore.de/z/zettel"
)

// SearchEqual returns all zettel that contains the given exact word.
// The word must be normalized through Unicode NKFD, trimmed and not empty.
func (mgr *Manager) SearchEqual(word string) *idset.Set {
	found := mgr.idxStore.SearchEqual(word)
	mgr.idxLog.Debug().Str("word", word).Int("found", int64(found.Length())).Msg("SearchEqual")
	if msg := mgr.idxLog.Trace(); msg.Enabled() {
		msg.Str("ids", fmt.Sprint(found)).Msg("IDs")
	}
	return found
}

// SearchPrefix returns all zettel that have a word with the given prefix.
// The prefix must be normalized through Unicode NKFD, trimmed and not empty.
func (mgr *Manager) SearchPrefix(prefix string) *idset.Set {
	found := mgr.idxStore.SearchPrefix(prefix)
	mgr.idxLog.Debug().Str("prefix", prefix).Int("found", int64(found.Length())).Msg("SearchPrefix")
	if msg := mgr.idxLog.Trace(); msg.Enabled() {
		msg.Str("ids", fmt.Sprint(found)).Msg("IDs")
	}
	return found
}

// SearchSuffix returns all zettel that have a word with the given suffix.
// The suffix must be normalized through Unicode NKFD, trimmed and not empty.
func (mgr *Manager) SearchSuffix(suffix string) *idset.Set {
	found := mgr.idxStore.SearchSuffix(suffix)
	mgr.idxLog.Debug().Str("suffix", suffix).Int("found", int64(found.Length())).Msg("SearchSuffix")
	if msg := mgr.idxLog.Trace(); msg.Enabled() {
		msg.Str("ids", fmt.Sprint(found)).Msg("IDs")
	}
	return found
}

// SearchContains returns all zettel that contains the given string.
// The string must be normalized through Unicode NKFD, trimmed and not empty.
func (mgr *Manager) SearchContains(s string) *idset.Set {
	found := mgr.idxStore.SearchContains(s)
	mgr.idxLog.Debug().Str("s", s).Int("found", int64(found.Length())).Msg("SearchContains")
	if msg := mgr.idxLog.Trace(); msg.Enabled() {
		msg.Str("ids", fmt.Sprint(found)).Msg("IDs")
	}
	return found
}

// idxIndexer runs in the background and updates the index data structures.
// This is the main service of the idxIndexer.
func (mgr *Manager) idxIndexer() {
	// Something may panic. Ensure a running indexer.
	defer func() {
		if ri := recover(); ri != nil {
			kernel.Main.LogRecover("Indexer", ri)
			go mgr.idxIndexer()
		}
	}()

	timerDuration := 15 * time.Second
	timer := time.NewTimer(timerDuration)
	ctx := box.NoEnrichContext(context.Background())
	for {
		mgr.idxWorkService(ctx)
		if !mgr.idxSleepService(timer, timerDuration) {
			return
		}
	}
}

func (mgr *Manager) idxWorkService(ctx context.Context) {
	var start time.Time
	for {
		switch action, zid, lastReload := mgr.idxAr.Dequeue(); action {
		case arNothing:
			return
		case arReload:
			mgr.idxLog.Debug().Msg("reload")
			zids, err := mgr.FetchZids(ctx)
			if err == nil {
				start = time.Now()
				mgr.idxAr.Reload(zids)
				mgr.idxMx.Lock()
				mgr.idxLastReload = time.Now().Local()
				mgr.idxSinceReload = 0
				mgr.idxMx.Unlock()
			}
		case arZettel:
			mgr.idxLog.Debug().Zid(zid).Msg("zettel")
			zettel, err := mgr.GetZettel(ctx, zid)
			if err != nil {
				// Zettel was deleted or is not accessible b/c of other reasons
				mgr.idxLog.Trace().Zid(zid).Msg("delete")
				mgr.idxDeleteZettel(ctx, zid)
				continue
			}
			mgr.idxLog.Trace().Zid(zid).Msg("update")
			mgr.idxUpdateZettel(ctx, zettel)
			mgr.idxMx.Lock()
			if lastReload {
				mgr.idxDurReload = time.Since(start)
			}
			mgr.idxSinceReload++
			mgr.idxMx.Unlock()
		}
	}
}

func (mgr *Manager) idxSleepService(timer *time.Timer, timerDuration time.Duration) bool {
	select {
	case _, ok := <-mgr.idxReady:
		if !ok {
			return false
		}
	case _, ok := <-timer.C:
		if !ok {
			return false
		}
		// mgr.idxStore.Optimize() // TODO: make it less often, for example once per 10 minutes
		timer.Reset(timerDuration)
	case <-mgr.done:
		if !timer.Stop() {
			<-timer.C
		}
		return false
	}
	return true
}

func (mgr *Manager) idxUpdateZettel(ctx context.Context, zettel zettel.Zettel) {
	var cData collectData
	cData.initialize()
	if mustIndexZettel(zettel.Meta) {
		collectZettelIndexData(parser.ParseZettel(ctx, zettel, "", mgr.rtConfig), &cData)
	}

	m := zettel.Meta
	zi := store.NewZettelIndex(m)
	mgr.idxCollectFromMeta(ctx, m, zi, &cData)
	mgr.idxProcessData(ctx, zi, &cData)
	toCheck := mgr.idxStore.UpdateReferences(ctx, zi)
	mgr.idxCheckZettel(toCheck)
}

func mustIndexZettel(m *meta.Meta) bool {
	return m.Zid >= id.Zid(999999900)
}

func (mgr *Manager) idxCollectFromMeta(ctx context.Context, m *meta.Meta, zi *store.ZettelIndex, cData *collectData) {
	for key, val := range m.Computed() {
		descr := meta.GetDescription(key)
		if descr.IsProperty() {
			continue
		}
		switch descr.Type {
		case meta.TypeID:
			mgr.idxUpdateValue(ctx, descr.Inverse, string(val), zi)
		case meta.TypeIDSet:
			for val := range val.Fields() {
				mgr.idxUpdateValue(ctx, descr.Inverse, val, zi)
			}
		case meta.TypeURL:
			if _, err := url.Parse(string(val)); err == nil {
				cData.urls.Add(string(val))
			}
		default:
			if descr.Type.IsSet {
				for val := range val.Fields() {
					idxCollectMetaValue(cData.words, val)
				}
			} else {
				idxCollectMetaValue(cData.words, string(val))
			}
		}
	}
}

func idxCollectMetaValue(stWords store.WordSet, value string) {
	if words := strfun.NormalizeWords(value); len(words) > 0 {
		for _, word := range words {
			stWords.Add(word)
		}
	} else {
		stWords.Add(value)
	}
}

func (mgr *Manager) idxProcessData(ctx context.Context, zi *store.ZettelIndex, cData *collectData) {
	cData.refs.ForEach(func(ref id.Zid) {
		if mgr.hasZettel(ctx, ref) {
			zi.AddBackRef(ref)
		} else {
			zi.AddDeadRef(ref)
		}
	})
	zi.SetWords(cData.words)
	zi.SetUrls(cData.urls)
}

func (mgr *Manager) idxUpdateValue(ctx context.Context, inverseKey, value string, zi *store.ZettelIndex) {
	zid, err := id.Parse(value)
	if err != nil {
		return
	}
	if !mgr.hasZettel(ctx, zid) {
		zi.AddDeadRef(zid)
		return
	}
	if inverseKey == "" {
		zi.AddBackRef(zid)
		return
	}
	zi.AddInverseRef(inverseKey, zid)
}

func (mgr *Manager) idxDeleteZettel(ctx context.Context, zid id.Zid) {
	toCheck := mgr.idxStore.DeleteZettel(ctx, zid)
	mgr.idxCheckZettel(toCheck)
}

func (mgr *Manager) idxCheckZettel(s *idset.Set) {
	s.ForEach(func(zid id.Zid) {
		mgr.idxAr.EnqueueZettel(zid)
	})
}
