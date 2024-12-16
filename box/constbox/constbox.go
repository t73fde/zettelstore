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

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/box"
	"zettelstore.de/z/box/manager"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
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
	id.ConfigurationZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Runtime Configuration",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxNone,
			api.KeyCreated:    "20200804111624",
			api.KeyVisibility: api.ValueVisibilityOwner,
		},
		zettel.NewContent(nil)},
	id.MustParse(api.ZidLicense): {
		constHeader{
			api.KeyTitle:      "Zettelstore License",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxText,
			api.KeyCreated:    "20210504135842",
			api.KeyLang:       api.ValueLangEN,
			api.KeyModified:   "20220131153422",
			api.KeyReadOnly:   api.ValueTrue,
			api.KeyVisibility: api.ValueVisibilityPublic,
		},
		zettel.NewContent(contentLicense)},
	id.MustParse(api.ZidAuthors): {
		constHeader{
			api.KeyTitle:      "Zettelstore Contributors",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxZmk,
			api.KeyCreated:    "20210504135842",
			api.KeyLang:       api.ValueLangEN,
			api.KeyReadOnly:   api.ValueTrue,
			api.KeyVisibility: api.ValueVisibilityLogin,
		},
		zettel.NewContent(contentContributors)},
	id.MustParse(api.ZidDependencies): {
		constHeader{
			api.KeyTitle:      "Zettelstore Dependencies",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxZmk,
			api.KeyLang:       api.ValueLangEN,
			api.KeyReadOnly:   api.ValueTrue,
			api.KeyVisibility: api.ValueVisibilityPublic,
			api.KeyCreated:    "20210504135842",
			api.KeyModified:   "20240418095500",
		},
		zettel.NewContent(contentDependencies)},
	id.BaseTemplateZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Base HTML Template",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20230510155100",
			api.KeyModified:   "20240219145300",
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentBaseSxn)},
	id.LoginTemplateZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Login Form HTML Template",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20200804111624",
			api.KeyModified:   "20240219145200",
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentLoginSxn)},
	id.ZettelTemplateZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Zettel HTML Template",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20230510155300",
			api.KeyModified:   "20241127170400",
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentZettelSxn)},
	id.InfoTemplateZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Info HTML Template",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20200804111624",
			api.KeyModified:   "20241127170500",
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentInfoSxn)},
	id.FormTemplateZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Form HTML Template",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20200804111624",
			api.KeyModified:   "20240219145200",
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentFormSxn)},
	id.DeleteTemplateZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Delete HTML Template",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20200804111624",
			api.KeyModified:   "20241127170530",
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentDeleteSxn)},
	id.ListTemplateZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore List Zettel HTML Template",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20230704122100",
			api.KeyModified:   "20240219145200",
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentListZettelSxn)},
	id.ErrorTemplateZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Error HTML Template",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20210305133215",
			api.KeyModified:   "20240219145200",
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentErrorSxn)},
	id.StartSxnZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Sxn Start Code",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20230824160700",
			api.KeyModified:   "20240219145200",
			api.KeyVisibility: api.ValueVisibilityExpert,
			api.KeyPrecursor:  string(api.ZidSxnBase),
		},
		zettel.NewContent(contentStartCodeSxn)},
	id.BaseSxnZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Sxn Base Code",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20230619132800",
			api.KeyModified:   "20241118173500",
			api.KeyReadOnly:   api.ValueTrue,
			api.KeyVisibility: api.ValueVisibilityExpert,
			api.KeyPrecursor:  string(api.ZidSxnPrelude),
		},
		zettel.NewContent(contentBaseCodeSxn)},
	id.PreludeSxnZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Sxn Prelude",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxSxn,
			api.KeyCreated:    "20231006181700",
			api.KeyModified:   "20240222121200",
			api.KeyReadOnly:   api.ValueTrue,
			api.KeyVisibility: api.ValueVisibilityExpert,
		},
		zettel.NewContent(contentPreludeSxn)},
	id.MustParse(api.ZidBaseCSS): {
		constHeader{
			api.KeyTitle:      "Zettelstore Base CSS",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxCSS,
			api.KeyCreated:    "20200804111624",
			api.KeyModified:   "20240827143500",
			api.KeyVisibility: api.ValueVisibilityPublic,
		},
		zettel.NewContent(contentBaseCSS)},
	id.MustParse(api.ZidUserCSS): {
		constHeader{
			api.KeyTitle:      "Zettelstore User CSS",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxCSS,
			api.KeyCreated:    "20210622110143",
			api.KeyVisibility: api.ValueVisibilityPublic,
		},
		zettel.NewContent([]byte("/* User-defined CSS */"))},
	id.EmojiZid: {
		constHeader{
			api.KeyTitle:      "Zettelstore Generic Emoji",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxGif,
			api.KeyReadOnly:   api.ValueTrue,
			api.KeyCreated:    "20210504175807",
			api.KeyVisibility: api.ValueVisibilityPublic,
		},
		zettel.NewContent(contentEmoji)},
	id.TOCNewTemplateZid: {
		constHeader{
			api.KeyTitle:      "New Menu",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxZmk,
			api.KeyLang:       api.ValueLangEN,
			api.KeyCreated:    "20210217161829",
			api.KeyModified:   "20231129111800",
			api.KeyVisibility: api.ValueVisibilityCreator,
		},
		zettel.NewContent(contentNewTOCZettel)},
	id.MustParse(api.ZidTemplateNewZettel): {
		constHeader{
			api.KeyTitle:                 "New Zettel",
			api.KeyRole:                  api.ValueRoleConfiguration,
			api.KeySyntax:                meta.SyntaxZmk,
			api.KeyCreated:               "20201028185209",
			api.KeyModified:              "20230929132900",
			meta.NewPrefix + api.KeyRole: api.ValueRoleZettel,
			api.KeyVisibility:            api.ValueVisibilityCreator,
		},
		zettel.NewContent(nil)},
	id.MustParse(api.ZidTemplateNewRole): {
		constHeader{
			api.KeyTitle:                  "New Role",
			api.KeyRole:                   api.ValueRoleConfiguration,
			api.KeySyntax:                 meta.SyntaxZmk,
			api.KeyCreated:                "20231129110800",
			meta.NewPrefix + api.KeyRole:  api.ValueRoleRole,
			meta.NewPrefix + api.KeyTitle: "",
			api.KeyVisibility:             api.ValueVisibilityCreator,
		},
		zettel.NewContent(nil)},
	id.MustParse(api.ZidTemplateNewTag): {
		constHeader{
			api.KeyTitle:                  "New Tag",
			api.KeyRole:                   api.ValueRoleConfiguration,
			api.KeySyntax:                 meta.SyntaxZmk,
			api.KeyCreated:                "20230929132400",
			meta.NewPrefix + api.KeyRole:  api.ValueRoleTag,
			meta.NewPrefix + api.KeyTitle: "#",
			api.KeyVisibility:             api.ValueVisibilityCreator,
		},
		zettel.NewContent(nil)},
	id.MustParse(api.ZidTemplateNewUser): {
		constHeader{
			api.KeyTitle:                       "New User",
			api.KeyRole:                        api.ValueRoleConfiguration,
			api.KeySyntax:                      meta.SyntaxNone,
			api.KeyCreated:                     "20201028185209",
			meta.NewPrefix + api.KeyCredential: "",
			meta.NewPrefix + api.KeyUserID:     "",
			meta.NewPrefix + api.KeyUserRole:   api.ValueUserRoleReader,
			api.KeyVisibility:                  api.ValueVisibilityOwner,
		},
		zettel.NewContent(nil)},
	id.MustParse(api.ZidRoleZettelZettel): {
		constHeader{
			api.KeyTitle:      api.ValueRoleZettel,
			api.KeyRole:       api.ValueRoleRole,
			api.KeySyntax:     meta.SyntaxZmk,
			api.KeyCreated:    "20231129161400",
			api.KeyLang:       api.ValueLangEN,
			api.KeyVisibility: api.ValueVisibilityLogin,
		},
		zettel.NewContent(contentRoleZettel)},
	id.MustParse(api.ZidRoleConfigurationZettel): {
		constHeader{
			api.KeyTitle:      api.ValueRoleConfiguration,
			api.KeyRole:       api.ValueRoleRole,
			api.KeySyntax:     meta.SyntaxZmk,
			api.KeyCreated:    "20241213103100",
			api.KeyLang:       api.ValueLangEN,
			api.KeyVisibility: api.ValueVisibilityLogin,
		},
		zettel.NewContent(contentRoleConfiguration)},
	id.MustParse(api.ZidRoleRoleZettel): {
		constHeader{
			api.KeyTitle:      api.ValueRoleRole,
			api.KeyRole:       api.ValueRoleRole,
			api.KeySyntax:     meta.SyntaxZmk,
			api.KeyCreated:    "20231129162900",
			api.KeyLang:       api.ValueLangEN,
			api.KeyVisibility: api.ValueVisibilityLogin,
		},
		zettel.NewContent(contentRoleRole)},
	id.MustParse(api.ZidRoleTagZettel): {
		constHeader{
			api.KeyTitle:      api.ValueRoleTag,
			api.KeyRole:       api.ValueRoleRole,
			api.KeySyntax:     meta.SyntaxZmk,
			api.KeyCreated:    "20231129162000",
			api.KeyLang:       api.ValueLangEN,
			api.KeyVisibility: api.ValueVisibilityLogin,
		},
		zettel.NewContent(contentRoleTag)},
	id.MustParse(api.ZidAppDirectory): {
		constHeader{
			api.KeyTitle:      "Zettelstore Application Directory",
			api.KeyRole:       api.ValueRoleConfiguration,
			api.KeySyntax:     meta.SyntaxNone,
			api.KeyLang:       api.ValueLangEN,
			api.KeyCreated:    "20240703235900",
			api.KeyVisibility: api.ValueVisibilityLogin,
		},
		zettel.NewContent(nil)},
	id.DefaultHomeZid: {
		constHeader{
			api.KeyTitle:    "Home",
			api.KeyRole:     api.ValueRoleZettel,
			api.KeySyntax:   meta.SyntaxZmk,
			api.KeyLang:     api.ValueLangEN,
			api.KeyCreated:  "20210210190757",
			api.KeyModified: "20241216105800",
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

//go:embed newtoc.zettel
var contentNewTOCZettel []byte

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
