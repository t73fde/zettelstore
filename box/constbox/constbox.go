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

// Package constbox puts zettel inside the executable.
package constbox

import (
	"context"
	_ "embed" // Allow to embed file content
	"net/url"

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
		" const",
		func(_ *url.URL, cdata *manager.ConnectData) (box.ManagedBox, error) {
			return &constBox{
				log: kernel.Main.GetLogger(kernel.BoxService).Clone().
					Str("box", "const").Int("boxnum", int64(cdata.Number)).Child(),
				number:   cdata.Number,
				zettel:   constZettelMap,
				enricher: cdata.Enricher,
			}, nil
		})
}

type constHeader map[string]string

type constZettel struct {
	header  constHeader
	content zettel.Content
}

type constBox struct {
	log      *logger.Logger
	number   int
	zettel   map[id.Zid]constZettel
	enricher box.Enricher
}

func (*constBox) Location() string { return "const:" }

func (cb *constBox) GetZettel(_ context.Context, zid id.Zid) (zettel.Zettel, error) {
	if z, ok := cb.zettel[zid]; ok {
		cb.log.Trace().Msg("GetZettel")
		return zettel.Zettel{Meta: meta.NewWithData(zid, z.header), Content: z.content}, nil
	}
	err := box.ErrZettelNotFound{Zid: zid}
	cb.log.Trace().Err(err).Msg("GetZettel/Err")
	return zettel.Zettel{}, err
}

func (cb *constBox) HasZettel(_ context.Context, zid id.Zid) bool {
	_, found := cb.zettel[zid]
	return found
}

func (cb *constBox) ApplyZid(_ context.Context, handle box.ZidFunc, constraint query.RetrievePredicate) error {
	cb.log.Trace().Int("entries", int64(len(cb.zettel))).Msg("ApplyZid")
	for zid := range cb.zettel {
		if constraint(zid) {
			handle(zid)
		}
	}
	return nil
}

func (cb *constBox) ApplyMeta(ctx context.Context, handle box.MetaFunc, constraint query.RetrievePredicate) error {
	cb.log.Trace().Int("entries", int64(len(cb.zettel))).Msg("ApplyMeta")
	for zid, zettel := range cb.zettel {
		if constraint(zid) {
			m := meta.NewWithData(zid, zettel.header)
			cb.enricher.Enrich(ctx, m, cb.number)
			handle(m)
		}
	}
	return nil
}

func (*constBox) CanDeleteZettel(context.Context, id.Zid) bool { return false }

func (cb *constBox) DeleteZettel(_ context.Context, zid id.Zid) (err error) {
	if _, ok := cb.zettel[zid]; ok {
		err = box.ErrReadOnly
	} else {
		err = box.ErrZettelNotFound{Zid: zid}
	}
	cb.log.Trace().Err(err).Msg("DeleteZettel")
	return err
}

func (cb *constBox) ReadStats(st *box.ManagedBoxStats) {
	st.ReadOnly = true
	st.Zettel = len(cb.zettel)
	cb.log.Trace().Int("zettel", int64(st.Zettel)).Msg("ReadStats")
}

var constZettelMap = map[id.Zid]constZettel{
	id.ZidConfiguration: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Runtime Configuration",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxNone,
			meta.KeyCreated:    "20200804111624",
			meta.KeyVisibility: meta.ValueVisibilityOwner,
		},
		zettel.NewContent(nil)},
	id.ZidLicense: {
		constHeader{
			meta.KeyTitle:      "Zettelstore License",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxText,
			meta.KeyCreated:    "20210504135842",
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyModified:   "20220131153422",
			meta.KeyReadOnly:   meta.ValueTrue,
			meta.KeyVisibility: meta.ValueVisibilityPublic,
		},
		zettel.NewContent(contentLicense)},
	id.ZidAuthors: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Contributors",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxZmk,
			meta.KeyCreated:    "20210504135842",
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyReadOnly:   meta.ValueTrue,
			meta.KeyVisibility: meta.ValueVisibilityLogin,
		},
		zettel.NewContent(contentContributors)},
	id.ZidDependencies: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Dependencies",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxZmk,
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyReadOnly:   meta.ValueTrue,
			meta.KeyVisibility: meta.ValueVisibilityPublic,
			meta.KeyCreated:    "20210504135842",
			meta.KeyModified:   "20240418095500",
		},
		zettel.NewContent(contentDependencies)},
	id.ZidBaseTemplate: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Base HTML Template",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20230510155100",
			meta.KeyModified:   "20241227212000",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentBaseSxn)},
	id.ZidLoginTemplate: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Login Form HTML Template",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20200804111624",
			meta.KeyModified:   "20240219145200",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentLoginSxn)},
	id.ZidZettelTemplate: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Zettel HTML Template",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20230510155300",
			meta.KeyModified:   "20241130205700",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentZettelSxn)},
	id.ZidInfoTemplate: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Info HTML Template",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20200804111624",
			meta.KeyModified:   "20241127170500",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentInfoSxn)},
	id.ZidFormTemplate: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Form HTML Template",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20200804111624",
			meta.KeyModified:   "20240219145200",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentFormSxn)},
	id.ZidDeleteTemplate: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Delete HTML Template",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20200804111624",
			meta.KeyModified:   "20241127170530",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentDeleteSxn)},
	id.ZidListTemplate: {
		constHeader{
			meta.KeyTitle:      "Zettelstore List Zettel HTML Template",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20230704122100",
			meta.KeyModified:   "20240219145200",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentListZettelSxn)},
	id.ZidErrorTemplate: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Error HTML Template",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20210305133215",
			meta.KeyModified:   "20240219145200",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentErrorSxn)},
	id.ZidSxnStart: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Sxn Start Code",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20230824160700",
			meta.KeyModified:   "20240219145200",
			meta.KeyVisibility: meta.ValueVisibilityExpert,
			meta.KeyPrecursor:  id.ZidSxnBase.String(),
		},
		zettel.NewContent(contentStartCodeSxn)},
	id.ZidSxnBase: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Sxn Base Code",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20230619132800",
			meta.KeyModified:   "20241118173500",
			meta.KeyReadOnly:   meta.ValueTrue,
			meta.KeyVisibility: meta.ValueVisibilityExpert,
			meta.KeyPrecursor:  id.ZidSxnPrelude.String(),
		},
		zettel.NewContent(contentBaseCodeSxn)},
	id.ZidSxnPrelude: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Sxn Prelude",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxSxn,
			meta.KeyCreated:    "20231006181700",
			meta.KeyModified:   "20240222121200",
			meta.KeyReadOnly:   meta.ValueTrue,
			meta.KeyVisibility: meta.ValueVisibilityExpert,
		},
		zettel.NewContent(contentPreludeSxn)},
	id.ZidBaseCSS: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Base CSS",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxCSS,
			meta.KeyCreated:    "20200804111624",
			meta.KeyModified:   "20240827143500",
			meta.KeyVisibility: meta.ValueVisibilityPublic,
		},
		zettel.NewContent(contentBaseCSS)},
	id.ZidUserCSS: {
		constHeader{
			meta.KeyTitle:      "Zettelstore User CSS",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxCSS,
			meta.KeyCreated:    "20210622110143",
			meta.KeyVisibility: meta.ValueVisibilityPublic,
		},
		zettel.NewContent([]byte("/* User-defined CSS */"))},
	id.ZidEmoji: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Generic Emoji",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxGif,
			meta.KeyReadOnly:   meta.ValueTrue,
			meta.KeyCreated:    "20210504175807",
			meta.KeyVisibility: meta.ValueVisibilityPublic,
		},
		zettel.NewContent(contentEmoji)},
	id.ZidTOCListsMenu: {
		constHeader{
			meta.KeyTitle:      "Lists Menu",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxZmk,
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyCreated:    "20241223205400",
			meta.KeyVisibility: meta.ValueVisibilityPublic,
		},
		zettel.NewContent(contentMenuListsZettel)},
	id.ZidTOCNewTemplate: {
		constHeader{
			meta.KeyTitle:      "New Menu",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxZmk,
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyCreated:    "20210217161829",
			meta.KeyModified:   "20231129111800",
			meta.KeyVisibility: meta.ValueVisibilityCreator,
		},
		zettel.NewContent(contentMenuNewZettel)},
	id.ZidTemplateNewZettel: {
		constHeader{
			meta.KeyTitle:                 "New Zettel",
			meta.KeyRole:                  meta.ValueRoleConfiguration,
			meta.KeySyntax:                meta.ValueSyntaxZmk,
			meta.KeyCreated:               "20201028185209",
			meta.KeyModified:              "20230929132900",
			meta.NewPrefix + meta.KeyRole: meta.ValueRoleZettel,
			meta.KeyVisibility:            meta.ValueVisibilityCreator,
		},
		zettel.NewContent(nil)},
	id.ZidTemplateNewRole: {
		constHeader{
			meta.KeyTitle:                  "New Role",
			meta.KeyRole:                   meta.ValueRoleConfiguration,
			meta.KeySyntax:                 meta.ValueSyntaxZmk,
			meta.KeyCreated:                "20231129110800",
			meta.NewPrefix + meta.KeyRole:  meta.ValueRoleRole,
			meta.NewPrefix + meta.KeyTitle: "",
			meta.KeyVisibility:             meta.ValueVisibilityCreator,
		},
		zettel.NewContent(nil)},
	id.ZidTemplateNewTag: {
		constHeader{
			meta.KeyTitle:                  "New Tag",
			meta.KeyRole:                   meta.ValueRoleConfiguration,
			meta.KeySyntax:                 meta.ValueSyntaxZmk,
			meta.KeyCreated:                "20230929132400",
			meta.NewPrefix + meta.KeyRole:  meta.ValueRoleTag,
			meta.NewPrefix + meta.KeyTitle: "#",
			meta.KeyVisibility:             meta.ValueVisibilityCreator,
		},
		zettel.NewContent(nil)},
	id.ZidTemplateNewUser: {
		constHeader{
			meta.KeyTitle:                       "New User",
			meta.KeyRole:                        meta.ValueRoleConfiguration,
			meta.KeySyntax:                      meta.ValueSyntaxNone,
			meta.KeyCreated:                     "20201028185209",
			meta.NewPrefix + meta.KeyCredential: "",
			meta.NewPrefix + meta.KeyUserID:     "",
			meta.NewPrefix + meta.KeyUserRole:   meta.ValueUserRoleReader,
			meta.KeyVisibility:                  meta.ValueVisibilityOwner,
		},
		zettel.NewContent(nil)},
	id.ZidRoleZettelZettel: {
		constHeader{
			meta.KeyTitle:      meta.ValueRoleZettel,
			meta.KeyRole:       meta.ValueRoleRole,
			meta.KeySyntax:     meta.ValueSyntaxZmk,
			meta.KeyCreated:    "20231129161400",
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyVisibility: meta.ValueVisibilityLogin,
		},
		zettel.NewContent(contentRoleZettel)},
	id.ZidRoleConfigurationZettel: {
		constHeader{
			meta.KeyTitle:      meta.ValueRoleConfiguration,
			meta.KeyRole:       meta.ValueRoleRole,
			meta.KeySyntax:     meta.ValueSyntaxZmk,
			meta.KeyCreated:    "20241213103100",
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyVisibility: meta.ValueVisibilityLogin,
		},
		zettel.NewContent(contentRoleConfiguration)},
	id.ZidRoleRoleZettel: {
		constHeader{
			meta.KeyTitle:      meta.ValueRoleRole,
			meta.KeyRole:       meta.ValueRoleRole,
			meta.KeySyntax:     meta.ValueSyntaxZmk,
			meta.KeyCreated:    "20231129162900",
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyVisibility: meta.ValueVisibilityLogin,
		},
		zettel.NewContent(contentRoleRole)},
	id.ZidRoleTagZettel: {
		constHeader{
			meta.KeyTitle:      meta.ValueRoleTag,
			meta.KeyRole:       meta.ValueRoleRole,
			meta.KeySyntax:     meta.ValueSyntaxZmk,
			meta.KeyCreated:    "20231129162000",
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyVisibility: meta.ValueVisibilityLogin,
		},
		zettel.NewContent(contentRoleTag)},
	id.ZidAppDirectory: {
		constHeader{
			meta.KeyTitle:      "Zettelstore Application Directory",
			meta.KeyRole:       meta.ValueRoleConfiguration,
			meta.KeySyntax:     meta.ValueSyntaxNone,
			meta.KeyLang:       meta.ValueLangEN,
			meta.KeyCreated:    "20240703235900",
			meta.KeyVisibility: meta.ValueVisibilityLogin,
		},
		zettel.NewContent(nil)},
	id.ZidDefaultHome: {
		constHeader{
			meta.KeyTitle:    "Home",
			meta.KeyRole:     meta.ValueRoleZettel,
			meta.KeySyntax:   meta.ValueSyntaxZmk,
			meta.KeyLang:     meta.ValueLangEN,
			meta.KeyCreated:  "20210210190757",
			meta.KeyModified: "20241216105800",
		},
		zettel.NewContent(contentHomeZettel)},
}

//go:embed license.txt
var contentLicense []byte

//go:embed contributors.zettel
var contentContributors []byte

//go:embed dependencies.zettel
var contentDependencies []byte

//go:embed base.sxn
var contentBaseSxn []byte

//go:embed login.sxn
var contentLoginSxn []byte

//go:embed zettel.sxn
var contentZettelSxn []byte

//go:embed info.sxn
var contentInfoSxn []byte

//go:embed form.sxn
var contentFormSxn []byte

//go:embed delete.sxn
var contentDeleteSxn []byte

//go:embed listzettel.sxn
var contentListZettelSxn []byte

//go:embed error.sxn
var contentErrorSxn []byte

//go:embed start.sxn
var contentStartCodeSxn []byte

//go:embed wuicode.sxn
var contentBaseCodeSxn []byte

//go:embed prelude.sxn
var contentPreludeSxn []byte

//go:embed base.css
var contentBaseCSS []byte

//go:embed emoji_spin.gif
var contentEmoji []byte

//go:embed menu_lists.zettel
var contentMenuListsZettel []byte

//go:embed menu_new.zettel
var contentMenuNewZettel []byte

//go:embed rolezettel.zettel
var contentRoleZettel []byte

//go:embed roleconfiguration.zettel
var contentRoleConfiguration []byte

//go:embed rolerole.zettel
var contentRoleRole []byte

//go:embed roletag.zettel
var contentRoleTag []byte

//go:embed home.zettel
var contentHomeZettel []byte
