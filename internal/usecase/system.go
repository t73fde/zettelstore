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

package usecase

import (
	"context"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/webapi"
	"zettelstore.de/z/internal/config"
)

// ----- Refresh all zettel ----------

// IsReadyPort is the interface used by this use case.
type IsReadyPort interface {
	IsReady() bool
}

// IsReady is the data for this use case.
type IsReady struct {
	port IsReadyPort
}

// NewIsReady creates a new use case.
func NewIsReady(port IsReadyPort) IsReady {
	return IsReady{port: port}
}

// Run executes the use case.
func (uc IsReady) Run() bool {
	return uc.port.IsReady()
}

// ----- Site Info -----------------------------------------------------------

// SiteInfoPort is the interface used by this use case.
type SiteInfoPort interface {
	Get(context.Context, *meta.Meta, string) string
	SiteName() string
}

// SiteInfo is the data for this use case.
type SiteInfo struct {
	port SiteInfoPort
}

// NewSiteInfo creates a new use case.
func NewSiteInfo(port SiteInfoPort) SiteInfo {
	return SiteInfo{port: port}
}

// Run executes the use case.
func (uc SiteInfo) Run(ctx context.Context) (result webapi.SiteInfo, err error) {
	s := uc.getConfig(ctx, config.KeyHomeZettel)
	zid, err := id.Parse(s)
	if err != nil {
		return result, err
	}
	result.SiteName = uc.port.SiteName()
	result.DefaultLang = uc.getConfig(context.Background(), meta.KeyLang)
	result.HomeZettel = zid
	result.Lang = uc.getConfig(ctx, meta.KeyLang)
	return result, nil
}
func (uc SiteInfo) getConfig(ctx context.Context, key string) string {
	return uc.port.Get(ctx, nil, key)
}
