//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package config provides functions to retrieve runtime configuration data.
package config

import (
	"context"

	"zettelstore.de/z/zettel/meta"
)

// Key values that are supported by Config.Get
const (
	KeyFooterZettel          = "footer-zettel"
	KeyHomeZettel            = "home-zettel"
	KeyShowBackLinks         = "show-back-links"
	KeyShowFolgeLinks        = "show-folge-links"
	KeyShowSubordinatesLinks = "show-subordinates-links"
	KeyShowSuccessorsLinks   = "show-successors-links"
	// api.KeyLang
)

// Config allows to retrieve all defined configuration values that can be changed during runtime.
type Config interface {
	AuthConfig

	// Get returns the value of the given key. It searches first in the given metadata,
	// then in the data of the current user, and at last in the system-wide data.
	Get(ctx context.Context, m *meta.Meta, key string) string

	// AddDefaultValues enriches the given meta data with its default values.
	AddDefaultValues(context.Context, *meta.Meta) *meta.Meta

	// GetSiteName returns the current value of the "site-name" key.
	GetSiteName() string

	// GetHTMLInsecurity returns the current
	GetHTMLInsecurity() HTMLInsecurity

	// GetMaxTransclusions returns the maximum number of indirect transclusions.
	GetMaxTransclusions() int

	// GetYAMLHeader returns the current value of the "yaml-header" key.
	GetYAMLHeader() bool

	// GetZettelFileSyntax returns the current value of the "zettel-file-syntax" key.
	GetZettelFileSyntax() []string
}

// AuthConfig are relevant configuration values for authentication.
type AuthConfig interface {
	// GetSimpleMode returns true if system tuns in simple-mode.
	GetSimpleMode() bool

	// GetExpertMode returns the current value of the "expert-mode" key.
	GetExpertMode() bool

	// GetVisibility returns the visibility value of the metadata.
	GetVisibility(m *meta.Meta) meta.Visibility
}

// HTMLInsecurity states what kind of insecure HTML is allowed.
// The lowest value is the most secure one (disallowing any HTML)
type HTMLInsecurity uint8

// Constant values for HTMLInsecurity:
const (
	NoHTML HTMLInsecurity = iota
	SyntaxHTML
	MarkdownHTML
	ZettelmarkupHTML
)

func (hi HTMLInsecurity) String() string {
	switch hi {
	case SyntaxHTML:
		return "html"
	case MarkdownHTML:
		return "markdown"
	case ZettelmarkupHTML:
		return "zettelmarkup"
	}
	return "secure"
}

// AllowHTML returns true, if the given HTML insecurity level matches the given syntax value.
func (hi HTMLInsecurity) AllowHTML(syntax string) bool {
	switch hi {
	case SyntaxHTML:
		return syntax == meta.SyntaxHTML
	case MarkdownHTML:
		return syntax == meta.SyntaxHTML || syntax == meta.SyntaxMarkdown || syntax == meta.SyntaxMD
	case ZettelmarkupHTML:
		return syntax == meta.SyntaxZmk || syntax == meta.SyntaxHTML ||
			syntax == meta.SyntaxMarkdown || syntax == meta.SyntaxMD
	}
	return false
}
