//-----------------------------------------------------------------------------
// Copyright (c) 2024-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

// Package mapper provides a mechanism to map zettel identifier with 14 digits
// to their successing identifier with four characters.
package mapper

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"t73f.de/r/zsc/input"
	"zettelstore.de/z/zettel/id"
)

// Mapper transforms old-style zettel identifier (14 digits) into new one (4 alphanums).
//
// Since there are no new-style identifier defined, there is only support for old-style
// identifier by checking, whether they are suported as new-style or not.
//
// This will change in later versions.
type Mapper struct {
	fetcher   Fetcher
	defined   map[id.Zid]id.ZidN // predefined mapping, constant after creation
	mx        sync.RWMutex       // protect toNew ... nextZidN
	toNew     map[id.Zid]id.ZidN // working mapping old->new
	toOld     map[id.ZidN]id.Zid // working mapping new->old
	nextZidM  id.ZidN            // next zid for manual
	hadManual bool
	nextZidN  id.ZidN // next zid for normal zettel
}

// Fetcher is an object that will fetch all identifier currently in user.
type Fetcher interface {
	// FetchZidsO fetch all old-style zettel identifier.
	FetchZidsO(context.Context) (*id.Set, error)
}

// Make creates a new Mapper.
func Make(fetcher Fetcher) *Mapper {
	defined := map[id.Zid]id.ZidN{
		id.Invalid: id.InvalidN,
		1:          id.MustParseN("0001"), // ZidVersion
		2:          id.MustParseN("0002"), // ZidHost
		3:          id.MustParseN("0003"), // ZidOperatingSystem
		4:          id.MustParseN("0004"), // ZidLicense
		5:          id.MustParseN("0005"), // ZidAuthors
		6:          id.MustParseN("0006"), // ZidDependencies
		7:          id.MustParseN("0007"), // ZidLog
		8:          id.MustParseN("0008"), // ZidMemory
		9:          id.MustParseN("0009"), // ZidSx
		10:         id.MustParseN("000a"), // ZidHTTP
		11:         id.MustParseN("000b"), // ZidAPI
		12:         id.MustParseN("000c"), // ZidWebUI
		13:         id.MustParseN("000d"), // ZidConsole
		20:         id.MustParseN("000e"), // ZidBoxManager
		21:         id.MustParseN("000f"), // ZidZettel
		22:         id.MustParseN("000g"), // ZidIndex
		23:         id.MustParseN("000h"), // ZidQuery
		90:         id.MustParseN("000i"), // ZidMetadataKey
		92:         id.MustParseN("000j"), // ZidParser
		96:         id.MustParseN("000k"), // ZidStartupConfiguration
		100:        id.MustParseN("000l"), // ZidRuntimeConfiguration
		101:        id.MustParseN("000m"), // ZidDirectory
		102:        id.MustParseN("000n"), // ZidWarnings
		10100:      id.MustParseN("000s"), // Base HTML Template
		10200:      id.MustParseN("000t"), // Login Form Template
		10300:      id.MustParseN("000u"), // List Zettel Template
		10401:      id.MustParseN("000v"), // Detail Template
		10402:      id.MustParseN("000w"), // Info Template
		10403:      id.MustParseN("000x"), // Form Template
		10405:      id.MustParseN("000y"), // Delete Template
		10700:      id.MustParseN("000z"), // Error Template
		19000:      id.MustParseN("000q"), // Sxn Start Code
		19990:      id.MustParseN("000r"), // Sxn Base Code
		20001:      id.MustParseN("0010"), // Base CSS
		25001:      id.MustParseN("0011"), // User CSS
		40001:      id.MustParseN("000o"), // Generic Emoji
		59900:      id.MustParseN("000p"), // Sxn Prelude
		60010:      id.MustParseN("0012"), // zettel
		60020:      id.MustParseN("0013"), // confguration
		60030:      id.MustParseN("0014"), // role
		60040:      id.MustParseN("0015"), // tag
		90000:      id.MustParseN("0016"), // New Menu
		90001:      id.MustParseN("0017"), // New Zettel
		90002:      id.MustParseN("0018"), // New User
		90003:      id.MustParseN("0019"), // New Tag
		90004:      id.MustParseN("001a"), // New Role
		// 100000000,   // Manual               -> 0020-00yz
		9999999996:  id.MustParseN("00zw"), // Current ZidMapping, TEMP for v0.19-dev
		9999999997:  id.MustParseN("00zx"), // ZidSession
		9999999998:  id.MustParseN("00zy"), // ZidAppDirectory
		9999999999:  id.MustParseN("00zz"), // ZidMapping
		10000000000: id.MustParseN("0100"), // ZidDefaultHome
	}
	toNew := maps.Clone(defined)
	toOld := make(map[id.ZidN]id.Zid, len(toNew))
	for o, n := range toNew {
		if _, found := toOld[n]; found {
			panic("duplicate predefined zid")
		}
		toOld[n] = o
	}

	return &Mapper{
		fetcher:   fetcher,
		defined:   defined,
		toNew:     toNew,
		toOld:     toOld,
		nextZidM:  id.MustParseN("0020"),
		hadManual: false,
		nextZidN:  id.MustParseN("0101"),
	}
}

// isWellDefined returns true, if the given zettel identifier is predefined
// (as stated in the manual), or is part of the manual itself, or is greater than
// 19699999999999.
func (zm *Mapper) isWellDefined(zid id.Zid) bool {
	if _, found := zm.defined[zid]; found || (1000000000 <= zid && zid <= 1099999999) {
		return true
	}
	if _, err := time.Parse("20060102150405", zid.String()); err != nil {
		return false
	}
	return 19700000000000 <= zid
}

// Warnings returns all zettel identifier with warnings.
func (zm *Mapper) Warnings(ctx context.Context) (*id.Set, error) {
	allZidsO, err := zm.fetcher.FetchZidsO(ctx)
	if err != nil {
		return nil, err
	}
	warnings := id.NewSet()
	allZidsO.ForEach(func(zid id.Zid) {
		if !zm.isWellDefined(zid) {
			warnings = warnings.Add(zid)
		}
	})
	return warnings, nil
}

// LookupZidN returns the new-style identifier for a given old-style identifier.
func (zm *Mapper) LookupZidN(zidO id.Zid) (id.ZidN, bool) {
	if !zidO.IsValid() {
		panic(zidO)
	}
	zm.mx.RLock()
	zidN, found := zm.toNew[zidO]
	zm.mx.RUnlock()
	return zidN, found
}

// GetZidN returns a new-style identifier for a given old-style identifier.
// If the old-style identifier is currently not mapped, the mapping will be
// established.
func (zm *Mapper) GetZidN(zidO id.Zid) id.ZidN {
	if zidN, found := zm.LookupZidN(zidO); found {
		return zidN
	}

	zm.mx.Lock()
	defer zm.mx.Unlock()
	// Double check to avoid races
	if zidN, found := zm.toNew[zidO]; found {
		return zidN
	}

	if 1000000000 <= zidO && zidO <= 1099999999 {
		if zidO == 1000000000 {
			zm.hadManual = true
		}
		if zm.hadManual {
			zidN := zm.nextZidM
			zm.nextZidM++
			zm.toNew[zidO] = zidN
			zm.toOld[zidN] = zidO
			return zidN
		}
	}

	zidN := zm.nextZidN
	zm.nextZidN++
	zm.toNew[zidO] = zidN
	zm.toOld[zidN] = zidO
	return zidN
}

// DeleteO removes a mapping with the given old-style identifier.
func (zm *Mapper) DeleteO(zidO id.Zid) {
	if _, found := zm.defined[zidO]; found {
		return
	}
	zm.mx.Lock()
	if zidN, found := zm.toNew[zidO]; found {
		delete(zm.toNew, zidO)
		delete(zm.toOld, zidN)
		if lastZidN := zm.nextZidN - 1; zidN == lastZidN {
			zm.nextZidN = lastZidN
		}
	}
	zm.mx.Unlock()
}

// AsBytes returns the current mapping as lines, where each line contains the
// old and the new zettel identifier.
func (zm *Mapper) AsBytes() []byte {
	zm.mx.RLock()
	defer zm.mx.RUnlock()
	return zm.asBytes()
}
func (zm *Mapper) asBytes() []byte {
	allZidsO := id.NewSetCap(len(zm.toNew))
	for zidO := range zm.toNew {
		allZidsO = allZidsO.Add(zidO)
	}
	var buf bytes.Buffer
	first := true
	allZidsO.ForEach(func(zidO id.Zid) {
		if !first {
			buf.WriteByte('\n')
		}
		first = false
		zidN := zm.toNew[zidO]
		buf.WriteString(zidO.String())
		buf.WriteByte(' ')
		buf.WriteString(zidN.String())
	})
	return buf.Bytes()
}

// FetchAsBytes fetches all zettel identifier and returns the mapping as lines,
// where each line contains the old zid and the new zid.
func (zm *Mapper) FetchAsBytes(ctx context.Context) ([]byte, error) {
	allZidsO, err := zm.fetcher.FetchZidsO(ctx)
	if err != nil {
		return nil, err
	}
	allZidsO.ForEach(func(zidO id.Zid) {
		_ = zm.GetZidN(zidO)
	})
	zm.mx.Lock()
	defer zm.mx.Unlock()
	if len(zm.toNew) != allZidsO.Length() {
		for zidO, zidN := range zm.toNew {
			if allZidsO.Contains(zidO) {
				continue
			}
			delete(zm.toNew, zidO)
			delete(zm.toOld, zidN)
		}
	}
	return zm.asBytes(), nil
}

// ParseAndUpdate parses the given content and updates the Mapping.
func (zm *Mapper) ParseAndUpdate(content []byte) (err error) {
	zm.mx.Lock()
	defer zm.mx.Unlock()
	inp := input.NewInput(content)
	for inp.Ch != input.EOS {
		inp.SkipSpace()
		pos := inp.Pos
		zidO := readZidO(inp)
		if !zidO.IsValid() {
			inp.SkipToEOL()
			inp.EatEOL()
			if err == nil {
				err = fmt.Errorf("unable to parse old zid: %q", string(inp.Src[pos:inp.Pos]))
			}
			continue
		}
		inp.SkipSpace()
		zidN := readZidN(inp)
		if !zidN.IsValid() {
			inp.SkipToEOL()
			inp.EatEOL()
			if err == nil {
				err = fmt.Errorf("unable to parse new zid: %q", string(inp.Src[pos:inp.Pos]))
			}
			continue
		}
		inp.SkipToEOL()
		inp.EatEOL()

		if oldZidN, found := zm.toNew[zidO]; found {
			if oldZidN != zidN {
				err = fmt.Errorf("old zid %v already mapped to %v, overwrite: %v", zidO, oldZidN, zidN)
			}
			continue
		}
		zm.toNew[zidO] = zidN
		zm.toOld[zidN] = zidO
		zm.nextZidN = max(zm.nextZidN, zidN+1)
	}
	return err
}

func readZidO(inp *input.Input) id.Zid {
	pos := inp.Pos
	for '0' <= inp.Ch && inp.Ch <= '9' {
		inp.Next()
	}
	zidO, _ := id.Parse(string(inp.Src[pos:inp.Pos]))
	return zidO
}
func readZidN(inp *input.Input) id.ZidN {
	pos := inp.Pos
	for ('0' <= inp.Ch && inp.Ch <= '9') || ('a' <= inp.Ch && inp.Ch <= 'z') || ('A' <= inp.Ch && inp.Ch <= 'Z') {
		inp.Next()
	}
	zidN, _ := id.ParseN(string(inp.Src[pos:inp.Pos]))
	return zidN
}
