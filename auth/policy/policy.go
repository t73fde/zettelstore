//-----------------------------------------------------------------------------
// Copyright (c) 2020-2021 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package policy provides some interfaces and implementation for authorizsation policies.
package policy

import (
	"zettelstore.de/z/auth"
	"zettelstore.de/z/config"
	"zettelstore.de/z/domain/meta"
)

// newPolicy creates a policy based on given constraints.
func newPolicy(manager auth.AuthzManager, authConfig config.AuthConfig) auth.Policy {
	var pol auth.Policy
	if manager.IsReadonly() {
		pol = &roPolicy{}
	} else {
		pol = &defaultPolicy{manager}
	}
	if manager.WithAuth() {
		pol = &ownerPolicy{
			manager:    manager,
			authConfig: authConfig,
			pre:        pol,
		}
	} else {
		pol = &anonPolicy{
			authConfig: authConfig,
			pre:        pol,
		}
	}
	return &prePolicy{pol}
}

type prePolicy struct {
	post auth.Policy
}

func (p *prePolicy) CanCreate(user, newMeta *meta.Meta) bool {
	return newMeta != nil && p.post.CanCreate(user, newMeta)
}

func (p *prePolicy) CanRead(user, m *meta.Meta) bool {
	return m != nil && p.post.CanRead(user, m)
}

func (p *prePolicy) CanWrite(user, oldMeta, newMeta *meta.Meta) bool {
	return oldMeta != nil && newMeta != nil && oldMeta.Zid == newMeta.Zid &&
		p.post.CanWrite(user, oldMeta, newMeta)
}

func (p *prePolicy) CanRename(user, m *meta.Meta) bool {
	return m != nil && p.post.CanRename(user, m)
}

func (p *prePolicy) CanDelete(user, m *meta.Meta) bool {
	return m != nil && p.post.CanDelete(user, m)
}

func (p *prePolicy) CanRefresh(user *meta.Meta) bool {
	return p.post.CanRefresh(user)
}
