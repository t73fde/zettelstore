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

// Mention all needed boxes, encoders, and parsers to have them registered.
import (
	_ "zettelstore.de/z/internal/box/compbox"  // Allow to use computed box.
	_ "zettelstore.de/z/internal/box/constbox" // Allow to use global internal box.
	_ "zettelstore.de/z/internal/box/dirbox"   // Allow to use directory box.
	_ "zettelstore.de/z/internal/box/filebox"  // Allow to use file box.
	_ "zettelstore.de/z/internal/box/membox"   // Allow to use in-memory box.
	_ "zettelstore.de/z/internal/kernel/impl"  // Allow kernel implementation to create itself.
)
