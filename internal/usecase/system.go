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

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
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

// ----- Metadata key info ---------------------------------------------------

// MetadataKeyInfoPort contains the interface needed by this use case.
type MetadataKeyInfoPort interface {
	GetUserRole(user *meta.Meta) meta.UserRole
}

// MetadataKeyInfo ist the use case itself.
type MetadataKeyInfo struct {
	port MetadataKeyInfoPort
}

// NewMetadataKeyInfo creates a new use case object.
func NewMetadataKeyInfo(port MetadataKeyInfoPort) MetadataKeyInfo {
	return MetadataKeyInfo{port: port}
}

// Run executes the use case
func (uc MetadataKeyInfo) Run(ctx context.Context, key string) (result webapi.MetadataKeyInfo, _ error) {
	if user := auth.GetCurrentUser(ctx); uc.port.GetUserRole(user) < meta.UserRoleReader {
		return result, box.NewErrNotAllowed("GetMetadataKeyInfo", user, id.Invalid)
	}
	dk := meta.GetDescription(key)
	return uc.getData(dk), nil
}

// RunPredefined executed the use case for the predefined keys.
func (uc MetadataKeyInfo) RunPredefined(ctx context.Context) (result []webapi.MetadataKeyInfo, _ error) {
	if user := auth.GetCurrentUser(ctx); uc.port.GetUserRole(user) < meta.UserRoleReader {
		return result, box.NewErrNotAllowed("GetMetadataKeyInfo", user, id.Invalid)
	}
	for dk := range meta.KeyDescriptionSeq() {
		result = append(result, uc.getData(dk))
	}
	return result, nil
}

func (MetadataKeyInfo) getData(dk meta.DescriptionKey) webapi.MetadataKeyInfo {
	dt := dk.Type
	return webapi.MetadataKeyInfo{
		Name:       dk.Name,
		Type:       dt.Name,
		IsSet:      dt.IsSet,
		IsComputed: dk.IsComputed(),
		IsProperty: dk.IsProperty(),
	}
}
