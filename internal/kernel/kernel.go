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

// Package kernel provides the main kernel service.
package kernel

import (
	"net/url"
	"time"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/logger"
	"zettelstore.de/z/internal/web/server"
)

// Main references the main kernel.
var Main *Kernel

// Unit is a type with just one value.
type Unit struct{}

// ShutdownChan is a channel used to signal a system shutdown.
type ShutdownChan <-chan Unit

// Constants for profile names.
const (
	ProfileCPU  = "CPU"
	ProfileHead = "heap"
)

// Service specifies a service, e.g. web, ...
type Service uint8

// Constants for type Service.
const (
	_             Service = iota
	KernelService         // The Kernel itself is also a sevice
	CoreService           // Manages startup specific functionality
	ConfigService         // Provides access to runtime configuration
	AuthService           // Manages authentication
	BoxService            // Boxes provide zettel
	WebService            // Access to Zettelstore through Web-based API and WebUI
)

// Constants for core service system keys.
const (
	CoreDebug     = "debug"
	CoreGoArch    = "go-arch"
	CoreGoOS      = "go-os"
	CoreGoVersion = "go-version"
	CoreHostname  = "hostname"
	CorePort      = "port"
	CoreProgname  = "progname"
	CoreStarted   = "started"
	CoreVerbose   = "verbose"
	CoreVersion   = "version"
	CoreVTime     = "vtime"
)

// Defined values for core service.
const (
	CoreDefaultVersion = "unknown"
)

// Constants for config service keys.
const (
	ConfigSimpleMode   = "simple-mode"
	ConfigInsecureHTML = "insecure-html"
)

// Constants for authentication service keys.
const (
	AuthOwner    = "owner"
	AuthReadonly = "readonly"
)

// Constants for box service keys.
const (
	BoxDefaultDirType = "defdirtype"
	BoxURIs           = "box-uri-"
)

// Allowed values for BoxDefaultDirType
const (
	BoxDirTypeNotify = "notify"
	BoxDirTypeSimple = "simple"
)

// Constants for config service keys.
const (
	ConfigSecureHTML   = "secure"
	ConfigSyntaxHTML   = "html"
	ConfigMarkdownHTML = "markdown"
	ConfigZmkHTML      = "zettelmarkup"
)

// Constants for web service keys.
const (
	WebAssetDir          = "asset-dir"
	WebBaseURL           = "base-url"
	WebListenAddress     = "listen"
	WebPersistentCookie  = "persistent"
	WebProfiling         = "profiling"
	WebMaxRequestSize    = "max-request-size"
	WebSecureCookie      = "secure"
	WebTokenLifetimeAPI  = "api-lifetime"
	WebTokenLifetimeHTML = "html-lifetime"
	WebURLPrefix         = "prefix"
)

// KeyDescrValue is a triple of config data.
type KeyDescrValue struct{ Key, Descr, Value string }

// KeyValue is a pair of key and value.
type KeyValue struct{ Key, Value string }

// LogEntry stores values of one log line written by a logger.Logger
type LogEntry struct {
	Level   logger.Level
	TS      time.Time
	Prefix  string
	Message string
}

// CreateAuthManagerFunc is called to create a new auth manager.
type CreateAuthManagerFunc func(readonly bool, owner id.Zid) (auth.Manager, error)

// CreateBoxManagerFunc is called to create a new box manager.
type CreateBoxManagerFunc func(
	boxURIs []*url.URL,
	authManager auth.Manager,
	rtConfig config.Config,
) (box.Manager, error)

// SetupWebServerFunc is called to create a new web service handler.
type SetupWebServerFunc func(
	webServer server.Server,
	boxManager box.Manager,
	authManager auth.Manager,
	rtConfig config.Config,
) error
