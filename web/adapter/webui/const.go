//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package webui

// WebUI related constants.

const queryKeyAction = "_action"

// Values for queryKeyAction
const (
	valueActionCopy    = "copy"
	valueActionFolge   = "folge"
	valueActionNew     = "new"
	valueActionSequel  = "sequel"
	valueActionVersion = "version"
)

// Enumeration for queryKeyAction
type createAction uint8

const (
	actionCopy createAction = iota
	actionFolge
	actionNew
	actionSequel
	actionVersion
)

var createActionMap = map[string]createAction{
	valueActionSequel:  actionSequel,
	valueActionCopy:    actionCopy,
	valueActionFolge:   actionFolge,
	valueActionNew:     actionNew,
	valueActionVersion: actionVersion,
}

func getCreateAction(s string) createAction {
	if action, found := createActionMap[s]; found {
		return action
	}
	return actionCopy
}
