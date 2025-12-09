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

package kernel

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"t73f.de/r/zsc/domain/id"

	"zettelstore.de/z/internal/logging"
	"zettelstore.de/z/internal/web/server"
)

type webService struct {
	srvConfig
	mxService   sync.RWMutex
	srvw        server.Server
	setupServer SetupWebServerFunc
}

var errURLPrefixSyntax = errors.New("must not be empty and must start with '//'")

func (ws *webService) Initialize(levelVar *slog.LevelVar, logger *slog.Logger) {
	ws.logLevelVar = levelVar
	ws.logger = logger
	ws.descr = descriptionMap{
		WebAssetDir: {
			"Asset file  directory",
			func(val string) (any, error) {
				val = filepath.Clean(val)
				finfo, err := os.Stat(val)
				if err == nil && finfo.IsDir() {
					return val, nil
				}
				return nil, err
			},
			true,
		},
		WebBaseURL: {
			"Base URL",
			func(val string) (any, error) {
				if _, err := url.Parse(val); err != nil {
					return nil, err
				}
				return val, nil
			},
			true,
		},
		WebListenAddress: {
			"Listen address",
			func(val string) (any, error) {
				// If there is no host, prepend 127.0.0.1 as host.
				host, _, err := net.SplitHostPort(val)
				if err == nil && host == "" {
					val = "127.0.0.1" + val
				}
				ap, err := netip.ParseAddrPort(val)
				if err != nil {
					return "", err
				}
				return ap.String(), nil
			},
			true},
		WebLoopbackIdent:    {"Loopback user ident", ws.noFrozen(parseString), true},
		WebLoopbackZid:      {"Loopback user zettel identifier", ws.noFrozen(parseInvalidZid), true},
		WebMaxRequestSize:   {"Max Request Size", parseInt64, true},
		WebPersistentCookie: {"Persistent cookie", parseBool, true},
		WebProfiling:        {"Runtime profiling", parseBool, true},
		WebSecureCookie:     {"Secure cookie", parseBool, true},
		WebSxMaxNesting:     {"Max nesting of Sx calls", parseInt, true},
		WebTokenLifetimeAPI: {
			"Token lifetime API",
			makeDurationParser(10*time.Minute, 0, 1*time.Hour),
			true,
		},
		WebTokenLifetimeHTML: {
			"Token lifetime HTML",
			makeDurationParser(1*time.Hour, 1*time.Minute, 30*24*time.Hour),
			true,
		},
		WebURLPrefix: {
			"URL prefix under which the web server runs",
			func(val string) (any, error) {
				if val != "" && val[0] == '/' && val[len(val)-1] == '/' {
					return val, nil
				}
				return nil, errURLPrefixSyntax
			},
			true,
		},
	}
	ws.next = interfaceMap{
		WebAssetDir:          "",
		WebBaseURL:           "http://127.0.0.1:23123/",
		WebListenAddress:     "127.0.0.1:23123",
		WebLoopbackIdent:     "",
		WebLoopbackZid:       id.Invalid,
		WebMaxRequestSize:    int64(16 * 1024 * 1024),
		WebPersistentCookie:  false,
		WebProfiling:         false,
		WebSecureCookie:      true,
		WebSxMaxNesting:      32 * 1024,
		WebTokenLifetimeAPI:  10 * time.Minute,
		WebTokenLifetimeHTML: 60 * time.Minute,
		WebURLPrefix:         "/",
	}
}

func makeDurationParser(defDur, minDur, maxDur time.Duration) parseFunc {
	return func(val string) (any, error) {
		if d, err := strconv.ParseUint(val, 10, 64); err == nil {
			secs := time.Duration(d) * time.Minute
			if secs < minDur {
				return minDur, nil
			}
			if secs > maxDur {
				return maxDur, nil
			}
			return secs, nil
		}
		return defDur, nil
	}
}

var errWrongBasePrefix = errors.New(WebURLPrefix + " does not match " + WebBaseURL)

func (ws *webService) GetLogger() *slog.Logger { return ws.logger }
func (ws *webService) GetLevel() slog.Level    { return ws.logLevelVar.Level() }
func (ws *webService) SetLevel(l slog.Level)   { ws.logLevelVar.Set(l) }

func (ws *webService) Start(kern *Kernel) error {
	baseURL := ws.GetNextConfig(WebBaseURL).(string)
	listenAddr := ws.GetNextConfig(WebListenAddress).(string)
	loopbackIdent := ws.GetNextConfig(WebLoopbackIdent).(string)
	loopbackZid := ws.GetNextConfig(WebLoopbackZid).(id.Zid)
	urlPrefix := ws.GetNextConfig(WebURLPrefix).(string)
	persistentCookie := ws.GetNextConfig(WebPersistentCookie).(bool)
	secureCookie := ws.GetNextConfig(WebSecureCookie).(bool)
	profile := ws.GetNextConfig(WebProfiling).(bool)
	maxRequestSize := max(ws.GetNextConfig(WebMaxRequestSize).(int64), 1024)

	if !strings.HasSuffix(baseURL, urlPrefix) {
		ws.logger.Error("url-prefix is not a suffix of base-url", "base-url", baseURL, "url-prefix", urlPrefix)
		return errWrongBasePrefix
	}

	if lap := netip.MustParseAddrPort(listenAddr); !kern.auth.manager.WithAuth() && !lap.Addr().IsLoopback() {
		ws.logger.Info("service may be reached from outside, but authentication is not enabled", "listen", listenAddr)
	}

	sd := server.ConfigData{
		Log:              ws.logger,
		ListenAddr:       listenAddr,
		BaseURL:          baseURL,
		URLPrefix:        urlPrefix,
		MaxRequestSize:   maxRequestSize,
		Auth:             kern.auth.manager,
		LoopbackIdent:    loopbackIdent,
		LoopbackZid:      loopbackZid,
		PersistentCookie: persistentCookie,
		SecureCookie:     secureCookie,
		Profiling:        profile,
	}
	srvw := server.New(sd)
	err := kern.web.setupServer(srvw, kern.box.manager, kern.auth.manager, &kern.cfg)
	if err != nil {
		ws.logger.Error("Unable to create", "err", err)
		return err
	}
	if err = srvw.Run(); err != nil {
		ws.logger.Error("Unable to start", "err", err)
		return err
	}
	ws.logger.Info("Start Service", "listen", listenAddr, "base-url", baseURL)
	ws.mxService.Lock()
	ws.srvw = srvw
	ws.mxService.Unlock()

	if kern.cfg.GetCurConfig(ConfigSimpleMode).(bool) {
		listenAddr := ws.GetNextConfig(WebListenAddress).(string)
		if idx := strings.LastIndexByte(listenAddr, ':'); idx >= 0 {
			logging.LogMandatory(ws.logger, strings.Repeat("--------------------", 3))
			logging.LogMandatory(ws.logger, "Open your browser and enter the following URL:")
			logging.LogMandatory(ws.logger, "    http://localhost"+listenAddr[idx:])
			logging.LogMandatory(ws.logger, "")
			logging.LogMandatory(ws.logger, "If this does not work, try:")
			logging.LogMandatory(ws.logger, "    http://127.0.0.1"+listenAddr[idx:])
		}
	}

	return nil
}

func (ws *webService) IsStarted() bool {
	ws.mxService.RLock()
	defer ws.mxService.RUnlock()
	return ws.srvw != nil
}

func (ws *webService) Stop(*Kernel) {
	ws.logger.Info("Stop Service")
	ws.srvw.Stop()
	ws.mxService.Lock()
	ws.srvw = nil
	ws.mxService.Unlock()
}

func (*webService) GetStatistics() []KeyValue {
	return nil
}
