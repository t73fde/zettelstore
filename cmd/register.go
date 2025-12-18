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

package cmd

// Mention all needed boxes to have them registered.
import (
	_ "zettelstore.de/z/internal/box/compbox"
	_ "zettelstore.de/z/internal/box/constbox"
	_ "zettelstore.de/z/internal/box/dirbox"
	_ "zettelstore.de/z/internal/box/filebox"
	_ "zettelstore.de/z/internal/box/membox"
)
