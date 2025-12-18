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

package policy

import (
	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/config"
)

type ownerPolicy struct {
	manager    auth.AuthzManager
	authConfig config.AuthConfig
	pre        auth.Policy
}

func (o *ownerPolicy) CanCreate(user, newMeta *meta.Meta) bool {
	if user == nil || !o.pre.CanCreate(user, newMeta) {
		return false
	}
	return o.userIsOwner(user) || o.userCanCreate(user, newMeta)
}

func (o *ownerPolicy) userCanCreate(user, newMeta *meta.Meta) bool {
	if o.manager.GetUserRole(user) == meta.UserRoleReader {
		return false
	}
	if _, ok := newMeta.Get(meta.KeyUserID); ok {
		return false
	}
	return true
}

func (o *ownerPolicy) CanRead(user, m *meta.Meta) bool {
	// No need to call o.pre.CanRead(user, meta), because it will always return true.
	// Both the default and the readonly policy allow reading a zettel.
	vis := o.authConfig.GetVisibility(m)
	if res, ok := o.checkVisibility(user, vis); ok {
		return res
	}
	return o.userIsOwner(user) || o.userCanRead(user, m, vis)
}

func (o *ownerPolicy) userCanRead(user, m *meta.Meta, vis meta.Visibility) bool {
	switch vis {
	case meta.VisibilityOwner, meta.VisibilityExpert:
		return false
	case meta.VisibilityPublic:
		return true
	}
	if user == nil {
		return false
	}
	if _, ok := m.Get(meta.KeyUserID); ok {
		// Only the user can read its own zettel
		return user.Zid == m.Zid
	}
	switch o.manager.GetUserRole(user) {
	case meta.UserRoleReader, meta.UserRoleWriter, meta.UserRoleOwner:
		return true
	case meta.UserRoleCreator:
		return vis == meta.VisibilityCreator
	default:
		return false
	}
}

var noChangeUser = []string{
	meta.KeyID,
	meta.KeyRole,
	meta.KeyUserID,
	meta.KeyUserRole,
}

func (o *ownerPolicy) CanWrite(user, oldMeta, newMeta *meta.Meta) bool {
	if user == nil || !o.pre.CanWrite(user, oldMeta, newMeta) {
		return false
	}
	vis := o.authConfig.GetVisibility(oldMeta)
	if res, ok := o.checkVisibility(user, vis); ok {
		return res
	}
	if o.userIsOwner(user) {
		return true
	}
	if !o.userCanRead(user, oldMeta, vis) {
		return false
	}
	if _, ok := oldMeta.Get(meta.KeyUserID); ok {
		// Here we know, that user.Zid == newMeta.Zid (because of userCanRead) and
		// user.Zid == newMeta.Zid (because oldMeta.Zid == newMeta.Zid)
		for _, key := range noChangeUser {
			if oldMeta.GetDefault(key, "") != newMeta.GetDefault(key, "") {
				return false
			}
		}
		return true
	}
	switch userRole := o.manager.GetUserRole(user); userRole {
	case meta.UserRoleReader, meta.UserRoleCreator:
		return false
	}
	return o.userCanCreate(user, newMeta)
}

func (o *ownerPolicy) CanDelete(user, m *meta.Meta) bool {
	if user == nil || !o.pre.CanDelete(user, m) {
		return false
	}
	if res, ok := o.checkVisibility(user, o.authConfig.GetVisibility(m)); ok {
		return res
	}
	return o.userIsOwner(user)
}

func (o *ownerPolicy) CanRefresh(user *meta.Meta) bool {
	switch userRole := o.manager.GetUserRole(user); userRole {
	case meta.UserRoleUnknown:
		return o.authConfig.GetSimpleMode()
	case meta.UserRoleCreator:
		return o.authConfig.GetExpertMode() || o.authConfig.GetSimpleMode()
	}
	return true
}

func (o *ownerPolicy) checkVisibility(user *meta.Meta, vis meta.Visibility) (bool, bool) {
	if vis == meta.VisibilityExpert {
		return o.userIsOwner(user) && o.authConfig.GetExpertMode(), true
	}
	return false, false
}

func (o *ownerPolicy) userIsOwner(user *meta.Meta) bool {
	if user == nil {
		return false
	}
	if o.manager.IsOwner(user.Zid) {
		return true
	}
	if val, ok := user.Get(meta.KeyUserRole); ok && val == meta.ValueUserRoleOwner {
		return true
	}
	return false
}
