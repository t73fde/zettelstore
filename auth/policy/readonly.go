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

import "t73f.de/r/zsc/domain/meta"

type roPolicy struct{}

func (*roPolicy) CanCreate(_, _ *meta.Meta) bool   { return false }
func (*roPolicy) CanRead(_, _ *meta.Meta) bool     { return true }
func (*roPolicy) CanWrite(_, _, _ *meta.Meta) bool { return false }
func (*roPolicy) CanDelete(_, _ *meta.Meta) bool   { return false }
func (*roPolicy) CanRefresh(user *meta.Meta) bool  { return user != nil }
