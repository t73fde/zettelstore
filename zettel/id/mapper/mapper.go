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
// to identifier with four characters.
package mapper

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"strconv"
	"sync"
	"time"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/input"
	"zettelstore.de/z/zettel/id"
)

// ----- Base36 zettel identifier.

// ZidN is the internal identifier of a zettel. It is a number in the range
// 1..36^4-1 (1..1679615), as it is externally represented by four alphanumeric
// characters.
type ZidN uint32

// Some important ZettelIDs.
const (
	InvalidN = ZidN(0) // Invalid is a Zid that will never be valid
)

const maxZidN = 36*36*36*36 - 1

// ParseUintN interprets a string as a possible zettel identifier
// and returns its integer value.
func ParseUintN(s string) (uint64, error) {
	res, err := strconv.ParseUint(s, 36, 21)
	if err != nil {
		return 0, err
	}
	if res == 0 || res > maxZidN {
		return res, strconv.ErrRange
	}
	return res, nil
}

// ParseN interprets a string as a zettel identification and
// returns its value.
func ParseN(s string) (ZidN, error) {
	if len(s) != 4 {
		return InvalidN, strconv.ErrSyntax
	}
	res, err := ParseUintN(s)
	if err != nil {
		return InvalidN, err
	}
	return ZidN(res), nil
}

// MustParseN tries to interpret a string as a zettel identifier and returns
// its value or panics otherwise.
func MustParseN(s api.ZettelID) ZidN {
	zid, err := ParseN(string(s))
	if err == nil {
		return zid
	}
	panic(err)
}

// String converts the zettel identification to a string of 14 digits.
// Only defined for valid ids.
func (zid ZidN) String() string {
	var result [4]byte
	zid.toByteArray(&result)
	return string(result[:])
}

// ZettelID return the zettel identification as a api.ZettelID.
func (zid ZidN) ZettelID() api.ZettelID { return api.ZettelID(zid.String()) }

// Bytes converts the zettel identification to a byte slice of 14 digits.
// Only defined for valid ids.
func (zid ZidN) Bytes() []byte {
	var result [4]byte
	zid.toByteArray(&result)
	return result[:]
}

// toByteArray converts the Zid into a fixed byte array, usable for printing.
//
// Based on idea by Daniel Lemire: "Converting integers to fix-digit representations quickly"
// https://lemire.me/blog/2021/11/18/converting-integers-to-fix-digit-representations-quickly/
func (zid ZidN) toByteArray(result *[4]byte) {
	d12 := uint32(zid) / (36 * 36)
	d1 := d12 / 36
	d2 := d12 % 36
	d34 := uint32(zid) % (36 * 36)
	d3 := d34 / 36
	d4 := d34 % 36

	const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result[0] = digits[d1]
	result[1] = digits[d2]
	result[2] = digits[d3]
	result[3] = digits[d4]
}

// IsValid determines if zettel id is a valid one, e.g. consists of max. 14 digits.
func (zid ZidN) IsValid() bool { return 0 < zid && zid <= maxZidN }

// Mapper transforms old-style zettel identifier (14 digits) into new one (4 alphanums).
//
// Since there are no new-style identifier defined, there is only support for old-style
// identifier by checking, whether they are suported as new-style or not.
//
// This will change in later versions.
type Mapper struct {
	fetcher   Fetcher
	defined   map[id.Zid]ZidN // predefined mapping, constant after creation
	mx        sync.RWMutex    // protect toNew ... nextZidN
	toNew     map[id.Zid]ZidN // working mapping old->new
	toOld     map[ZidN]id.Zid // working mapping new->old
	nextZidM  ZidN            // next zid for manual
	hadManual bool
	nextZidN  ZidN // next zid for normal zettel
}

// Fetcher is an object that will fetch all identifier currently in user.
type Fetcher interface {
	// FetchZidsO fetch all old-style zettel identifier.
	FetchZidsO(context.Context) (*id.Set, error)
}

// Make creates a new Mapper.
func Make(fetcher Fetcher) *Mapper {
	defined := map[id.Zid]ZidN{
		id.Invalid: InvalidN,
		1:          MustParseN("0001"), // ZidVersion
		2:          MustParseN("0002"), // ZidHost
		3:          MustParseN("0003"), // ZidOperatingSystem
		4:          MustParseN("0004"), // ZidLicense
		5:          MustParseN("0005"), // ZidAuthors
		6:          MustParseN("0006"), // ZidDependencies
		7:          MustParseN("0007"), // ZidLog
		8:          MustParseN("0008"), // ZidMemory
		9:          MustParseN("0009"), // ZidSx
		10:         MustParseN("000a"), // ZidHTTP
		11:         MustParseN("000b"), // ZidAPI
		12:         MustParseN("000c"), // ZidWebUI
		13:         MustParseN("000d"), // ZidConsole
		20:         MustParseN("000e"), // ZidBoxManager
		21:         MustParseN("000f"), // ZidZettel
		22:         MustParseN("000g"), // ZidIndex
		23:         MustParseN("000h"), // ZidQuery
		90:         MustParseN("000i"), // ZidMetadataKey
		92:         MustParseN("000j"), // ZidParser
		96:         MustParseN("000k"), // ZidStartupConfiguration
		100:        MustParseN("000l"), // ZidRuntimeConfiguration
		101:        MustParseN("000m"), // ZidDirectory
		102:        MustParseN("000n"), // ZidWarnings
		10100:      MustParseN("000s"), // Base HTML Template
		10200:      MustParseN("000t"), // Login Form Template
		10300:      MustParseN("000u"), // List Zettel Template
		10401:      MustParseN("000v"), // Detail Template
		10402:      MustParseN("000w"), // Info Template
		10403:      MustParseN("000x"), // Form Template
		10405:      MustParseN("000y"), // Delete Template
		10700:      MustParseN("000z"), // Error Template
		19000:      MustParseN("000q"), // Sxn Start Code
		19990:      MustParseN("000r"), // Sxn Base Code
		20001:      MustParseN("0010"), // Base CSS
		25001:      MustParseN("0011"), // User CSS
		40001:      MustParseN("000o"), // Generic Emoji
		59900:      MustParseN("000p"), // Sxn Prelude
		60010:      MustParseN("0012"), // zettel
		60020:      MustParseN("0013"), // confguration
		60030:      MustParseN("0014"), // role
		60040:      MustParseN("0015"), // tag
		90000:      MustParseN("0016"), // New Menu
		90001:      MustParseN("0017"), // New Zettel
		90002:      MustParseN("0018"), // New User
		90003:      MustParseN("0019"), // New Tag
		90004:      MustParseN("001a"), // New Role
		// 100000000,   // Manual               -> 0020-00yz
		9999999997:  MustParseN("00zx"), // ZidSession
		9999999998:  MustParseN("00zy"), // ZidAppDirectory
		9999999999:  MustParseN("00zz"), // ZidMapping
		10000000000: MustParseN("0100"), // ZidDefaultHome
	}
	toNew := maps.Clone(defined)
	toOld := make(map[ZidN]id.Zid, len(toNew))
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
		nextZidM:  MustParseN("0020"),
		hadManual: false,
		nextZidN:  MustParseN("0101"),
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
func (zm *Mapper) LookupZidN(zidO id.Zid) (ZidN, bool) {
	if !zidO.IsValid() {
		panic(zidO)
	}
	zm.mx.RLock()
	zidN, found := zm.toNew[zidO]
	zm.mx.RUnlock()
	return zidN, found
}

// AllocateZidN allocates a new new-style identifier, which is associated with
// the given old-style identifier.
func (zm *Mapper) AllocateZidN(zidO id.Zid) ZidN {
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

// LookupZidO returns the old-style identifier for a new-style identifier.
func (zm *Mapper) LookupZidO(zidN ZidN) (id.Zid, bool) {
	if zm != nil {
		zm.mx.RLock()
		zidO, found := zm.toOld[zidN]
		zm.mx.RUnlock()
		return zidO, found
	}
	return id.Invalid, false
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

// Fetch all zettel and update the mapping.
func (zm *Mapper) Fetch(ctx context.Context) error {
	allZidsO, err := zm.fetcher.FetchZidsO(ctx)
	if err != nil {
		return err
	}
	allZidsO.ForEach(func(zidO id.Zid) {
		zm.AllocateZidN(zidO)
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
	return nil
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
func readZidN(inp *input.Input) ZidN {
	pos := inp.Pos
	for ('0' <= inp.Ch && inp.Ch <= '9') || ('a' <= inp.Ch && inp.Ch <= 'z') || ('A' <= inp.Ch && inp.Ch <= 'Z') {
		inp.Next()
	}
	zidN, _ := ParseN(string(inp.Src[pos:inp.Pos]))
	return zidN
}
