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
	"fmt"
	"log/slog"
	"net/url"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/box/manager"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/zettel"
)

func init() {
	manager.Register(
		box.SchemeCompBox,
		func(_ *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
			return getCompBox(cdata.Enricher), nil
		})
}

type compBox struct {
	logger   *slog.Logger
	enricher box.Enricher
}

type genFns = struct {
	meta    func(id.Zid) *meta.Meta
	content func() []byte
}

var myConfig *meta.Meta
var myZettel = map[id.Zid]genFns{
	id.ZidVersion:         {genVersionBuildM, genVersionBuildC},
	id.ZidHost:            {genVersionHostM, genVersionHostC},
	id.ZidOperatingSystem: {genVersionOSM, genVersionOSC},
	id.ZidModules:         {genModulesM, genModulesC},
	id.ZidLog:             {genLogM, genLogC},
	id.ZidMemory:          {genMemoryM, genMemoryC},
	id.ZidSx:              {genSxM, genSxC},
	id.ZidKernelService:   makeSrvGen(kernel.KernelService, "Kernel"),
	id.ZidCoreService:     makeSrvGen(kernel.CoreService, "Core"),
	id.ZidConfigService:   makeSrvGen(kernel.ConfigService, "Configuration"),
	id.ZidAuthService:     makeSrvGen(kernel.AuthService, "Authentication"),
	id.ZidBoxService:      makeSrvGen(kernel.BoxService, "Box"),
	// id.ZidIndexService:  makeSrvGen(kernel.IndexService, "Index"),
	id.ZidWebService:           makeSrvGen(kernel.WebService, "Web"),
	id.ZidMetadataKey:          {genKeysM, genKeysC},
	id.ZidParser:               {genParserM, genParserC},
	id.ZidStartupConfiguration: {genConfigZettelM, genConfigZettelC},
}

func makeSrvGen(srv kernel.Service, name string) genFns {
	return genFns{
		func(zid id.Zid) *meta.Meta {
			return getTitledMeta(zid, "Zettelstore "+name+" Service")
		},
		func() []byte {
			var buf []byte
			stats := kernel.Main.GetServiceStatistics(srv)
			if len(stats) == 0 {
				return append(buf, "No service data is currently available.\n"...)
			}
			buf = append(buf, "|=Name|=Value>\n"...)
			for _, kv := range stats {
				buf = fmt.Appendf(buf, "| %v | %v\n", kv.Key, kv.Value)
			}
			return buf
		},
	}
}

// Get returns the one program box.
func getCompBox(mf box.Enricher) *compBox {
	return &compBox{
		logger:   kernel.Main.GetLogger(kernel.BoxService).With("box", box.SchemeCompBox),
		enricher: mf,
	}
}

// Setup remembers important values.
func Setup(cfg *meta.Meta) { myConfig = cfg.Clone() }

func (*compBox) Name() string     { return box.SchemeCompBox }
func (*compBox) Location() string { return box.SchemeCompBox + ":" }

func (cb *compBox) GetZettel(_ context.Context, zid id.Zid) (box.Zettel, error) {
	if gen, ok := myZettel[zid]; ok && gen.meta != nil {
		if m := gen.meta(zid); m != nil {
			updateMeta(m)
			if genContent := gen.content; genContent != nil {
				logging.LogTrace(cb.logger, "GetZettel/Content")
				return box.Zettel{
					Meta:    m,
					Content: zettel.NewContent(genContent()),
				}, nil
			}
			logging.LogTrace(cb.logger, "GetZettel/NoContent")
			return box.Zettel{Meta: m}, nil
		}
	}
	err := box.ErrZettelNotFound{Zid: zid}
	logging.LogTrace(cb.logger, "GetZettel/Err", "err", err)
	return box.Zettel{}, err
}

func (*compBox) HasZettel(_ context.Context, zid id.Zid) bool {
	_, found := myZettel[zid]
	return found
}

func (cb *compBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint box.RetrievePredicate) error {
	logging.LogTrace(cb.logger, "ApplyZid", "entries", len(myZettel))
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

func (cb *compBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint box.RetrievePredicate) error {
	logging.LogTrace(cb.logger, "ApplyMeta", "entries", len(myZettel))
	for zid, gen := range myZettel {
		if !constraint(zid) {
			continue
		}
		if genMeta := gen.meta; genMeta != nil {
			if m := genMeta(zid); m != nil {
				updateMeta(m)
				cb.enricher.Enrich(ctx, m, box.SchemeCompBox)
				handle(m)
			}
		}
	}
	return nil
}

func (cb *compBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = true
	st.Zettel = len(myZettel)
	logging.LogTrace(cb.logger, "ReadStats", "zettel", st.Zettel)
}

func getTitledMeta(zid id.Zid, title string) *meta.Meta {
	m := meta.New(zid)
	m.Set(meta.KeyTitle, meta.Value(title))
	return m
}

func updateMeta(m *meta.Meta) {
	if _, ok := m.Get(meta.KeySyntax); !ok {
		m.Set(meta.KeySyntax, meta.ValueSyntaxZmk)
	}
	m.Set(meta.KeyRole, meta.ValueRoleConfiguration)
	if _, ok := m.Get(meta.KeyCreated); !ok {
		m.Set(meta.KeyCreated, meta.Value(kernel.Main.GetConfig(kernel.CoreService, kernel.CoreStarted).(string)))
	}
	m.Set(meta.KeyLang, meta.ValueLangEN)
	m.Set(meta.KeyReadOnly, meta.ValueTrue)
	if _, ok := m.Get(meta.KeyVisibility); !ok {
		m.Set(meta.KeyVisibility, meta.ValueVisibilityExpert)
	}
}
