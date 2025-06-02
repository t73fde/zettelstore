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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/web/server"
)

// Main references the main kernel.
var Main *Kernel

// Kernel is the main internal kernel.
type Kernel struct {
	logLevelVar slog.LevelVar
	logger      *slog.Logger
	dlogWriter  *kernelLogWriter
	wg          sync.WaitGroup
	mx          sync.RWMutex
	interrupt   chan os.Signal

	profileName string
	fileName    string
	profileFile *os.File
	profile     *pprof.Profile

	self kernelService
	core coreService
	cfg  configService
	auth authService
	box  boxService
	web  webService

	srvs     map[Service]*serviceDescr
	srvNames map[string]serviceData
	depStart serviceDependency
	depStop  serviceDependency // reverse of depStart
}

type serviceDescr struct {
	srv         service
	name        string
	logLevel    slog.Level
	logLevelVar slog.LevelVar
}
type serviceData struct {
	srv    service
	srvnum Service
}
type serviceDependency map[Service][]Service

// create a new kernel.
func init() {
	Main = createKernel()
}

// create a new
func createKernel() *Kernel {
	lw := newKernelLogWriter(8192)
	kern := &Kernel{
		dlogWriter: lw,
		interrupt:  make(chan os.Signal, 5),
	}
	kern.logLevelVar.Set(defaultNormalLogLevel)
	kern.logger = slog.New(newKernelLogHandler(lw, &kern.logLevelVar))
	kern.self.kernel = kern
	kern.srvs = map[Service]*serviceDescr{
		KernelService: {srv: &kern.self, name: "kernel", logLevel: defaultNormalLogLevel},
		CoreService:   {srv: &kern.core, name: "core", logLevel: defaultNormalLogLevel},
		ConfigService: {srv: &kern.cfg, name: "config", logLevel: defaultNormalLogLevel},
		AuthService:   {srv: &kern.auth, name: "auth", logLevel: defaultNormalLogLevel},
		BoxService:    {srv: &kern.box, name: "box", logLevel: defaultNormalLogLevel},
		WebService:    {srv: &kern.web, name: "web", logLevel: defaultNormalLogLevel},
	}
	kern.srvNames = make(map[string]serviceData, len(kern.srvs))
	for key, srvD := range kern.srvs {
		if _, ok := kern.srvNames[srvD.name]; ok {
			kern.logger.Error("Service data already set, ignored", "service", srvD.name)
		}
		kern.srvNames[srvD.name] = serviceData{srvD.srv, key}

		srvD.logLevelVar.Set(srvD.logLevel)
		srvLogger := slog.New(newKernelLogHandler(lw, &srvD.logLevelVar)).With(
			"system", strings.ToUpper(srvD.name))
		kern.logger.Debug("Initialize", "service", srvD.name)
		srvD.srv.Initialize(&srvD.logLevelVar, srvLogger)
	}
	kern.depStart = serviceDependency{
		KernelService: nil,
		CoreService:   {KernelService},
		ConfigService: {CoreService},
		AuthService:   {CoreService},
		BoxService:    {CoreService, ConfigService, AuthService},
		WebService:    {ConfigService, AuthService, BoxService},
	}
	kern.depStop = make(serviceDependency, len(kern.depStart))
	for srv, deps := range kern.depStart {
		for _, dep := range deps {
			kern.depStop[dep] = append(kern.depStop[dep], srv)
		}
	}
	return kern
}

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
	WebLoopbackIdent     = "loopback-ident"
	WebLoopbackZid       = "loopback-zid"
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

// LogEntry stores values of one log line written by a logger.
type LogEntry struct {
	Level   slog.Level
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

const (
	defaultNormalLogLevel = slog.LevelInfo
	defaultSimpleLogLevel = slog.LevelError
)

// Setup sets the most basic data of a software: its name, its version,
// and when the version was created.
func (kern *Kernel) Setup(progname, version string, versionTime time.Time) {
	_ = kern.SetConfig(CoreService, CoreProgname, progname)
	_ = kern.SetConfig(CoreService, CoreVersion, version)
	_ = kern.SetConfig(CoreService, CoreVTime, versionTime.Local().Format(id.TimestampLayout))
}

// Start the service.
func (kern *Kernel) Start(headline, lineServer bool, configFilename string) {
	for _, srvD := range kern.srvs {
		srvD.srv.Freeze()
	}
	if kern.cfg.GetCurConfig(ConfigSimpleMode).(bool) {
		kern.logLevelVar.Set(defaultSimpleLogLevel)
	}
	kern.wg.Add(1)
	signal.Notify(kern.interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		// Wait for interrupt.
		sig := <-kern.interrupt
		if strSig := sig.String(); strSig != "" {
			kern.logger.Info("Shut down Zettelstore", "signal", strSig)
		}
		kern.doShutdown()
		kern.wg.Done()
	}()

	_ = kern.StartService(KernelService)
	if headline {
		logger := kern.logger
		logging.LogMandatory(logger, fmt.Sprintf(
			"%v %v (%v@%v/%v)",
			kern.core.GetCurConfig(CoreProgname),
			kern.core.GetCurConfig(CoreVersion),
			kern.core.GetCurConfig(CoreGoVersion),
			kern.core.GetCurConfig(CoreGoOS),
			kern.core.GetCurConfig(CoreGoArch),
		))
		logging.LogMandatory(logger, "Licensed under the latest version of the EUPL (European Union Public License)")
		if configFilename != "" {
			logging.LogMandatory(logger, "Configuration file found", "filename", configFilename)
		} else {
			logging.LogMandatory(logger, "No configuration file found / used")
		}
		if kern.core.GetCurConfig(CoreDebug).(bool) {
			logger.Info("----------------------------------------")
			logger.Info("DEBUG MODE, DO NO USE THIS IN PRODUCTION")
			logger.Info("----------------------------------------")
		}
		if kern.auth.GetCurConfig(AuthReadonly).(bool) {
			logger.Info("Read-only mode")
		}
	}
	if lineServer {
		port := kern.core.GetNextConfig(CorePort).(int)
		if port > 0 {
			listenAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			_ = startLineServer(kern, listenAddr)
		}
	}
}

func (kern *Kernel) doShutdown() {
	kern.stopService(KernelService) // Will stop all other services.
}

// WaitForShutdown blocks the call until Shutdown is called.
func (kern *Kernel) WaitForShutdown() {
	kern.wg.Wait()
	_ = kern.doStopProfiling()
}

// --- Shutdown operation ----------------------------------------------------

// Shutdown the service. Waits for all concurrent activities to stop.
func (kern *Kernel) Shutdown(silent bool) {
	logging.LogTrace(kern.logger, "Shutdown")
	kern.interrupt <- &shutdownSignal{silent: silent}
}

type shutdownSignal struct{ silent bool }

func (s *shutdownSignal) String() string {
	if s.silent {
		return ""
	}
	return "shutdown"
}
func (*shutdownSignal) Signal() { /* Just a signal */ }

// --- Log operation ---------------------------------------------------------

// GetKernelLogger returns the kernel logger.
func (kern *Kernel) GetKernelLogger() *slog.Logger { return kern.logger }

// GetKernelLogLevel return the logging level of the kernel logger.
func (kern *Kernel) GetKernelLogLevel() slog.Level { return kern.logLevelVar.Level() }

// SetLogLevel sets the logging level for logger maintained by the kernel.
//
// Its syntax is: (SERVICE ":")? LEVEL (";" (SERVICE ":")? LEVEL)*.
func (kern *Kernel) SetLogLevel(logLevel string) {
	defaultLevel, srvLevel := kern.parseLogLevel(logLevel)

	kern.mx.RLock()
	defer kern.mx.RUnlock()

	for srvN, srvD := range kern.srvs {
		if lvl, found := srvLevel[srvN]; found {
			srvD.srv.SetLevel(lvl)
		} else if defaultLevel != logging.LevelMissing {
			srvD.srv.SetLevel(defaultLevel)
		}
	}
}

func (kern *Kernel) parseLogLevel(logLevel string) (slog.Level, map[Service]slog.Level) {
	defaultLevel := logging.LevelMissing
	srvLevel := map[Service]slog.Level{}
	for spec := range strings.SplitSeq(logLevel, ";") {
		vals := cleanLogSpec(strings.Split(spec, ":"))
		switch len(vals) {
		case 0:
		case 1:
			if lvl := logging.ParseLevel(vals[0]); lvl != logging.LevelMissing {
				defaultLevel = lvl
			}
		default:
			serviceText, levelText := vals[0], vals[1]
			if srv, found := kern.srvNames[serviceText]; found {
				if lvl := logging.ParseLevel(levelText); lvl != logging.LevelMissing {
					srvLevel[srv.srvnum] = lvl
				}
			}
		}
	}
	return defaultLevel, srvLevel
}

func cleanLogSpec(rawVals []string) []string {
	vals := make([]string, 0, len(rawVals))
	for _, rVal := range rawVals {
		val := strings.TrimSpace(rVal)
		if val != "" {
			vals = append(vals, val)
		}
	}
	return vals
}

// RetrieveLogEntries returns all buffered log entries.
func (kern *Kernel) RetrieveLogEntries() []LogEntry {
	return kern.dlogWriter.retrieveLogEntries()
}

// GetLastLogTime returns the time when the last logging with level > DEBUG happened.
func (kern *Kernel) GetLastLogTime() time.Time { return kern.dlogWriter.getLastLogTime() }

// LogRecover outputs some information about the previous panic.
func (kern *Kernel) LogRecover(name string, recoverInfo any) {
	stack := debug.Stack()
	kern.logger.Error(name, "recovered_from", recoverInfo, "stack", stack)
	kern.core.updateRecoverInfo(name, recoverInfo, stack)
}

// --- Profiling ---------------------------------------------------------

var errProfileInWork = errors.New("already profiling")
var errProfileNotFound = errors.New("profile not found")

// StartProfiling starts profiling the software according to a profile.
// It is an error to start more than one profile.
//
// profileName is a valid profile (see runtime/pprof/Lookup()), or the
// value "cpu" for profiling the CPI.
// fileName is the name of the file where the results are written to.
func (kern *Kernel) StartProfiling(profileName, fileName string) error {
	kern.mx.Lock()
	defer kern.mx.Unlock()
	return kern.doStartProfiling(profileName, fileName)
}
func (kern *Kernel) doStartProfiling(profileName, fileName string) error {
	if kern.profileName != "" {
		return errProfileInWork
	}
	if profileName == ProfileCPU {
		f, err := os.Create(fileName)
		if err != nil {
			return err
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return err
		}
		kern.profileName = profileName
		kern.fileName = fileName
		kern.profileFile = f
		return nil
	}
	profile := pprof.Lookup(profileName)
	if profile == nil {
		return errProfileNotFound
	}
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	kern.profileName = profileName
	kern.fileName = fileName
	kern.profile = profile
	kern.profileFile = f
	runtime.GC() // get up-to-date statistics
	return profile.WriteTo(f, 0)
}

// StopProfiling stops the current profiling and writes the result to
// the file, which was named during StartProfiling().
// It will always be called before the software stops its operations.
func (kern *Kernel) StopProfiling() error {
	kern.mx.Lock()
	defer kern.mx.Unlock()
	return kern.doStopProfiling()
}
func (kern *Kernel) doStopProfiling() error {
	if kern.profileName == "" {
		return nil // No profile started
	}
	if kern.profileName == ProfileCPU {
		pprof.StopCPUProfile()
	}
	err := kern.profileFile.Close()
	kern.profileName = ""
	kern.fileName = ""
	kern.profile = nil
	kern.profileFile = nil
	return err
}

// --- Service handling --------------------------------------------------

var errUnknownService = errors.New("unknown service")

// SetConfig stores a configuration value.
func (kern *Kernel) SetConfig(srvnum Service, key, value string) error {
	kern.mx.Lock()
	defer kern.mx.Unlock()
	if srvD, ok := kern.srvs[srvnum]; ok {
		return srvD.srv.SetConfig(key, value)
	}
	return errUnknownService
}

// GetConfig returns a configuration value.
func (kern *Kernel) GetConfig(srvnum Service, key string) any {
	kern.mx.RLock()
	defer kern.mx.RUnlock()
	if srvD, ok := kern.srvs[srvnum]; ok {
		return srvD.srv.GetCurConfig(key)
	}
	return nil
}

// GetServiceStatistics returns a key/value list with statistical data.
func (kern *Kernel) GetServiceStatistics(srvnum Service) []KeyValue {
	kern.mx.RLock()
	defer kern.mx.RUnlock()
	if srvD, ok := kern.srvs[srvnum]; ok {
		return srvD.srv.GetStatistics()
	}
	return nil
}

// GetLogger returns a logger for the given service.
func (kern *Kernel) GetLogger(srvnum Service) *slog.Logger {
	kern.mx.RLock()
	defer kern.mx.RUnlock()
	if srvD, ok := kern.srvs[srvnum]; ok {
		return srvD.srv.GetLogger()
	}
	return kern.GetKernelLogger()
}

// StartService start the given service.
func (kern *Kernel) StartService(srvnum Service) error {
	kern.mx.RLock()
	defer kern.mx.RUnlock()
	return kern.doStartService(srvnum)
}
func (kern *Kernel) doStartService(srvnum Service) error {
	for _, srv := range kern.sortDependency(srvnum, kern.depStart, true) {
		if err := srv.Start(kern); err != nil {
			return err
		}
		srv.SwitchNextToCur()
	}
	return nil
}

// restartService stops and restarts the given service, while maintaining service dependencies.
func (kern *Kernel) restartService(srvnum Service) error {
	deps := kern.sortDependency(srvnum, kern.depStop, false)
	for _, srv := range deps {
		srv.Stop(kern)
	}
	for i := len(deps) - 1; i >= 0; i-- {
		srv := deps[i]
		if err := srv.Start(kern); err != nil {
			return err
		}
		srv.SwitchNextToCur()
	}
	return nil
}

// stopService stop the given service.
func (kern *Kernel) stopService(srvnum Service) {
	kern.mx.Lock()
	defer kern.mx.Unlock()
	kern.doStopService(srvnum)
}

func (kern *Kernel) doStopService(srvnum Service) {
	for _, srv := range kern.sortDependency(srvnum, kern.depStop, false) {
		srv.Stop(kern)
	}
}

func (kern *Kernel) sortDependency(
	srvnum Service,
	srvdeps serviceDependency,
	isStarted bool,
) []service {
	srvD, ok := kern.srvs[srvnum]
	if !ok {
		return nil
	}
	if srvD.srv.IsStarted() == isStarted {
		return nil
	}
	deps := srvdeps[srvnum]
	found := make(map[service]bool, 8)
	result := make([]service, 0, len(found))
	for _, dep := range deps {
		srvDeps := kern.sortDependency(dep, srvdeps, isStarted)
		for _, depSrv := range srvDeps {
			if !found[depSrv] {
				result = append(result, depSrv)
				found[depSrv] = true
			}
		}
	}
	return append(result, srvD.srv)
}

// dumpIndex writes some data about the internal index into a writer.
func (kern *Kernel) dumpIndex(w io.Writer) { kern.box.dumpIndex(w) }

type service interface {
	// Initialize the data for the service.
	Initialize(*slog.LevelVar, *slog.Logger)

	// Get service logger.
	GetLogger() *slog.Logger

	// Get the log level.
	GetLevel() slog.Level

	// Set the service log level.
	SetLevel(slog.Level)

	// ConfigDescriptions returns a sorted list of configuration descriptions.
	ConfigDescriptions() []serviceConfigDescription

	// SetConfig stores a configuration value.
	SetConfig(key, value string) error

	// GetCurConfig returns the current configuration value.
	GetCurConfig(key string) any

	// GetNextConfig returns the next configuration value.
	GetNextConfig(key string) any

	// GetCurConfigList returns a sorted list of current configuration data.
	GetCurConfigList(all bool) []KeyDescrValue

	// GetNextConfigList returns a sorted list of next configuration data.
	GetNextConfigList() []KeyDescrValue

	// GetStatistics returns a key/value list of statistical data.
	GetStatistics() []KeyValue

	// Freeze disallows to change some fixed configuration values.
	Freeze()

	// Start the service.
	Start(*Kernel) error

	// SwitchNextToCur moves next config data to current.
	SwitchNextToCur()

	// IsStarted returns true if the service was started successfully.
	IsStarted() bool

	// Stop the service.
	Stop(*Kernel)
}

type serviceConfigDescription struct{ Key, Descr string }

// SetCreators store functions to be called when a service has to be created.
func (kern *Kernel) SetCreators(
	createAuthManager CreateAuthManagerFunc,
	createBoxManager CreateBoxManagerFunc,
	setupWebServer SetupWebServerFunc,
) {
	kern.auth.createManager = createAuthManager
	kern.box.createManager = createBoxManager
	kern.web.setupServer = setupWebServer
}

// --- The kernel as a service -------------------------------------------

type kernelService struct {
	kernel *Kernel
}

func (*kernelService) Initialize(*slog.LevelVar, *slog.Logger) {}

func (ks *kernelService) GetLogger() *slog.Logger { return ks.kernel.logger }
func (ks *kernelService) GetLevel() slog.Level    { return ks.kernel.logLevelVar.Level() }
func (ks *kernelService) SetLevel(lvl slog.Level) { ks.kernel.logLevelVar.Set(lvl) }

func (*kernelService) ConfigDescriptions() []serviceConfigDescription { return nil }
func (*kernelService) SetConfig(string, string) error                 { return errAlreadyFrozen }
func (*kernelService) GetCurConfig(string) any                        { return nil }
func (*kernelService) GetNextConfig(string) any                       { return nil }
func (*kernelService) GetCurConfigList(bool) []KeyDescrValue          { return nil }
func (*kernelService) GetNextConfigList() []KeyDescrValue             { return nil }

func (*kernelService) GetStatistics() []KeyValue { return nil }
func (*kernelService) Freeze()                   {}
func (*kernelService) Start(*Kernel) error       { return nil }
func (*kernelService) SwitchNextToCur()          {}
func (*kernelService) IsStarted() bool           { return true }
func (*kernelService) Stop(*Kernel)              {}
