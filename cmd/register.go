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
	_ "zettelstore.de/z/internal/box/compbox"       // Allow to use computed box.
	_ "zettelstore.de/z/internal/box/constbox"      // Allow to use global internal box.
	_ "zettelstore.de/z/internal/box/dirbox"        // Allow to use directory box.
	_ "zettelstore.de/z/internal/box/filebox"       // Allow to use file box.
	_ "zettelstore.de/z/internal/box/membox"        // Allow to use in-memory box.
	_ "zettelstore.de/z/internal/encoder/htmlenc"   // Allow to use HTML encoder.
	_ "zettelstore.de/z/internal/encoder/mdenc"     // Allow to use markdown encoder.
	_ "zettelstore.de/z/internal/encoder/shtmlenc"  // Allow to use SHTML encoder.
	_ "zettelstore.de/z/internal/encoder/szenc"     // Allow to use Sz encoder.
	_ "zettelstore.de/z/internal/encoder/textenc"   // Allow to use text encoder.
	_ "zettelstore.de/z/internal/encoder/zmkenc"    // Allow to use zmk encoder.
	_ "zettelstore.de/z/internal/kernel/impl"       // Allow kernel implementation to create itself
	_ "zettelstore.de/z/internal/parser/blob"       // Allow to use BLOB parser.
	_ "zettelstore.de/z/internal/parser/draw"       // Allow to use draw parser.
	_ "zettelstore.de/z/internal/parser/markdown"   // Allow to use markdown parser.
	_ "zettelstore.de/z/internal/parser/none"       // Allow to use none parser.
	_ "zettelstore.de/z/internal/parser/plain"      // Allow to use plain parser.
	_ "zettelstore.de/z/internal/parser/zettelmark" // Allow to use zettelmark parser.
)
