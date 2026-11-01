//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package manager

import (
	"zettelstore.de/z/internal/index"
	"zettelstore.de/z/internal/logging"
)

// SearchEqual returns all zettel that contains the given exact word.
// The word must be normalized through Unicode NFKD, trimmed and not empty.
func (mgr *Manager) SearchEqual(word string) *index.Result {
	found := mgr.index.SearchEqual(word)

	mgr.mgrLogger.Debug("SearchEqual", "word", word, "found", found.Count())
	logging.LogTrace(mgr.mgrLogger, "IDs", "ids", found)
	return found
}

// SearchPrefix returns all zettel that have a word with the given prefix.
// The prefix must be normalized through Unicode NFKD, trimmed and not empty.
func (mgr *Manager) SearchPrefix(prefix string) *index.Result {
	found := mgr.index.SearchPrefix(prefix)

	mgr.mgrLogger.Debug("SearchPrefix", "prefix", prefix, "found", found.Count())
	logging.LogTrace(mgr.mgrLogger, "IDs", "ids", found)
	return found
}

// SearchSuffix returns all zettel that have a word with the given suffix.
// The suffix must be normalized through Unicode NFKD, trimmed and not empty.
func (mgr *Manager) SearchSuffix(suffix string) *index.Result {
	found := mgr.index.SearchSuffix(suffix)

	mgr.mgrLogger.Debug("SearchSuffix", "suffix", suffix, "found", found.Count())
	logging.LogTrace(mgr.mgrLogger, "IDs", "ids", found)
	return found
}

// SearchContains returns all zettel that contains the given string.
// The string must be normalized through Unicode NFKD, trimmed and not empty.
func (mgr *Manager) SearchContains(s string) *index.Result {
	found := mgr.index.SearchContains(s)

	mgr.mgrLogger.Debug("SearchContains", "s", s, "found", found.Count())
	logging.LogTrace(mgr.mgrLogger, "IDs", "ids", found)
	return found
}
