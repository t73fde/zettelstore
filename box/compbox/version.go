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

package compbox

import (
	"context"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/zettel/meta"
)

func genVersionBuildM(zid id.Zid) *meta.Meta {
	m := getTitledMeta(zid, "Zettelstore Version")
	m.Set(api.KeyCreated, kernel.Main.GetConfig(kernel.CoreService, kernel.CoreVTime).(string))
	m.Set(api.KeyVisibility, api.ValueVisibilityLogin)
	return m
}
func genVersionBuildC(context.Context, *compBox) []byte {
	return []byte(kernel.Main.GetConfig(kernel.CoreService, kernel.CoreVersion).(string))
}

func genVersionHostM(zid id.Zid) *meta.Meta {
	return getTitledMeta(zid, "Zettelstore Host")
}
func genVersionHostC(context.Context, *compBox) []byte {
	return []byte(kernel.Main.GetConfig(kernel.CoreService, kernel.CoreHostname).(string))
}

func genVersionOSM(zid id.Zid) *meta.Meta {
	return getTitledMeta(zid, "Zettelstore Operating System")
}
func genVersionOSC(context.Context, *compBox) []byte {
	goOS := kernel.Main.GetConfig(kernel.CoreService, kernel.CoreGoOS).(string)
	goArch := kernel.Main.GetConfig(kernel.CoreService, kernel.CoreGoArch).(string)
	result := make([]byte, 0, len(goOS)+len(goArch)+1)
	result = append(result, goOS...)
	result = append(result, '/')
	return append(result, goArch...)
}
