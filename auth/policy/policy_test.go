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
	"fmt"
	"testing"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/auth"
)

func TestPolicies(t *testing.T) {
	t.Parallel()
	testScene := []struct {
		readonly bool
		withAuth bool
		expert   bool
		simple   bool
	}{
		{true, true, true, true},
		{true, true, true, false},
		{true, true, false, true},
		{true, true, false, false},
		{true, false, true, true},
		{true, false, true, false},
		{true, false, false, true},
		{true, false, false, false},
		{false, true, true, true},
		{false, true, true, false},
		{false, true, false, true},
		{false, true, false, false},
		{false, false, true, true},
		{false, false, true, false},
		{false, false, false, true},
		{false, false, false, false},
	}
	for _, ts := range testScene {
		pol := newPolicy(
			&testAuthzManager{readOnly: ts.readonly, withAuth: ts.withAuth},
			&authConfig{simple: ts.simple, expert: ts.expert},
		)
		name := fmt.Sprintf("readonly=%v/withauth=%v/expert=%v/simple=%v",
			ts.readonly, ts.withAuth, ts.expert, ts.simple)
		t.Run(name, func(tt *testing.T) {
			testCreate(tt, pol, ts.withAuth, ts.readonly)
			testRead(tt, pol, ts.withAuth, ts.expert)
			testWrite(tt, pol, ts.withAuth, ts.readonly, ts.expert)
			testDelete(tt, pol, ts.withAuth, ts.readonly, ts.expert)
			testRefresh(tt, pol, ts.withAuth, ts.expert, ts.simple)
		})
	}
}

type testAuthzManager struct {
	readOnly bool
	withAuth bool
}

func (a *testAuthzManager) IsReadonly() bool      { return a.readOnly }
func (*testAuthzManager) Owner() id.Zid           { return ownerZid }
func (*testAuthzManager) IsOwner(zid id.Zid) bool { return zid == ownerZid }

func (a *testAuthzManager) WithAuth() bool { return a.withAuth }

func (a *testAuthzManager) GetUserRole(user *meta.Meta) meta.UserRole {
	if user == nil {
		if a.WithAuth() {
			return meta.UserRoleUnknown
		}
		return meta.UserRoleOwner
	}
	if a.IsOwner(user.Zid) {
		return meta.UserRoleOwner
	}
	if val, ok := user.Get(api.KeyUserRole); ok {
		if ur := val.GetUserRole(); ur != meta.UserRoleUnknown {
			return ur
		}
	}
	return meta.UserRoleReader
}

type authConfig struct{ simple, expert bool }

func (ac *authConfig) GetSimpleMode() bool { return ac.simple }
func (ac *authConfig) GetExpertMode() bool { return ac.expert }

func (*authConfig) GetVisibility(m *meta.Meta) meta.Visibility {
	if val, ok := m.Get(api.KeyVisibility); ok {
		return val.GetVisibility()
	}
	return meta.VisibilityLogin
}

func testCreate(t *testing.T, pol auth.Policy, withAuth, readonly bool) {
	t.Helper()
	anonUser := newAnon()
	creator := newCreator()
	reader := newReader()
	writer := newWriter()
	owner := newOwner()
	owner2 := newOwner2()
	zettel := newZettel()
	userZettel := newUserZettel()
	testCases := []struct {
		user *meta.Meta
		meta *meta.Meta
		exp  bool
	}{
		// No meta
		{anonUser, nil, false},
		{creator, nil, false},
		{reader, nil, false},
		{writer, nil, false},
		{owner, nil, false},
		{owner2, nil, false},
		// Ordinary zettel
		{anonUser, zettel, !withAuth && !readonly},
		{creator, zettel, !readonly},
		{reader, zettel, !withAuth && !readonly},
		{writer, zettel, !readonly},
		{owner, zettel, !readonly},
		{owner2, zettel, !readonly},
		// User zettel
		{anonUser, userZettel, !withAuth && !readonly},
		{creator, userZettel, !withAuth && !readonly},
		{reader, userZettel, !withAuth && !readonly},
		{writer, userZettel, !withAuth && !readonly},
		{owner, userZettel, !readonly},
		{owner2, userZettel, !readonly},
	}
	for _, tc := range testCases {
		t.Run("Create", func(tt *testing.T) {
			got := pol.CanCreate(tc.user, tc.meta)
			if tc.exp != got {
				tt.Errorf("exp=%v, but got=%v", tc.exp, got)
			}
		})
	}
}

func testRead(t *testing.T, pol auth.Policy, withAuth, expert bool) {
	t.Helper()
	anonUser := newAnon()
	creator := newCreator()
	reader := newReader()
	writer := newWriter()
	owner := newOwner()
	owner2 := newOwner2()
	zettel := newZettel()
	publicZettel := newPublicZettel()
	creatorZettel := newCreatorZettel()
	loginZettel := newLoginZettel()
	ownerZettel := newOwnerZettel()
	expertZettel := newExpertZettel()
	userZettel := newUserZettel()
	testCases := []struct {
		user *meta.Meta
		meta *meta.Meta
		exp  bool
	}{
		// No meta
		{anonUser, nil, false},
		{creator, nil, false},
		{reader, nil, false},
		{writer, nil, false},
		{owner, nil, false},
		{owner2, nil, false},
		// Ordinary zettel
		{anonUser, zettel, !withAuth},
		{creator, zettel, !withAuth},
		{reader, zettel, true},
		{writer, zettel, true},
		{owner, zettel, true},
		{owner2, zettel, true},
		// Public zettel
		{anonUser, publicZettel, true},
		{creator, publicZettel, true},
		{reader, publicZettel, true},
		{writer, publicZettel, true},
		{owner, publicZettel, true},
		{owner2, publicZettel, true},
		// Creator zettel
		{anonUser, creatorZettel, !withAuth},
		{creator, creatorZettel, true},
		{reader, creatorZettel, true},
		{writer, creatorZettel, true},
		{owner, creatorZettel, true},
		{owner2, creatorZettel, true},
		// Login zettel
		{anonUser, loginZettel, !withAuth},
		{creator, loginZettel, !withAuth},
		{reader, loginZettel, true},
		{writer, loginZettel, true},
		{owner, loginZettel, true},
		{owner2, loginZettel, true},
		// Owner zettel
		{anonUser, ownerZettel, !withAuth},
		{creator, ownerZettel, !withAuth},
		{reader, ownerZettel, !withAuth},
		{writer, ownerZettel, !withAuth},
		{owner, ownerZettel, true},
		{owner2, ownerZettel, true},
		// Expert zettel
		{anonUser, expertZettel, !withAuth && expert},
		{creator, expertZettel, !withAuth && expert},
		{reader, expertZettel, !withAuth && expert},
		{writer, expertZettel, !withAuth && expert},
		{owner, expertZettel, expert},
		{owner2, expertZettel, expert},
		// Other user zettel
		{anonUser, userZettel, !withAuth},
		{creator, userZettel, !withAuth},
		{reader, userZettel, !withAuth},
		{writer, userZettel, !withAuth},
		{owner, userZettel, true},
		{owner2, userZettel, true},
		// Own user zettel
		{creator, creator, true},
		{reader, reader, true},
		{writer, writer, true},
		{owner, owner, true},
		{owner, owner2, true},
		{owner2, owner, true},
		{owner2, owner2, true},
	}
	for _, tc := range testCases {
		t.Run("Read", func(tt *testing.T) {
			got := pol.CanRead(tc.user, tc.meta)
			if tc.exp != got {
				tt.Errorf("exp=%v, but got=%v", tc.exp, got)
			}
		})
	}
}

func testWrite(t *testing.T, pol auth.Policy, withAuth, readonly, expert bool) {
	t.Helper()
	anonUser := newAnon()
	creator := newCreator()
	reader := newReader()
	writer := newWriter()
	owner := newOwner()
	owner2 := newOwner2()
	zettel := newZettel()
	publicZettel := newPublicZettel()
	loginZettel := newLoginZettel()
	ownerZettel := newOwnerZettel()
	expertZettel := newExpertZettel()
	userZettel := newUserZettel()
	writerNew := writer.Clone()
	writerNew.Set(api.KeyUserRole, owner.GetDefault(api.KeyUserRole, ""))
	roFalse := newRoFalseZettel()
	roTrue := newRoTrueZettel()
	roReader := newRoReaderZettel()
	roWriter := newRoWriterZettel()
	roOwner := newRoOwnerZettel()
	notAuthNotReadonly := !withAuth && !readonly
	testCases := []struct {
		user *meta.Meta
		old  *meta.Meta
		new  *meta.Meta
		exp  bool
	}{
		// No old and new meta
		{anonUser, nil, nil, false},
		{creator, nil, nil, false},
		{reader, nil, nil, false},
		{writer, nil, nil, false},
		{owner, nil, nil, false},
		{owner2, nil, nil, false},
		// No old meta
		{anonUser, nil, zettel, false},
		{creator, nil, zettel, false},
		{reader, nil, zettel, false},
		{writer, nil, zettel, false},
		{owner, nil, zettel, false},
		{owner2, nil, zettel, false},
		// No new meta
		{anonUser, zettel, nil, false},
		{creator, zettel, nil, false},
		{reader, zettel, nil, false},
		{writer, zettel, nil, false},
		{owner, zettel, nil, false},
		{owner2, zettel, nil, false},
		// Old an new zettel have different zettel identifier
		{anonUser, zettel, publicZettel, false},
		{creator, zettel, publicZettel, false},
		{reader, zettel, publicZettel, false},
		{writer, zettel, publicZettel, false},
		{owner, zettel, publicZettel, false},
		{owner2, zettel, publicZettel, false},
		// Overwrite a normal zettel
		{anonUser, zettel, zettel, notAuthNotReadonly},
		{creator, zettel, zettel, notAuthNotReadonly},
		{reader, zettel, zettel, notAuthNotReadonly},
		{writer, zettel, zettel, !readonly},
		{owner, zettel, zettel, !readonly},
		{owner2, zettel, zettel, !readonly},
		// Public zettel
		{anonUser, publicZettel, publicZettel, notAuthNotReadonly},
		{creator, publicZettel, publicZettel, notAuthNotReadonly},
		{reader, publicZettel, publicZettel, notAuthNotReadonly},
		{writer, publicZettel, publicZettel, !readonly},
		{owner, publicZettel, publicZettel, !readonly},
		{owner2, publicZettel, publicZettel, !readonly},
		// Login zettel
		{anonUser, loginZettel, loginZettel, notAuthNotReadonly},
		{creator, loginZettel, loginZettel, notAuthNotReadonly},
		{reader, loginZettel, loginZettel, notAuthNotReadonly},
		{writer, loginZettel, loginZettel, !readonly},
		{owner, loginZettel, loginZettel, !readonly},
		{owner2, loginZettel, loginZettel, !readonly},
		// Owner zettel
		{anonUser, ownerZettel, ownerZettel, notAuthNotReadonly},
		{creator, ownerZettel, ownerZettel, notAuthNotReadonly},
		{reader, ownerZettel, ownerZettel, notAuthNotReadonly},
		{writer, ownerZettel, ownerZettel, notAuthNotReadonly},
		{owner, ownerZettel, ownerZettel, !readonly},
		{owner2, ownerZettel, ownerZettel, !readonly},
		// Expert zettel
		{anonUser, expertZettel, expertZettel, notAuthNotReadonly && expert},
		{creator, expertZettel, expertZettel, notAuthNotReadonly && expert},
		{reader, expertZettel, expertZettel, notAuthNotReadonly && expert},
		{writer, expertZettel, expertZettel, notAuthNotReadonly && expert},
		{owner, expertZettel, expertZettel, !readonly && expert},
		{owner2, expertZettel, expertZettel, !readonly && expert},
		// Other user zettel
		{anonUser, userZettel, userZettel, notAuthNotReadonly},
		{creator, userZettel, userZettel, notAuthNotReadonly},
		{reader, userZettel, userZettel, notAuthNotReadonly},
		{writer, userZettel, userZettel, notAuthNotReadonly},
		{owner, userZettel, userZettel, !readonly},
		{owner2, userZettel, userZettel, !readonly},
		// Own user zettel
		{creator, creator, creator, !readonly},
		{reader, reader, reader, !readonly},
		{writer, writer, writer, !readonly},
		{owner, owner, owner, !readonly},
		{owner2, owner2, owner2, !readonly},
		// Writer cannot change importand metadata of its own user zettel
		{writer, writer, writerNew, notAuthNotReadonly},
		// No r/o zettel
		{anonUser, roFalse, roFalse, notAuthNotReadonly},
		{creator, roFalse, roFalse, notAuthNotReadonly},
		{reader, roFalse, roFalse, notAuthNotReadonly},
		{writer, roFalse, roFalse, !readonly},
		{owner, roFalse, roFalse, !readonly},
		{owner2, roFalse, roFalse, !readonly},
		// Reader r/o zettel
		{anonUser, roReader, roReader, false},
		{creator, roReader, roReader, false},
		{reader, roReader, roReader, false},
		{writer, roReader, roReader, !readonly},
		{owner, roReader, roReader, !readonly},
		{owner2, roReader, roReader, !readonly},
		// Writer r/o zettel
		{anonUser, roWriter, roWriter, false},
		{creator, roWriter, roWriter, false},
		{reader, roWriter, roWriter, false},
		{writer, roWriter, roWriter, false},
		{owner, roWriter, roWriter, !readonly},
		{owner2, roWriter, roWriter, !readonly},
		// Owner r/o zettel
		{anonUser, roOwner, roOwner, false},
		{creator, roOwner, roOwner, false},
		{reader, roOwner, roOwner, false},
		{writer, roOwner, roOwner, false},
		{owner, roOwner, roOwner, false},
		{owner2, roOwner, roOwner, false},
		// r/o = true zettel
		{anonUser, roTrue, roTrue, false},
		{creator, roTrue, roTrue, false},
		{reader, roTrue, roTrue, false},
		{writer, roTrue, roTrue, false},
		{owner, roTrue, roTrue, false},
		{owner2, roTrue, roTrue, false},
	}
	for _, tc := range testCases {
		t.Run("Write", func(tt *testing.T) {
			got := pol.CanWrite(tc.user, tc.old, tc.new)
			if tc.exp != got {
				tt.Errorf("exp=%v, but got=%v", tc.exp, got)
			}
		})
	}
}

func testDelete(t *testing.T, pol auth.Policy, withAuth, readonly, expert bool) {
	t.Helper()
	anonUser := newAnon()
	creator := newCreator()
	reader := newReader()
	writer := newWriter()
	owner := newOwner()
	owner2 := newOwner2()
	zettel := newZettel()
	expertZettel := newExpertZettel()
	roFalse := newRoFalseZettel()
	roTrue := newRoTrueZettel()
	roReader := newRoReaderZettel()
	roWriter := newRoWriterZettel()
	roOwner := newRoOwnerZettel()
	notAuthNotReadonly := !withAuth && !readonly
	testCases := []struct {
		user *meta.Meta
		meta *meta.Meta
		exp  bool
	}{
		// No meta
		{anonUser, nil, false},
		{creator, nil, false},
		{reader, nil, false},
		{writer, nil, false},
		{owner, nil, false},
		{owner2, nil, false},
		// Any zettel
		{anonUser, zettel, notAuthNotReadonly},
		{creator, zettel, notAuthNotReadonly},
		{reader, zettel, notAuthNotReadonly},
		{writer, zettel, notAuthNotReadonly},
		{owner, zettel, !readonly},
		{owner2, zettel, !readonly},
		// Expert zettel
		{anonUser, expertZettel, notAuthNotReadonly && expert},
		{creator, expertZettel, notAuthNotReadonly && expert},
		{reader, expertZettel, notAuthNotReadonly && expert},
		{writer, expertZettel, notAuthNotReadonly && expert},
		{owner, expertZettel, !readonly && expert},
		{owner2, expertZettel, !readonly && expert},
		// No r/o zettel
		{anonUser, roFalse, notAuthNotReadonly},
		{creator, roFalse, notAuthNotReadonly},
		{reader, roFalse, notAuthNotReadonly},
		{writer, roFalse, notAuthNotReadonly},
		{owner, roFalse, !readonly},
		{owner2, roFalse, !readonly},
		// Reader r/o zettel
		{anonUser, roReader, false},
		{creator, roReader, false},
		{reader, roReader, false},
		{writer, roReader, notAuthNotReadonly},
		{owner, roReader, !readonly},
		{owner2, roReader, !readonly},
		// Writer r/o zettel
		{anonUser, roWriter, false},
		{creator, roWriter, false},
		{reader, roWriter, false},
		{writer, roWriter, false},
		{owner, roWriter, !readonly},
		{owner2, roWriter, !readonly},
		// Owner r/o zettel
		{anonUser, roOwner, false},
		{creator, roOwner, false},
		{reader, roOwner, false},
		{writer, roOwner, false},
		{owner, roOwner, false},
		{owner2, roOwner, false},
		// r/o = true zettel
		{anonUser, roTrue, false},
		{creator, roTrue, false},
		{reader, roTrue, false},
		{writer, roTrue, false},
		{owner, roTrue, false},
		{owner2, roTrue, false},
	}
	for _, tc := range testCases {
		t.Run("Delete", func(tt *testing.T) {
			got := pol.CanDelete(tc.user, tc.meta)
			if tc.exp != got {
				tt.Errorf("exp=%v, but got=%v", tc.exp, got)
			}
		})
	}
}

func testRefresh(t *testing.T, pol auth.Policy, withAuth, expert, simple bool) {
	t.Helper()
	testCases := []struct {
		user *meta.Meta
		exp  bool
	}{
		{newAnon(), (!withAuth && expert) || simple},
		{newCreator(), !withAuth || expert || simple},
		{newReader(), true},
		{newWriter(), true},
		{newOwner(), true},
		{newOwner2(), true},
	}
	for _, tc := range testCases {
		t.Run("Refresh", func(tt *testing.T) {
			got := pol.CanRefresh(tc.user)
			if tc.exp != got {
				tt.Errorf("exp=%v, but got=%v", tc.exp, got)
			}
		})
	}
}

const (
	creatorZid = id.Zid(1013)
	readerZid  = id.Zid(1013)
	writerZid  = id.Zid(1015)
	ownerZid   = id.Zid(1017)
	owner2Zid  = id.Zid(1019)
	zettelZid  = id.Zid(1021)
	visZid     = id.Zid(1023)
	userZid    = id.Zid(1025)
)

func newAnon() *meta.Meta { return nil }
func newCreator() *meta.Meta {
	user := meta.New(creatorZid)
	user.Set(api.KeyTitle, "Creator")
	user.Set(api.KeyUserID, "ceator")
	user.Set(api.KeyUserRole, api.ValueUserRoleCreator)
	return user
}
func newReader() *meta.Meta {
	user := meta.New(readerZid)
	user.Set(api.KeyTitle, "Reader")
	user.Set(api.KeyUserID, "reader")
	user.Set(api.KeyUserRole, api.ValueUserRoleReader)
	return user
}
func newWriter() *meta.Meta {
	user := meta.New(writerZid)
	user.Set(api.KeyTitle, "Writer")
	user.Set(api.KeyUserID, "writer")
	user.Set(api.KeyUserRole, api.ValueUserRoleWriter)
	return user
}
func newOwner() *meta.Meta {
	user := meta.New(ownerZid)
	user.Set(api.KeyTitle, "Owner")
	user.Set(api.KeyUserID, "owner")
	user.Set(api.KeyUserRole, api.ValueUserRoleOwner)
	return user
}
func newOwner2() *meta.Meta {
	user := meta.New(owner2Zid)
	user.Set(api.KeyTitle, "Owner 2")
	user.Set(api.KeyUserID, "owner-2")
	user.Set(api.KeyUserRole, api.ValueUserRoleOwner)
	return user
}
func newZettel() *meta.Meta {
	m := meta.New(zettelZid)
	m.Set(api.KeyTitle, "Any Zettel")
	return m
}
func newPublicZettel() *meta.Meta {
	m := meta.New(visZid)
	m.Set(api.KeyTitle, "Public Zettel")
	m.Set(api.KeyVisibility, api.ValueVisibilityPublic)
	return m
}
func newCreatorZettel() *meta.Meta {
	m := meta.New(visZid)
	m.Set(api.KeyTitle, "Creator Zettel")
	m.Set(api.KeyVisibility, api.ValueVisibilityCreator)
	return m
}
func newLoginZettel() *meta.Meta {
	m := meta.New(visZid)
	m.Set(api.KeyTitle, "Login Zettel")
	m.Set(api.KeyVisibility, api.ValueVisibilityLogin)
	return m
}
func newOwnerZettel() *meta.Meta {
	m := meta.New(visZid)
	m.Set(api.KeyTitle, "Owner Zettel")
	m.Set(api.KeyVisibility, api.ValueVisibilityOwner)
	return m
}
func newExpertZettel() *meta.Meta {
	m := meta.New(visZid)
	m.Set(api.KeyTitle, "Expert Zettel")
	m.Set(api.KeyVisibility, api.ValueVisibilityExpert)
	return m
}
func newRoFalseZettel() *meta.Meta {
	m := meta.New(zettelZid)
	m.Set(api.KeyTitle, "No r/o Zettel")
	m.Set(api.KeyReadOnly, api.ValueFalse)
	return m
}
func newRoTrueZettel() *meta.Meta {
	m := meta.New(zettelZid)
	m.Set(api.KeyTitle, "A r/o Zettel")
	m.Set(api.KeyReadOnly, api.ValueTrue)
	return m
}
func newRoReaderZettel() *meta.Meta {
	m := meta.New(zettelZid)
	m.Set(api.KeyTitle, "Reader r/o Zettel")
	m.Set(api.KeyReadOnly, api.ValueUserRoleReader)
	return m
}
func newRoWriterZettel() *meta.Meta {
	m := meta.New(zettelZid)
	m.Set(api.KeyTitle, "Writer r/o Zettel")
	m.Set(api.KeyReadOnly, api.ValueUserRoleWriter)
	return m
}
func newRoOwnerZettel() *meta.Meta {
	m := meta.New(zettelZid)
	m.Set(api.KeyTitle, "Owner r/o Zettel")
	m.Set(api.KeyReadOnly, api.ValueUserRoleOwner)
	return m
}
func newUserZettel() *meta.Meta {
	m := meta.New(userZid)
	m.Set(api.KeyTitle, "Any User")
	m.Set(api.KeyUserID, "any")
	return m
}
