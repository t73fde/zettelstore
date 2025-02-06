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
	"zettelstore.de/z/auth"
)

type defaultPolicy struct {
	manager auth.AuthzManager
}

func (*defaultPolicy) CanCreate(_, _ *meta.Meta) bool { return true }
func (*defaultPolicy) CanRead(_, _ *meta.Meta) bool   { return true }
func (d *defaultPolicy) CanWrite(user, oldMeta, _ *meta.Meta) bool {
	return d.canChange(user, oldMeta)
}
func (d *defaultPolicy) CanDelete(user, m *meta.Meta) bool { return d.canChange(user, m) }

func (*defaultPolicy) CanRefresh(user *meta.Meta) bool { return user != nil }

func (d *defaultPolicy) canChange(user, m *meta.Meta) bool {
	metaRo, ok := m.Get(meta.KeyReadOnly)
	if !ok {
		return true
	}
	if user == nil {
		// If we are here, there is no authentication.
		// See owner.go:CanWrite.

		// No authentication: check for owner-like restriction, because the user
		// acts as an owner
		return metaRo != meta.ValueUserRoleOwner && !metaRo.AsBool()
	}

	userRole := d.manager.GetUserRole(user)
	switch metaRo {
	case meta.ValueUserRoleReader:
		return userRole > meta.UserRoleReader
	case meta.ValueUserRoleWriter:
		return userRole > meta.UserRoleWriter
	case meta.ValueUserRoleOwner:
		return userRole > meta.UserRoleOwner
	}
	return !metaRo.AsBool()
}
