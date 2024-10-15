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

package impl

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/web/server/impl"
)

type webService struct {
	srvConfig
	mxService   sync.RWMutex
	srvw        server.Server
	setupServer kernel.SetupWebServerFunc
}

var errURLPrefixSyntax = errors.New("must not be empty and must start with '//'")

func (ws *webService) Initialize(logger *logger.Logger) {
	ws.logger = logger
	ws.descr = descriptionMap{
		kernel.WebAssetDir: {
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
		kernel.WebBaseURL: {
			"Base URL",
			func(val string) (any, error) {
				if _, err := url.Parse(val); err != nil {
					return nil, err
				}
				return val, nil
			},
			true,
		},
		kernel.WebListenAddress: {
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
		kernel.WebMaxRequestSize:   {"Max Request Size", parseInt64, true},
		kernel.WebPersistentCookie: {"Persistent cookie", parseBool, true},
		kernel.WebProfiling:        {"Runtime profiling", parseBool, true},
		kernel.WebSecureCookie:     {"Secure cookie", parseBool, true},
		kernel.WebTokenLifetimeAPI: {
			"Token lifetime API",
			makeDurationParser(10*time.Minute, 0, 1*time.Hour),
			true,
		},
		kernel.WebTokenLifetimeHTML: {
			"Token lifetime HTML",
			makeDurationParser(1*time.Hour, 1*time.Minute, 30*24*time.Hour),
			true,
		},
		kernel.WebURLPrefix: {
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
		kernel.WebAssetDir:          "",
		kernel.WebBaseURL:           "http://127.0.0.1:23123/",
		kernel.WebListenAddress:     "127.0.0.1:23123",
		kernel.WebMaxRequestSize:    int64(16 * 1024 * 1024),
		kernel.WebPersistentCookie:  false,
		kernel.WebSecureCookie:      true,
		kernel.WebProfiling:         false,
		kernel.WebTokenLifetimeAPI:  1 * time.Hour,
		kernel.WebTokenLifetimeHTML: 10 * time.Minute,
		kernel.WebURLPrefix:         "/",
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

var errWrongBasePrefix = errors.New(kernel.WebURLPrefix + " does not match " + kernel.WebBaseURL)

func (ws *webService) GetLogger() *logger.Logger { return ws.logger }

func (ws *webService) Start(kern *myKernel) error {
	baseURL := ws.GetNextConfig(kernel.WebBaseURL).(string)
	listenAddr := ws.GetNextConfig(kernel.WebListenAddress).(string)
	urlPrefix := ws.GetNextConfig(kernel.WebURLPrefix).(string)
	persistentCookie := ws.GetNextConfig(kernel.WebPersistentCookie).(bool)
	secureCookie := ws.GetNextConfig(kernel.WebSecureCookie).(bool)
	profile := ws.GetNextConfig(kernel.WebProfiling).(bool)
	maxRequestSize := ws.GetNextConfig(kernel.WebMaxRequestSize).(int64)
	if maxRequestSize < 1024 {
		maxRequestSize = 1024
	}

	if !strings.HasSuffix(baseURL, urlPrefix) {
		ws.logger.Error().Str("base-url", baseURL).Str("url-prefix", urlPrefix).Msg(
			"url-prefix is not a suffix of base-url")
		return errWrongBasePrefix
	}

	if lap := netip.MustParseAddrPort(listenAddr); !kern.auth.manager.WithAuth() && !lap.Addr().IsLoopback() {
		ws.logger.Info().Str("listen", listenAddr).Msg("service may be reached from outside, but authentication is not enabled")
	}

	sd := impl.ServerData{
		Log:              ws.logger,
		ListenAddr:       listenAddr,
		BaseURL:          baseURL,
		URLPrefix:        urlPrefix,
		MaxRequestSize:   maxRequestSize,
		Auth:             kern.auth.manager,
		PersistentCookie: persistentCookie,
		SecureCookie:     secureCookie,
		Profiling:        profile,
		ZidMapper:        kern.box.manager.Mapper(),
	}
	srvw := impl.New(sd)
	err := kern.web.setupServer(srvw, kern.box.manager, kern.auth.manager, &kern.cfg)
	if err != nil {
		ws.logger.Error().Err(err).Msg("Unable to create")
		return err
	}
	if kern.core.GetNextConfig(kernel.CoreDebug).(bool) {
		srvw.SetDebug()
	}
	if err = srvw.Run(); err != nil {
		ws.logger.Error().Err(err).Msg("Unable to start")
		return err
	}
	ws.logger.Info().Str("listen", listenAddr).Str("base-url", baseURL).Msg("Start Service")
	ws.mxService.Lock()
	ws.srvw = srvw
	ws.mxService.Unlock()

	if kern.cfg.GetCurConfig(kernel.ConfigSimpleMode).(bool) {
		listenAddr := ws.GetNextConfig(kernel.WebListenAddress).(string)
		if idx := strings.LastIndexByte(listenAddr, ':'); idx >= 0 {
			ws.logger.Mandatory().Msg(strings.Repeat("--------------------", 3))
			ws.logger.Mandatory().Msg("Open your browser and enter the following URL:")
			ws.logger.Mandatory().Msg("    http://localhost" + listenAddr[idx:])
			ws.logger.Mandatory().Msg("")
			ws.logger.Mandatory().Msg("If this does not work, try:")
			ws.logger.Mandatory().Msg("    http://127.0.0.1" + listenAddr[idx:])
		}
	}

	return nil
}

func (ws *webService) IsStarted() bool {
	ws.mxService.RLock()
	defer ws.mxService.RUnlock()
	return ws.srvw != nil
}

func (ws *webService) Stop(*myKernel) {
	ws.logger.Info().Msg("Stop Service")
	ws.srvw.Stop()
	ws.mxService.Lock()
	ws.srvw = nil
	ws.mxService.Unlock()
}

func (*webService) GetStatistics() []kernel.KeyValue {
	return nil
}
