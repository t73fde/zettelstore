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
	"io"
	"net/url"
	"time"

	"zettelstore.de/z/auth"
	"zettelstore.de/z/box"
	"zettelstore.de/z/config"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/zettel/id"
)

// Kernel is the main internal service.
type Kernel interface {
	// Setup sets the most basic data of a software: its name, its version,
	// and when the version was created.
	Setup(progname, version string, versionTime time.Time)

	// Start the service.
	Start(headline bool, lineServer bool, configFile string)

	// WaitForShutdown blocks the call until Shutdown is called.
	WaitForShutdown()

	// Shutdown the service. Waits for all concurrent activities to stop.
	Shutdown(silent bool)

	// GetKernelLogger returns the kernel logger.
	GetKernelLogger() *logger.Logger

	// SetLogLevel sets the logging level for logger maintained by the kernel.
	//
	// Its syntax is: (SERVICE ":")? LEVEL (";" (SERICE ":")? LEVEL)*.
	SetLogLevel(string)

	// LogRecover outputs some information about the previous panic.
	LogRecover(name string, recoverInfo interface{}) bool

	// StartProfiling starts profiling the software according to a profile.
	// It is an error to start more than one profile.
	//
	// profileName is a valid profile (see runtime/pprof/Lookup()), or the
	// value "cpu" for profiling the CPI.
	// fileName is the name of the file where the results are written to.
	StartProfiling(profileName, fileName string) error

	// StopProfiling stops the current profiling and writes the result to
	// the file, which was named during StartProfiling().
	// It will always be called before the software stops its operations.
	StopProfiling() error

	// SetConfig stores a configuration value.
	SetConfig(srv Service, key, value string) error

	// GetConfig returns a configuration value.
	GetConfig(srv Service, key string) interface{}

	// GetConfigList returns a sorted list of configuration data.
	GetConfigList(Service) []KeyDescrValue

	// GetLogger returns a logger for the given service.
	GetLogger(Service) *logger.Logger

	// SetLevel sets the logging level for the given service.
	SetLevel(Service, logger.Level)

	// RetrieveLogEntries returns all buffered log entries.
	RetrieveLogEntries() []LogEntry

	// GetLastLogTime returns the time when the last logging with level > DEBUG happened.
	GetLastLogTime() time.Time

	// StartService start the given service.
	StartService(Service) error

	// RestartService stops and restarts the given service, while maintaining service dependencies.
	RestartService(Service) error

	// StopService stop the given service.
	StopService(Service)

	// GetServiceStatistics returns a key/value list with statistical data.
	GetServiceStatistics(Service) []KeyValue

	// DumpIndex writes some data about the internal index into a writer.
	DumpIndex(io.Writer)

	// SetCreators store functions to be called when a service has to be created.
	SetCreators(CreateAuthManagerFunc, CreateBoxManagerFunc, SetupWebServerFunc)
}

// Main references the main kernel.
var Main Kernel

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
