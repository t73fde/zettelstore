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

package usecase

import (
	"context"
	"time"

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/config"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// CreateZettelPort is the interface used by this use case.
type CreateZettelPort interface {
	// CreateZettel creates a new zettel.
	CreateZettel(ctx context.Context, zettel zettel.Zettel) (id.Zid, error)
}

// CreateZettel is the data for this use case.
type CreateZettel struct {
	log      *logger.Logger
	rtConfig config.Config
	port     CreateZettelPort
}

// NewCreateZettel creates a new use case.
func NewCreateZettel(log *logger.Logger, rtConfig config.Config, port CreateZettelPort) CreateZettel {
	return CreateZettel{
		log:      log,
		rtConfig: rtConfig,
		port:     port,
	}
}

// PrepareCopy the zettel for further modification.
func (*CreateZettel) PrepareCopy(origZettel zettel.Zettel) zettel.Zettel {
	origMeta := origZettel.Meta
	m := origMeta.Clone()
	if title, found := origMeta.Get(api.KeyTitle); found {
		m.Set(api.KeyTitle, prependTitle(title, "Copy", "Copy of "))
	}
	setReadonly(m)
	content := origZettel.Content
	content.TrimSpace()
	return zettel.Zettel{Meta: m, Content: content}
}

// PrepareVersion the zettel for further modification.
func (*CreateZettel) PrepareVersion(origZettel zettel.Zettel) zettel.Zettel {
	origMeta := origZettel.Meta
	m := origMeta.Clone()
	m.Set(api.KeyPredecessor, origMeta.Zid.String())
	setReadonly(m)
	content := origZettel.Content
	content.TrimSpace()
	return zettel.Zettel{Meta: m, Content: content}
}

// PrepareFolge the zettel for further modification.
func (*CreateZettel) PrepareFolge(origZettel zettel.Zettel) zettel.Zettel {
	origMeta := origZettel.Meta
	m := meta.New(id.Invalid)
	if title, found := origMeta.Get(api.KeyTitle); found {
		m.Set(api.KeyTitle, prependTitle(title, "Folge", "Folge of "))
	}
	updateMetaRoleTagsSyntax(m, origMeta)
	m.Set(api.KeyPrecursor, origMeta.Zid.String())
	return zettel.Zettel{Meta: m, Content: zettel.NewContent(nil)}
}

// PrepareSequel the zettel for further modification.
func (*CreateZettel) PrepareSequel(origZettel zettel.Zettel) zettel.Zettel {
	origMeta := origZettel.Meta
	m := meta.New(id.Invalid)
	if title, found := origMeta.Get(api.KeyTitle); found {
		m.Set(api.KeyTitle, prependTitle(title, "Sequel", "Sequel of "))
	}
	updateMetaRoleTagsSyntax(m, origMeta)
	m.Set(api.KeyPrequel, origMeta.Zid.String())
	return zettel.Zettel{Meta: m, Content: zettel.NewContent(nil)}
}

// PrepareNew the zettel for further modification.
func (*CreateZettel) PrepareNew(origZettel zettel.Zettel, newTitle string) zettel.Zettel {
	m := meta.New(id.Invalid)
	om := origZettel.Meta
	m.SetNonEmpty(api.KeyTitle, om.GetDefault(api.KeyTitle, ""))
	updateMetaRoleTagsSyntax(m, om)

	const prefixLen = len(meta.NewPrefix)
	for _, pair := range om.PairsRest() {
		if key := pair.Key; len(key) > prefixLen && key[0:prefixLen] == meta.NewPrefix {
			m.Set(key[prefixLen:], pair.Value)
		}
	}
	if newTitle != "" {
		m.Set(api.KeyTitle, newTitle)
	}
	content := origZettel.Content
	content.TrimSpace()
	return zettel.Zettel{Meta: m, Content: content}
}

func updateMetaRoleTagsSyntax(m, orig *meta.Meta) {
	m.SetNonEmpty(api.KeyRole, orig.GetDefault(api.KeyRole, ""))
	m.SetNonEmpty(api.KeyTags, orig.GetDefault(api.KeyTags, ""))
	m.SetNonEmpty(api.KeySyntax, orig.GetDefault(api.KeySyntax, meta.DefaultSyntax))
}

func prependTitle(title, s0, s1 string) string {
	if len(title) > 0 {
		return s1 + title
	}
	return s0
}

func setReadonly(m *meta.Meta) {
	if _, found := m.Get(api.KeyReadOnly); found {
		// Currently, "false" is a safe value.
		//
		// If the current user and its role is known, a more elaborative calculation
		// could be done: set it to a value, so that the current user will be able
		// to modify it later.
		m.Set(api.KeyReadOnly, api.ValueFalse)
	}
}

// Run executes the use case.
func (uc *CreateZettel) Run(ctx context.Context, zettel zettel.Zettel) (id.Zid, error) {
	m := zettel.Meta
	if m.Zid.IsValid() {
		return m.Zid, nil // TODO: new error: already exists
	}

	m.Set(api.KeyCreated, time.Now().Local().Format(id.TimestampLayout))
	m.Delete(api.KeyModified)
	m.YamlSep = uc.rtConfig.GetYAMLHeader()

	zettel.Content.TrimSpace()
	zid, err := uc.port.CreateZettel(ctx, zettel)
	uc.log.Info().User(ctx).Zid(zid).Err(err).Msg("Create zettel")
	return zid, err
}
