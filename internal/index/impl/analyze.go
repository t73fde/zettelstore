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

package impl

import (
	"cmp"
	"iter"
	"net/url"
	"slices"
	"strings"

	"t73f.de/r/sx"
	zeroiter "t73f.de/r/zero/iter"
	"t73f.de/r/zero/roster"
	zeroslices "t73f.de/r/zero/slices"
	zerostrings "t73f.de/r/zero/strings"
	"t73f.de/r/zero/vocab"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"
)

type analyzer struct {
	dict *dictionary
}

func makeAnalyzer(dict *dictionary) *analyzer {
	return &analyzer{dict: dict}
}

func (a *analyzer) processDeleted(in *analyzeData, out *storeData) {
	out.zid = in.zid
	out.aztl = nil
	out.bulk = in.bulk
	out.dzid = a.dict.Encode(in.zid)
}

type analyzedZettel struct {
	meta     storeMeta
	forward  roster.Roster[denseZid]
	words    roster.Roster[vocab.WordID]
	urls     roster.Roster[string]
	metaZids []metaZid
}

type metaZid struct {
	keyID   vocab.WordID
	forward roster.Roster[denseZid]
}

func (az *analyzedZettel) shrink() {
	az.meta.shrink()
	az.forward.Shrink()
	az.words.Shrink()
	az.urls.Shrink()
	for i := range az.metaZids {
		az.metaZids[i].forward.Shrink()
	}
	az.metaZids = zeroslices.Shrink(az.metaZids)
}

func (a *analyzer) analyze(in *analyzeData, out *storeData) {
	out.zid = in.zid
	out.bulk = in.bulk
	out.dzid = a.dict.Encode(in.zid)

	az := &analyzedZettel{}
	za := zettelAnalyzer{a, az}
	za.collect(in.zettel.Meta, in.zettel.Blocks)
	az.shrink()
	out.aztl = az
}

type zettelAnalyzer struct {
	a  *analyzer
	az *analyzedZettel
}

func (za *zettelAnalyzer) collect(m *meta.Meta, blocks *sx.Pair) {
	za.collectMeta(m)
	zsx.WalkIt(za, blocks, nil)
}

func (za *zettelAnalyzer) collectMeta(m *meta.Meta) {
	a, az := za.a, za.az
	az.meta = makeStoreMeta(m.Count())
	for k, v := range m.All() {
		keyID := a.dict.AddString(k)
		cv := strings.Clone(string(v))
		az.meta.add(keyID, meta.Value(cv))
		az.words.Insert(keyID)

		kd := meta.GetDescription(k)
		if kd.IsComputed() { // IsComputed() ==> IsProperty()
			continue
		}
		switch kd.Type {
		case meta.TypeID:
			za.addMetaZid(keyID, zeroiter.OneSeq(string(v)))
		case meta.TypeIDSet:
			za.addMetaZid(keyID, v.Fields())
		case meta.TypeURL:
			za.addClonedURL(cv)
		default:
			if kd.Type.IsSet {
				for s := range v.Fields() {
					za.addMetaValue(meta.Value(s))
				}
			} else {
				za.addMetaValue(v)
			}
		}
	}
	az.meta.sort()
	slices.SortFunc(az.metaZids, func(a, b metaZid) int { return cmp.Compare(a.keyID, b.keyID) })
}

func (za *zettelAnalyzer) addMetaZid(keyID vocab.WordID, vals iter.Seq[string]) {
	mz := metaZid{keyID: keyID}
	for val := range vals {
		if zid, err := id.Parse(val); err == nil {
			dzid := za.a.dict.Encode(zid)
			mz.forward.Insert(dzid)
		}
	}
	if !mz.forward.IsEmpty() {
		za.az.metaZids = append(za.az.metaZids, mz)
	}
}

func (za *zettelAnalyzer) addMetaValue(val meta.Value) {
	hasWord := false
	for word := range zerostrings.NormalizeWordSeq(string(val)) {
		if len(word) <= vocab.MaxWordLen {
			za.az.words.Insert(za.a.dict.AddBytes(word))
			hasWord = true
		}
	}
	if !hasWord && len(string(val)) <= vocab.MaxWordLen && val != "" {
		za.az.words.Insert(za.a.dict.AddString(string(val)))
	}
}
func (za *zettelAnalyzer) addString(s string) {
	for word := range zerostrings.NormalizeWordSeq(s) {
		if len(word) <= vocab.MaxWordLen {
			za.az.words.Insert(za.a.dict.AddBytes(word))
		}
	}
}
func (za *zettelAnalyzer) addURL(s string) {
	if _, err := url.Parse(s); err == nil {
		za.az.urls.Insert(strings.Clone(s))
	}
}
func (za *zettelAnalyzer) addClonedURL(s string) {
	if _, err := url.Parse(s); err == nil {
		za.az.urls.Insert(s)
	}
}

func (za *zettelAnalyzer) VisitItBefore(node *sx.Pair, _ *sx.Pair) bool {
	switch sym := zsx.NodeSymbol(node); sym {
	case zsx.SymText:
		za.addString(zsx.GetText(node))
	case zsx.SymVerbatimCode, zsx.SymVerbatimComment, zsx.SymVerbatimEval,
		zsx.SymVerbatimHTML, zsx.SymVerbatimMath, zsx.SymVerbatimZettel:
		_, _, s := zsx.GetVerbatim(node)
		za.addString(s)
	case zsx.SymLiteralCode, zsx.SymLiteralComment, zsx.SymLiteralInput,
		zsx.SymLiteralMath, zsx.SymLiteralOutput:
		_, _, s := zsx.GetLiteral(node)
		za.addString(s)
	case zsx.SymLink:
		_, ref, _ := zsx.GetLink(node)
		za.addRef(ref)
	case zsx.SymEmbed:
		_, ref, _, _ := zsx.GetEmbed(node)
		za.addRef(ref)
	case zsx.SymTransclude:
		_, ref, _ := zsx.GetTransclusion(node)
		za.addRef(ref)
	case zsx.SymCite:
		_, key, _ := zsx.GetCite(node)
		za.addString(key)
	}
	return false
}
func (za *zettelAnalyzer) VisitItAfter(*sx.Pair, *sx.Pair) {}
func (za *zettelAnalyzer) addRef(ref *sx.Pair) {
	sym, refValue := zsx.GetReference(ref)
	if zsx.SymRefStateExternal.IsEqual(sym) {
		za.addURL(refValue)
	} else if sz.SymRefStateZettel.IsEqual(sym) {
		if zid, err := id.Parse(refValue); err == nil {
			za.az.forward.Insert(za.a.dict.Encode(zid))
		}
	}
}
