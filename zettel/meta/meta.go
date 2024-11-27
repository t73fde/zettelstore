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

// Package meta provides the zettel specific type 'meta'.
package meta

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/input"
	"t73f.de/r/zsc/maps"
	"zettelstore.de/z/strfun"
	"zettelstore.de/z/zettel/id"
)

type keyUsage int

const (
	_             keyUsage = iota
	usageUser              // Key will be manipulated by the user
	usageComputed          // Key is computed by zettelstore
	usageProperty          // Key is computed and not stored by zettelstore
)

// DescriptionKey formally describes each supported metadata key.
type DescriptionKey struct {
	Name    string
	Type    *DescriptionType
	usage   keyUsage
	Inverse string
}

// IsComputed returns true, if metadata is computed and not set by the user.
func (kd *DescriptionKey) IsComputed() bool { return kd.usage >= usageComputed }

// IsProperty returns true, if metadata is a computed property.
func (kd *DescriptionKey) IsProperty() bool { return kd.usage >= usageProperty }

var registeredKeys = make(map[string]*DescriptionKey)

func registerKey(name string, t *DescriptionType, usage keyUsage, inverse string) {
	if _, ok := registeredKeys[name]; ok {
		panic("Key '" + name + "' already defined")
	}
	if inverse != "" {
		if t != TypeID && t != TypeIDSet {
			panic("Inversable key '" + name + "' is not identifier type, but " + t.String())
		}
		inv, ok := registeredKeys[inverse]
		if !ok {
			panic("Inverse Key '" + inverse + "' not found")
		}
		if !inv.IsComputed() {
			panic("Inverse Key '" + inverse + "' is not computed.")
		}
		if inv.Type != TypeIDSet {
			panic("Inverse Key '" + inverse + "' is not an identifier set, but " + inv.Type.String())
		}
	}
	registeredKeys[name] = &DescriptionKey{name, t, usage, inverse}
}

// IsComputed returns true, if key denotes a computed metadata key.
func IsComputed(name string) bool {
	if kd, ok := registeredKeys[name]; ok {
		return kd.IsComputed()
	}
	return false
}

// IsProperty returns true, if key denotes a property metadata value.
func IsProperty(name string) bool {
	if kd, ok := registeredKeys[name]; ok {
		return kd.IsProperty()
	}
	return false
}

// Inverse returns the name of the inverse key.
func Inverse(name string) string {
	if kd, ok := registeredKeys[name]; ok {
		return kd.Inverse
	}
	return ""
}

// GetDescription returns the key description object of the given key name.
func GetDescription(name string) DescriptionKey {
	if d, ok := registeredKeys[name]; ok {
		return *d
	}
	return DescriptionKey{Type: Type(name)}
}

// GetSortedKeyDescriptions delivers all metadata key descriptions as a slice, sorted by name.
func GetSortedKeyDescriptions() []*DescriptionKey {
	keys := maps.Keys(registeredKeys)
	result := make([]*DescriptionKey, 0, len(keys))
	for _, n := range keys {
		result = append(result, registeredKeys[n])
	}
	return result
}

// KeyCreatedMissing is temporary until migration to B36 has ended.
// It is not an "official" key to be designed to last long.
const KeyCreatedMissing = "created-missing"

// Supported keys.
func init() {
	registerKey(api.KeyID, TypeID, usageComputed, "")
	registerKey(api.KeyTitle, TypeEmpty, usageUser, "")
	registerKey(api.KeyRole, TypeWord, usageUser, "")
	registerKey(api.KeyTags, TypeTagSet, usageUser, "")
	registerKey(api.KeySyntax, TypeWord, usageUser, "")

	// Properties that are inverse keys
	registerKey(api.KeyFolge, TypeIDSet, usageProperty, "")
	registerKey(api.KeySequel, TypeIDSet, usageProperty, "")
	registerKey(api.KeySuccessors, TypeIDSet, usageProperty, "")
	registerKey(api.KeySubordinates, TypeIDSet, usageProperty, "")

	// Non-inverse keys
	registerKey(api.KeyAuthor, TypeString, usageUser, "")
	registerKey(api.KeyBack, TypeIDSet, usageProperty, "")
	registerKey(api.KeyBackward, TypeIDSet, usageProperty, "")
	registerKey(api.KeyBoxNumber, TypeNumber, usageProperty, "")
	registerKey(api.KeyCopyright, TypeString, usageUser, "")
	registerKey(api.KeyCreated, TypeTimestamp, usageComputed, "")
	registerKey(api.KeyCredential, TypeCredential, usageUser, "")
	registerKey(KeyCreatedMissing, TypeWord, usageProperty, "")
	registerKey(api.KeyDead, TypeIDSet, usageProperty, "")
	registerKey(api.KeyExpire, TypeTimestamp, usageUser, "")
	registerKey(api.KeyFolgeRole, TypeWord, usageUser, "")
	registerKey(api.KeyForward, TypeIDSet, usageProperty, "")
	registerKey(api.KeyLang, TypeWord, usageUser, "")
	registerKey(api.KeyLicense, TypeEmpty, usageUser, "")
	registerKey(api.KeyModified, TypeTimestamp, usageComputed, "")
	registerKey(api.KeyPrecursor, TypeIDSet, usageUser, api.KeyFolge)
	registerKey(api.KeyPredecessor, TypeID, usageUser, api.KeySuccessors)
	registerKey(api.KeyPrequel, TypeIDSet, usageUser, api.KeySequel)
	registerKey(api.KeyPublished, TypeTimestamp, usageProperty, "")
	registerKey(api.KeyQuery, TypeEmpty, usageUser, "")
	registerKey(api.KeyReadOnly, TypeWord, usageUser, "")
	registerKey(api.KeySummary, TypeZettelmarkup, usageUser, "")
	registerKey(api.KeySuperior, TypeIDSet, usageUser, api.KeySubordinates)
	registerKey(api.KeyURL, TypeURL, usageUser, "")
	registerKey(api.KeyUselessFiles, TypeString, usageProperty, "")
	registerKey(api.KeyUserID, TypeWord, usageUser, "")
	registerKey(api.KeyUserRole, TypeWord, usageUser, "")
	registerKey(api.KeyVisibility, TypeWord, usageUser, "")
}

// NewPrefix is the prefix for metadata key in template zettel for creating new zettel.
const NewPrefix = "new-"

// Meta contains all meta-data of a zettel.
type Meta struct {
	Zid     id.Zid
	pairs   map[string]string
	YamlSep bool
}

// New creates a new chunk for storing metadata.
func New(zid id.Zid) *Meta {
	return &Meta{Zid: zid, pairs: make(map[string]string, 5)}
}

// NewWithData creates metadata object with given data.
func NewWithData(zid id.Zid, data map[string]string) *Meta {
	pairs := make(map[string]string, len(data))
	for k, v := range data {
		pairs[k] = v
	}
	return &Meta{Zid: zid, pairs: pairs}
}

// Length returns the number of bytes stored for the metadata.
func (m *Meta) Length() int {
	if m == nil {
		return 0
	}
	result := 6 // storage needed for Zid
	for k, v := range m.pairs {
		result += len(k) + len(v) + 1 // 1 because separator
	}
	return result
}

// Clone returns a new copy of the metadata.
func (m *Meta) Clone() *Meta {
	return &Meta{
		Zid:     m.Zid,
		pairs:   m.Map(),
		YamlSep: m.YamlSep,
	}
}

// Map returns a copy of the meta data as a string map.
func (m *Meta) Map() map[string]string {
	pairs := make(map[string]string, len(m.pairs))
	for k, v := range m.pairs {
		pairs[k] = v
	}
	return pairs
}

var reKey = regexp.MustCompile("^[0-9a-z][-0-9a-z]{0,254}$")

// KeyIsValid returns true, if the string is a valid metadata key.
func KeyIsValid(s string) bool { return reKey.MatchString(s) }

// Pair is one key-value-pair of a Zettel meta.
type Pair struct {
	Key   string
	Value string
}

var firstKeys = []string{api.KeyTitle, api.KeyRole, api.KeyTags, api.KeySyntax}
var firstKeySet strfun.Set

func init() {
	firstKeySet = strfun.NewSet(firstKeys...)
}

// Set stores the given string value under the given key.
func (m *Meta) Set(key, value string) {
	if key != api.KeyID {
		m.pairs[key] = trimValue(value)
	}
}

// SetNonEmpty stores the given value under the given key, if the value is non-empty.
// An empty value will delete the previous association.
func (m *Meta) SetNonEmpty(key, value string) {
	if value == "" {
		delete(m.pairs, key)
	} else {
		m.Set(key, trimValue(value))
	}
}

func trimValue(value string) string {
	return strings.TrimFunc(value, input.IsSpace)
}

// Get retrieves the string value of a given key. The bool value signals,
// whether there was a value stored or not.
func (m *Meta) Get(key string) (string, bool) {
	if m == nil {
		return "", false
	}
	if key == api.KeyID {
		return m.Zid.String(), true
	}
	value, ok := m.pairs[key]
	return value, ok
}

// GetDefault retrieves the string value of the given key. If no value was
// stored, the given default value is returned.
func (m *Meta) GetDefault(key, def string) string {
	if value, found := m.Get(key); found {
		return value
	}
	return def
}

// GetTitle returns the title of the metadata. It is the only key that has a
// defined default value: the string representation of the zettel identifier.
func (m *Meta) GetTitle() string {
	if title, found := m.Get(api.KeyTitle); found {
		return title
	}
	return m.Zid.String()
}

// Pairs returns not computed key/values pairs stored, in a specific order.
// First come the pairs with predefined keys: MetaTitleKey, MetaTagsKey, MetaSyntaxKey,
// MetaContextKey. Then all other pairs are append to the list, ordered by key.
func (m *Meta) Pairs() []Pair {
	return m.doPairs(m.getFirstKeys(), notComputedKey)
}

// ComputedPairs returns all key/values pairs stored, in a specific order. First come
// the pairs with predefined keys: MetaTitleKey, MetaTagsKey, MetaSyntaxKey,
// MetaContextKey. Then all other pairs are append to the list, ordered by key.
func (m *Meta) ComputedPairs() []Pair {
	return m.doPairs(m.getFirstKeys(), anyKey)
}

// PairsRest returns not computed key/values pairs stored, except the values with
// predefined keys. The pairs are ordered by key.
func (m *Meta) PairsRest() []Pair {
	result := make([]Pair, 0, len(m.pairs))
	return m.doPairs(result, notComputedKey)
}

// ComputedPairsRest returns all key/values pairs stored, except the values with
// predefined keys. The pairs are ordered by key.
func (m *Meta) ComputedPairsRest() []Pair {
	result := make([]Pair, 0, len(m.pairs))
	return m.doPairs(result, anyKey)
}

func notComputedKey(key string) bool { return !IsComputed(key) }
func anyKey(string) bool             { return true }

func (m *Meta) doPairs(firstKeys []Pair, addKeyPred func(string) bool) []Pair {
	keys := m.getKeysRest(addKeyPred)
	for _, k := range keys {
		firstKeys = append(firstKeys, Pair{k, m.pairs[k]})
	}
	return firstKeys
}

func (m *Meta) getFirstKeys() []Pair {
	result := make([]Pair, 0, len(m.pairs))
	for _, key := range firstKeys {
		if value, ok := m.pairs[key]; ok {
			result = append(result, Pair{key, value})
		}
	}
	return result
}

func (m *Meta) getKeysRest(addKeyPred func(string) bool) []string {
	keys := make([]string, 0, len(m.pairs))
	for k := range m.pairs {
		if !firstKeySet.Has(k) && addKeyPred(k) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// Delete removes a key from the data.
func (m *Meta) Delete(key string) {
	if key != api.KeyID {
		delete(m.pairs, key)
	}
}

// Equal compares to metas for equality.
func (m *Meta) Equal(o *Meta, allowComputed bool) bool {
	if m == nil && o == nil {
		return true
	}
	if m == nil || o == nil || m.Zid != o.Zid {
		return false
	}
	tested := make(strfun.Set, len(m.pairs))
	for k, v := range m.pairs {
		tested.Set(k)
		if !equalValue(k, v, o, allowComputed) {
			return false
		}
	}
	for k, v := range o.pairs {
		if !tested.Has(k) && !equalValue(k, v, m, allowComputed) {
			return false
		}
	}
	return true
}

func equalValue(key, val string, other *Meta, allowComputed bool) bool {
	if allowComputed || !IsComputed(key) {
		if valO, found := other.pairs[key]; !found || val != valO {
			return false
		}
	}
	return true
}

// Sanitize all metadata keys and values, so that they can be written safely into a file.
func (m *Meta) Sanitize() {
	if m == nil {
		return
	}
	for k, v := range m.pairs {
		m.pairs[RemoveNonGraphic(k)] = RemoveNonGraphic(v)
	}
}

// RemoveNonGraphic changes the given string not to include non-graphical characters.
// It is needed to sanitize meta data.
func RemoveNonGraphic(s string) string {
	if s == "" {
		return ""
	}
	pos := 0
	var sb strings.Builder
	for pos < len(s) {
		nextPos := strings.IndexFunc(s[pos:], func(r rune) bool { return !unicode.IsGraphic(r) })
		if nextPos < 0 {
			break
		}
		sb.WriteString(s[pos:nextPos])
		sb.WriteByte(' ')
		_, size := utf8.DecodeRuneInString(s[nextPos:])
		pos = nextPos + size
	}
	if pos == 0 {
		return strings.TrimSpace(s)
	}
	sb.WriteString(s[pos:])
	return strings.TrimSpace(sb.String())
}
