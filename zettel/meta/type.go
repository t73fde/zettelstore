//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
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

package meta

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
)

// DescriptionType is a description of a specific key type.
type DescriptionType struct {
	Name  string
	IsSet bool
}

// String returns the string representation of the given type
func (t DescriptionType) String() string { return t.Name }

var registeredTypes = make(map[string]*DescriptionType)

func registerType(name string, isSet bool) *DescriptionType {
	if _, ok := registeredTypes[name]; ok {
		panic("Type '" + name + "' already registered")
	}
	t := &DescriptionType{name, isSet}
	registeredTypes[name] = t
	return t
}

// Supported key types.
var (
	TypeCredential = registerType(api.MetaCredential, false)
	TypeEmpty      = registerType(api.MetaEmpty, false)
	TypeID         = registerType(api.MetaID, false)
	TypeIDSet      = registerType(api.MetaIDSet, true)
	TypeNumber     = registerType(api.MetaNumber, false)
	TypeString     = registerType(api.MetaString, false)
	TypeTagSet     = registerType(api.MetaTagSet, true)
	TypeTimestamp  = registerType(api.MetaTimestamp, false)
	TypeURL        = registerType(api.MetaURL, false)
	TypeWord       = registerType(api.MetaWord, false)
)

// Type returns a type hint for the given key. If no type hint is specified,
// TypeUnknown is returned.
func (*Meta) Type(key string) *DescriptionType {
	return Type(key)
}

// Some constants for key suffixes that determine a type.
const (
	SuffixKeyRole = "-role"
	SuffixKeyURL  = "-url"
)

var (
	cachedTypedKeys = make(map[string]*DescriptionType)
	mxTypedKey      sync.RWMutex
	suffixTypes     = map[string]*DescriptionType{
		"-date":       TypeTimestamp,
		"-number":     TypeNumber,
		SuffixKeyRole: TypeWord,
		"-time":       TypeTimestamp,
		SuffixKeyURL:  TypeURL,
		"-zettel":     TypeID,
		"-zid":        TypeID,
		"-zids":       TypeIDSet,
	}
)

// Type returns a type hint for the given key. If no type hint is specified,
// TypeEmpty is returned.
func Type(key string) *DescriptionType {
	if k, ok := registeredKeys[key]; ok {
		return k.Type
	}
	mxTypedKey.RLock()
	k, found := cachedTypedKeys[key]
	mxTypedKey.RUnlock()
	if found {
		return k
	}

	for suffix, t := range suffixTypes {
		if strings.HasSuffix(key, suffix) {
			mxTypedKey.Lock()
			defer mxTypedKey.Unlock()
			// Double check to avoid races
			if _, found = cachedTypedKeys[key]; !found {
				cachedTypedKeys[key] = t
			}
			return t
		}
	}
	return TypeEmpty
}

// SetList stores the given string list value under the given key.
func (m *Meta) SetList(key string, values []string) {
	if key != api.KeyID {
		for i, val := range values {
			values[i] = trimValue(val)
		}
		m.pairs[key] = strings.Join(values, " ")
	}
}

// SetWord stores the given word under the given key.
func (m *Meta) SetWord(key, word string) {
	if slist := ListFromValue(word); len(slist) > 0 {
		m.Set(key, slist[0])
	}
}

// SetNow stores the current timestamp under the given key.
func (m *Meta) SetNow(key string) {
	m.Set(key, time.Now().Local().Format(id.TimestampLayout))
}

// BoolValue returns the value interpreted as a bool.
func BoolValue(value string) bool {
	if len(value) > 0 {
		switch value[0] {
		case '0', 'f', 'F', 'n', 'N':
			return false
		}
	}
	return true
}

// GetBool returns the boolean value of the given key.
func (m *Meta) GetBool(key string) bool {
	if value, ok := m.Get(key); ok {
		return BoolValue(value)
	}
	return false
}

// TimeValue returns the time value of the given value.
func TimeValue(value string) (time.Time, bool) {
	if t, err := time.Parse(id.TimestampLayout, ExpandTimestamp(value)); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// ExpandTimestamp makes a short-form timestamp larger.
func ExpandTimestamp(value string) string {
	switch l := len(value); l {
	case 4: // YYYY
		return value + "0101000000"
	case 6: // YYYYMM
		return value + "01000000"
	case 8, 10, 12: // YYYYMMDD, YYYYMMDDhh, YYYYMMDDhhmm
		return value + "000000"[:14-l]
	case 14: // YYYYMMDDhhmmss
		return value
	default:
		if l > 14 {
			return value[:14]
		}
		return value
	}
}

// ListFromValue transforms a string value into a list value.
func ListFromValue(value string) []string {
	return strings.Fields(value)
}

// GetList retrieves the string list value of a given key. The bool value
// signals, whether there was a value stored or not.
func (m *Meta) GetList(key string) ([]string, bool) {
	value, ok := m.Get(key)
	if !ok {
		return nil, false
	}
	return ListFromValue(value), true
}

// TagsFromValue returns the value as a sequence of normalized tags.
func TagsFromValue(value string) []string {
	tags := ListFromValue(strings.ToLower(value))
	for i, tag := range tags {
		if len(tag) > 1 && tag[0] == '#' {
			tags[i] = tag[1:]
		}
	}
	return tags
}

// CleanTag removes the number character ('#') from a tag value and lowercases it.
func CleanTag(tag string) string {
	if len(tag) > 1 && tag[0] == '#' {
		return tag[1:]
	}
	return tag
}

// NormalizeTag adds a missing prefix "#" to the tag
func NormalizeTag(tag string) string {
	if len(tag) > 0 && tag[0] == '#' {
		return tag
	}
	return "#" + tag
}

// GetNumber retrieves the numeric value of a given key.
func (m *Meta) GetNumber(key string, def int64) int64 {
	if value, ok := m.Get(key); ok {
		if num, err := strconv.ParseInt(value, 10, 64); err == nil {
			return num
		}
	}
	return def
}
