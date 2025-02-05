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

// Package compbox provides zettel that have computed content.
package compbox

import (
	"context"
	"net/url"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/box"
	"zettelstore.de/z/box/manager"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel"
)

func init() {
	manager.Register(
		" comp",
		func(_ *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
			return getCompBox(cdata.Number, cdata.Enricher), nil
		})
}

type compBox struct {
	log      *logger.Logger
	number   int
	enricher box.Enricher
}

var myConfig *meta.Meta
var myZettel = map[id.Zid]struct {
	meta    func(id.Zid) *meta.Meta
	content func(context.Context, *compBox) []byte
}{
	api.ZidVersion:         {genVersionBuildM, genVersionBuildC},
	api.ZidHost:            {genVersionHostM, genVersionHostC},
	api.ZidOperatingSystem: {genVersionOSM, genVersionOSC},
	api.ZidLog:             {genLogM, genLogC},
	api.ZidMemory:          {genMemoryM, genMemoryC},
	api.ZidSx:              {genSxM, genSxC},
	// api.ZidHTTP:                 {genHttpM, genHttpC},
	// api.ZidAPI:                  {genApiM, genApiC},
	// api.ZidWebUI:                {genWebUiM, genWebUiC},
	// api.ZidConsole:              {genConsoleM, genConsoleC},
	api.ZidBoxManager: {genManagerM, genManagerC},
	// api.ZidIndex:                {genIndexM, genIndexC},
	// api.ZidQuery:                {genQueryM, genQueryC},
	api.ZidMetadataKey:          {genKeysM, genKeysC},
	api.ZidParser:               {genParserM, genParserC},
	api.ZidStartupConfiguration: {genConfigZettelM, genConfigZettelC},
}

// Get returns the one program box.
func getCompBox(boxNumber int, mf box.Enricher) *compBox {
	return &compBox{
		log: kernel.Main.GetLogger(kernel.BoxService).Clone().
			Str("box", "comp").Int("boxnum", int64(boxNumber)).Child(),
		number:   boxNumber,
		enricher: mf,
	}
}

// Setup remembers important values.
func Setup(cfg *meta.Meta) { myConfig = cfg.Clone() }

func (*compBox) Location() string { return "" }

func (cb *compBox) GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error) {
	if gen, ok := myZettel[zid]; ok && gen.meta != nil {
		if m := gen.meta(zid); m != nil {
			updateMeta(m)
			if genContent := gen.content; genContent != nil {
				cb.log.Trace().Msg("GetZettel/Content")
				return zettel.Zettel{
					Meta:    m,
					Content: zettel.NewContent(genContent(ctx, cb)),
				}, nil
			}
			cb.log.Trace().Msg("GetZettel/NoContent")
			return zettel.Zettel{Meta: m}, nil
		}
	}
	err := box.ErrZettelNotFound{Zid: zid}
	cb.log.Trace().Err(err).Msg("GetZettel/Err")
	return zettel.Zettel{}, err
}

func (*compBox) HasZettel(_ context.Context, zid id.Zid) bool {
	_, found := myZettel[zid]
	return found
}

func (cb *compBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint query.RetrievePredicate) error {
	cb.log.Trace().Int("entries", int64(len(myZettel))).Msg("ApplyZid")
	for zid, gen := range myZettel {
		if !constraint(zid) {
			continue
		}
		if genMeta := gen.meta; genMeta != nil {
			if genMeta(zid) != nil {
				handle(zid)
			}
		}
	}
	return nil
}

func (cb *compBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint query.RetrievePredicate) error {
	cb.log.Trace().Int("entries", int64(len(myZettel))).Msg("ApplyMeta")
	for zid, gen := range myZettel {
		if !constraint(zid) {
			continue
		}
		if genMeta := gen.meta; genMeta != nil {
			if m := genMeta(zid); m != nil {
				updateMeta(m)
				cb.enricher.Enrich(ctx, m, cb.number)
				handle(m)
			}
		}
	}
	return nil
}

func (*compBox) CanDeleteZettel(context.Context, id.Zid) bool { return false }

func (cb *compBox) DeleteZettel(_ context.Context, zid id.Zid) (err error) {
	if _, ok := myZettel[zid]; ok {
		err = box.ErrReadOnly
	} else {
		err = box.ErrZettelNotFound{Zid: zid}
	}
	cb.log.Trace().Err(err).Msg("DeleteZettel")
	return err
}

func (cb *compBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = true
	st.Zettel = len(myZettel)
	cb.log.Trace().Int("zettel", int64(st.Zettel)).Msg("ReadStats")
}

func getTitledMeta(zid id.Zid, title string) *meta.Meta {
	m := meta.New(zid)
	m.Set(api.KeyTitle, meta.Value(title))
	return m
}

func updateMeta(m *meta.Meta) {
	if _, ok := m.Get(api.KeySyntax); !ok {
		m.Set(api.KeySyntax, meta.SyntaxZmk)
	}
	m.Set(api.KeyRole, api.ValueRoleConfiguration)
	if _, ok := m.Get(api.KeyCreated); !ok {
		m.Set(api.KeyCreated, meta.Value(kernel.Main.GetConfig(kernel.CoreService, kernel.CoreStarted).(string)))
	}
	m.Set(api.KeyLang, api.ValueLangEN)
	m.Set(api.KeyReadOnly, api.ValueTrue)
	if _, ok := m.Get(api.KeyVisibility); !ok {
		m.Set(api.KeyVisibility, api.ValueVisibilityExpert)
	}
}
