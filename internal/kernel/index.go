//-----------------------------------------------------------------------------
// Copyright (c) 2026-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2026-present Detlef Stern
//-----------------------------------------------------------------------------

package kernel

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"sync"

	"zettelstore.de/z/internal/index"
)

type indexService struct {
	srvConfig
	mxService sync.RWMutex
	indexer   index.Indexer
	create    CreateIndexerFunc
}

func (is *indexService) Initialize(levelVar *slog.LevelVar, logger *slog.Logger) {
	is.logLevelVar = levelVar
	is.logger = logger
	is.descr = descriptionMap{}
	is.next = interfaceMap{}
}

func (is *indexService) GetLogger() *slog.Logger { return is.logger }
func (is *indexService) GetLevel() slog.Level    { return is.logLevelVar.Level() }
func (is *indexService) SetLevel(l slog.Level)   { is.logLevelVar.Set(l) }

func (is *indexService) Start(kern *Kernel) error {
	idx := is.create(&kern.cfg)
	is.logger.Info("Start Index")
	idx.Start()
	is.indexer = idx
	return nil
}
func (is *indexService) IsStarted() bool {
	is.mxService.RLock()
	started := is.indexer != nil
	is.mxService.RUnlock()
	return started
}
func (is *indexService) Stop(*Kernel) {
	is.logger.Info("Stop Index")
	is.mxService.RLock()
	idx := is.indexer
	is.mxService.RUnlock()
	idx.Stop()
	is.mxService.Lock()
	is.indexer = nil
	is.mxService.Unlock()

}

func (is *indexService) GetStatistics() []KeyValue {
	var st index.Stats
	is.mxService.RLock()
	is.indexer.ReadStats(&st)
	is.mxService.RUnlock()
	result := make([]KeyValue, 0, 10+len(st.WordCount))
	result = append(result, []KeyValue{
		{Key: "Ready", Value: fmt.Sprint(st.Ready)},
		{Key: "Building", Value: fmt.Sprint(st.Building)},
		{Key: "Build duration", Value: st.BuildDuration.String()},
		{Key: "Build failed", Value: strconv.Itoa(st.BuildFailed)},
		{Key: "Zettel", Value: strconv.Itoa(st.NumZettel)},
		{Key: "Words / Fragments", Value: strconv.Itoa(st.NumWords)},
		{Key: "Long Word Store", Value: strconv.Itoa(st.Vocabulary)},
		{Key: "Updated zettel", Value: strconv.FormatUint(st.UpdatedZettel, 10)},
		{Key: "Removed zettel", Value: strconv.FormatUint(st.RemovedZettel, 10)},
		{Key: "Resets", Value: strconv.FormatUint(st.NumResets, 10)},
	}...)

	sizes := slices.Collect(maps.Keys(st.WordCount))
	slices.Sort(sizes)
	smallCount, totalCount, longSize := 0, 0, 0
	for _, size := range sizes {
		count := st.WordCount[size]
		if size <= 14 {
			smallCount += count
		} else {
			longSize += (int(size) * count)
		}
		totalCount += count
		result = append(result, KeyValue{
			Key:   "Size " + strconv.Itoa(int(size)),
			Value: strconv.Itoa(count),
		})
	}
	result = append(result, KeyValue{Key: "Small words", Value: strconv.Itoa(smallCount)})
	result = append(result, KeyValue{Key: "All words", Value: strconv.Itoa(totalCount)})
	result = append(result, KeyValue{Key: "Long size", Value: strconv.Itoa(longSize)})
	return result
}
