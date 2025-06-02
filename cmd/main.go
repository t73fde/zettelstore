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

// Package cmd provides the commands to call Zettelstore from the command line.
package cmd

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/auth/impl"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/box/compbox"
	"zettelstore.de/z/internal/box/manager"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/web/server"
)

const strRunSimple = "run-simple"

func init() {
	RegisterCommand(Command{
		Name: "help",
		Func: func(*flag.FlagSet) (int, error) {
			fmt.Println("Available commands:")
			for _, name := range List() {
				fmt.Printf("- %q\n", name)
			}
			return 0, nil
		},
	})
	RegisterCommand(Command{
		Name:   "version",
		Func:   func(*flag.FlagSet) (int, error) { return 0, nil },
		Header: true,
	})
	RegisterCommand(Command{
		Name:       "run",
		Func:       runFunc,
		Boxes:      true,
		Header:     true,
		LineServer: true,
		SetFlags:   flgRun,
	})
	RegisterCommand(Command{
		Name:   strRunSimple,
		Func:   runFunc,
		Simple: true,
		Boxes:  true,
		Header: true,
		// LineServer: true,
		SetFlags: func(fs *flag.FlagSet) {
			// fs.Uint("a", 0, "port number kernel service (0=disable)")
			fs.String("d", "", "zettel directory")
		},
	})
	RegisterCommand(Command{
		Name: "file",
		Func: cmdFile,
		SetFlags: func(fs *flag.FlagSet) {
			fs.String("t", api.EncoderHTML.String(), "target output encoding")
		},
	})
	RegisterCommand(Command{
		Name: "password",
		Func: cmdPassword,
	})
}

func fetchStartupConfiguration(fs *flag.FlagSet) (string, *meta.Meta) {
	if configFlag := fs.Lookup("c"); configFlag != nil {
		if filename := configFlag.Value.String(); filename != "" {
			content, err := readConfiguration(filename)
			return filename, createConfiguration(content, err)
		}
	}
	filename, content, err := searchAndReadConfiguration()
	return filename, createConfiguration(content, err)
}

func createConfiguration(content []byte, err error) *meta.Meta {
	if err != nil {
		return meta.New(id.Invalid)
	}
	return meta.NewFromInput(id.Invalid, input.NewInput(content))
}

func readConfiguration(filename string) ([]byte, error) { return os.ReadFile(filename) }

func searchAndReadConfiguration() (string, []byte, error) {
	for _, filename := range []string{"zettelstore.cfg", "zsconfig.txt", "zscfg.txt", "_zscfg", ".zscfg"} {
		if content, err := readConfiguration(filename); err == nil {
			return filename, content, nil
		}
	}
	return "", nil, os.ErrNotExist
}

func getConfig(fs *flag.FlagSet) (string, *meta.Meta) {
	filename, cfg := fetchStartupConfiguration(fs)
	fs.Visit(func(flg *flag.Flag) {
		switch flg.Name {
		case "p":
			cfg.Set(keyListenAddr, meta.Value(net.JoinHostPort("127.0.0.1", flg.Value.String())))
		case "a":
			cfg.Set(keyAdminPort, meta.Value(flg.Value.String()))
		case "d":
			val := flg.Value.String()
			if strings.HasPrefix(val, "/") {
				val = "dir://" + val
			} else {
				val = "dir:" + val
			}
			deleteConfiguredBoxes(cfg)
			cfg.Set(keyBoxOneURI, meta.Value(val))
		case "l":
			cfg.Set(keyLogLevel, meta.Value(flg.Value.String()))
		case "debug":
			cfg.Set(keyDebug, meta.Value(flg.Value.String()))
		case "r":
			cfg.Set(keyReadOnly, meta.Value(flg.Value.String()))
		case "v":
			cfg.Set(keyVerbose, meta.Value(flg.Value.String()))
		}
	})
	return filename, cfg
}

func deleteConfiguredBoxes(cfg *meta.Meta) {
	for key := range cfg.Rest() {
		if strings.HasPrefix(key, kernel.BoxURIs) {
			cfg.Delete(key)
		}
	}
}

const (
	keyAdminPort         = "admin-port"
	keyAssetDir          = "asset-dir"
	keyBaseURL           = "base-url"
	keyBoxOneURI         = kernel.BoxURIs + "1"
	keyDebug             = "debug-mode"
	keyDefaultDirBoxType = "default-dir-box-type"
	keyInsecureCookie    = "insecure-cookie"
	keyInsecureHTML      = "insecure-html"
	keyListenAddr        = "listen-addr"
	keyLogLevel          = "log-level"
	keyLoopbackIdent     = "loopback-ident"
	keyLoopbackZid       = "loopback-zid"
	keyMaxRequestSize    = "max-request-size"
	keyOwner             = "owner"
	keyPersistentCookie  = "persistent-cookie"
	keyReadOnly          = "read-only-mode"
	keyRuntimeProfiling  = "runtime-profiling"
	keyTokenLifetimeHTML = "token-lifetime-html"
	keyTokenLifetimeAPI  = "token-lifetime-api"
	keyURLPrefix         = "url-prefix"
	keyVerbose           = "verbose-mode"
)

func setServiceConfig(cfg *meta.Meta) bool {
	debugMode := cfg.GetBool(keyDebug)
	if debugMode && kernel.Main.GetKernelLogLevel() > slog.LevelDebug {
		kernel.Main.SetLogLevel(logging.LevelString(slog.LevelDebug))
	}
	if logLevel, found := cfg.Get(keyLogLevel); found {
		kernel.Main.SetLogLevel(string(logLevel))
	}
	err := setConfigValue(nil, kernel.CoreService, kernel.CoreDebug, debugMode)
	err = setConfigValue(err, kernel.CoreService, kernel.CoreVerbose, cfg.GetBool(keyVerbose))
	if val, found := cfg.Get(keyAdminPort); found {
		err = setConfigValue(err, kernel.CoreService, kernel.CorePort, val)
	}

	err = setConfigValue(err, kernel.AuthService, kernel.AuthOwner, cfg.GetDefault(keyOwner, ""))
	err = setConfigValue(err, kernel.AuthService, kernel.AuthReadonly, cfg.GetBool(keyReadOnly))

	err = setConfigValue(
		err, kernel.BoxService, kernel.BoxDefaultDirType,
		cfg.GetDefault(keyDefaultDirBoxType, kernel.BoxDirTypeNotify))
	err = setConfigValue(err, kernel.BoxService, kernel.BoxURIs+"1", "dir:./zettel")
	for i := 1; ; i++ {
		key := kernel.BoxURIs + strconv.Itoa(i)
		val, found := cfg.Get(key)
		if !found {
			break
		}
		err = setConfigValue(err, kernel.BoxService, key, val)
	}

	err = setConfigValue(
		err, kernel.ConfigService, kernel.ConfigInsecureHTML, cfg.GetDefault(keyInsecureHTML, kernel.ConfigSecureHTML))

	err = setConfigValue(
		err, kernel.WebService, kernel.WebListenAddress, cfg.GetDefault(keyListenAddr, "127.0.0.1:23123"))
	err = setConfigValue(err, kernel.WebService, kernel.WebLoopbackIdent, cfg.GetDefault(keyLoopbackIdent, ""))
	err = setConfigValue(err, kernel.WebService, kernel.WebLoopbackZid, cfg.GetDefault(keyLoopbackZid, ""))
	if val, found := cfg.Get(keyBaseURL); found {
		err = setConfigValue(err, kernel.WebService, kernel.WebBaseURL, val)
	}
	if val, found := cfg.Get(keyURLPrefix); found {
		err = setConfigValue(err, kernel.WebService, kernel.WebURLPrefix, val)
	}
	err = setConfigValue(err, kernel.WebService, kernel.WebSecureCookie, !cfg.GetBool(keyInsecureCookie))
	err = setConfigValue(err, kernel.WebService, kernel.WebPersistentCookie, cfg.GetBool(keyPersistentCookie))
	if val, found := cfg.Get(keyMaxRequestSize); found {
		err = setConfigValue(err, kernel.WebService, kernel.WebMaxRequestSize, val)
	}
	err = setConfigValue(
		err, kernel.WebService, kernel.WebTokenLifetimeAPI, cfg.GetDefault(keyTokenLifetimeAPI, ""))
	err = setConfigValue(
		err, kernel.WebService, kernel.WebTokenLifetimeHTML, cfg.GetDefault(keyTokenLifetimeHTML, ""))
	err = setConfigValue(err, kernel.WebService, kernel.WebProfiling, debugMode || cfg.GetBool(keyRuntimeProfiling))
	if val, found := cfg.Get(keyAssetDir); found {
		err = setConfigValue(err, kernel.WebService, kernel.WebAssetDir, val)
	}
	return err == nil
}

func setConfigValue(err error, subsys kernel.Service, key string, val any) error {
	if err == nil {
		if err = kernel.Main.SetConfig(subsys, key, fmt.Sprint(val)); err != nil {
			kernel.Main.GetKernelLogger().Error("Unable to set configuration",
				"key", key, "value", val, "err", err)
		}
	}
	return err
}

func executeCommand(name string, args ...string) int {
	command, ok := Get(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown command %q\n", name)
		return 1
	}
	fs := command.GetFlags()
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: unable to parse flags: %v %v\n", name, args, err)
		return 1
	}
	filename, cfg := getConfig(fs)
	if !setServiceConfig(cfg) {
		fs.Usage()
		return 2
	}

	kern := kernel.Main
	var createManager kernel.CreateBoxManagerFunc
	if command.Boxes {
		createManager = func(boxURIs []*url.URL, authManager auth.Manager, rtConfig config.Config) (box.Manager, error) {
			compbox.Setup(cfg)
			return manager.New(boxURIs, authManager, rtConfig)
		}
	} else {
		createManager = func([]*url.URL, auth.Manager, config.Config) (box.Manager, error) { return nil, nil }
	}

	secret := cfg.GetDefault("secret", "")
	if len(secret) < 16 && cfg.GetDefault(keyOwner, "") != "" {
		fmt.Fprintf(os.Stderr, "secret must have at least length 16 when authentication is enabled, but is %q\n", secret)
		return 2
	}
	cfg.Delete("secret")
	secretHash := fmt.Sprintf("%x", sha256.Sum256([]byte(string(secret))))

	kern.SetCreators(
		func(readonly bool, owner id.Zid) (auth.Manager, error) {
			return impl.New(readonly, owner, secretHash), nil
		},
		createManager,
		func(srv server.Server, plMgr box.Manager, authMgr auth.Manager, rtConfig config.Config) error {
			setupRouting(srv, plMgr, authMgr, rtConfig)
			return nil
		},
	)

	if command.Simple {
		if err := kern.SetConfig(kernel.ConfigService, kernel.ConfigSimpleMode, "true"); err != nil {
			kern.GetKernelLogger().Error("unable to set simple-mode", "err", err)
			return 1
		}
	}
	kern.Start(command.Header, command.LineServer, filename)
	exitCode, err := command.Func(fs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
	}
	kern.Shutdown(true)
	return exitCode
}

// runSimple is called, when the user just starts the software via a double click
// or via a simple call “./zettelstore“ on the command line.
func runSimple() int {
	if _, _, err := searchAndReadConfiguration(); err == nil {
		return executeCommand(strRunSimple)
	}
	dir := "./zettel"
	if err := os.MkdirAll(dir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create zettel directory %q (%s)\n", dir, err)
		return 1
	}
	return executeCommand(strRunSimple, "-d", dir)
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

// Main is the real entrypoint of the zettelstore.
func Main(progName, buildVersion string) int {
	info := retrieveVCSInfo(buildVersion)
	fullVersion := info.revision
	if info.dirty {
		fullVersion += "-dirty"
	}
	kernel.Main.Setup(progName, fullVersion, info.time)
	flag.Parse()
	if *cpuprofile != "" || *memprofile != "" {
		var err error
		if *cpuprofile != "" {
			err = kernel.Main.StartProfiling(kernel.ProfileCPU, *cpuprofile)
		} else {
			err = kernel.Main.StartProfiling(kernel.ProfileHead, *memprofile)
		}
		if err != nil {
			kernel.Main.GetKernelLogger().Error("start profiling", "err", err)
			return 1
		}
		defer func() {
			if err = kernel.Main.StopProfiling(); err != nil {
				kernel.Main.GetKernelLogger().Error("stop profiling", "err", err)
			}
		}()
	}
	args := flag.Args()
	if len(args) == 0 {
		return runSimple()
	}
	return executeCommand(args[0], args[1:]...)
}

type vcsInfo struct {
	revision string
	dirty    bool
	time     time.Time
}

func retrieveVCSInfo(version string) vcsInfo {
	buildTime := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return vcsInfo{revision: version, dirty: false, time: buildTime}
	}
	result := vcsInfo{time: buildTime}
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			revision := "+" + kv.Value
			if len(revision) > 11 {
				revision = revision[:11]
			}
			result.revision = version + revision
		case "vcs.modified":
			if kv.Value == "true" {
				result.dirty = true
			}
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, kv.Value); err == nil {
				result.time = t
			}
		}
	}
	return result
}
