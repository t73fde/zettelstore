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
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/box"
	"zettelstore.de/z/config"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/logger"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

type configService struct {
	srvConfig
	mxService sync.RWMutex
	orig      *meta.Meta
	manager   box.Manager
}

// Predefined Metadata keys for runtime configuration
// See: https://zettelstore.de/manual/h/00001004020000
const (
	keyDefaultCopyright  = "default-copyright"
	keyDefaultLicense    = "default-license"
	keyDefaultVisibility = "default-visibility"
	keyExpertMode        = "expert-mode"
	keyMaxTransclusions  = "max-transclusions"
	keySiteName          = "site-name"
	keyYAMLHeader        = "yaml-header"
	keyZettelFileSyntax  = "zettel-file-syntax"
)

var errUnknownVisibility = errors.New("unknown visibility")

func (cs *configService) Initialize(logger *logger.Logger) {
	cs.logger = logger
	cs.descr = descriptionMap{
		keyDefaultCopyright: {"Default copyright", parseString, true},
		keyDefaultLicense:   {"Default license", parseString, true},
		keyDefaultVisibility: {
			"Default zettel visibility",
			func(val string) (any, error) {
				vis := meta.GetVisibility(val)
				if vis == meta.VisibilityUnknown {
					return nil, errUnknownVisibility
				}
				return vis, nil
			},
			true,
		},
		keyExpertMode:          {"Expert mode", parseBool, true},
		config.KeyFooterZettel: {"Footer Zettel", parseInvalidZid, true},
		config.KeyHomeZettel:   {"Home zettel", parseZid, true},
		kernel.ConfigInsecureHTML: {
			"Insecure HTML",
			cs.noFrozen(func(val string) (any, error) {
				switch val {
				case kernel.ConfigSyntaxHTML:
					return config.SyntaxHTML, nil
				case kernel.ConfigMarkdownHTML:
					return config.MarkdownHTML, nil
				case kernel.ConfigZmkHTML:
					return config.ZettelmarkupHTML, nil
				}
				return config.NoHTML, nil
			}),
			true,
		},
		api.KeyLang:         {"Language", parseString, true},
		keyMaxTransclusions: {"Maximum transclusions", parseInt64, true},
		keySiteName:         {"Site name", parseString, true},
		keyYAMLHeader:       {"YAML header", parseBool, true},
		keyZettelFileSyntax: {
			"Zettel file syntax",
			func(val string) (any, error) { return strings.Fields(val), nil },
			true,
		},
		kernel.ConfigSimpleMode:        {"Simple mode", cs.noFrozen(parseBool), true},
		config.KeyListsMenuZettel:      {"Lists menu", parseZid, true},
		config.KeyShowBackLinks:        {"Show back links", parseString, true},
		config.KeyShowFolgeLinks:       {"Show folge links", parseString, true},
		config.KeyShowSequelLinks:      {"Show sequel links", parseString, true},
		config.KeyShowSubordinateLinks: {"Show subordinate links", parseString, true},
		config.KeyShowSuccessorLinks:   {"Show successor links", parseString, true},
	}
	cs.next = interfaceMap{
		keyDefaultCopyright:            "",
		keyDefaultLicense:              "",
		keyDefaultVisibility:           meta.VisibilityLogin,
		keyExpertMode:                  false,
		config.KeyFooterZettel:         id.Invalid,
		config.KeyHomeZettel:           id.DefaultHomeZid,
		kernel.ConfigInsecureHTML:      config.NoHTML,
		api.KeyLang:                    api.ValueLangEN,
		keyMaxTransclusions:            int64(1024),
		keySiteName:                    "Zettelstore",
		keyYAMLHeader:                  false,
		keyZettelFileSyntax:            nil,
		kernel.ConfigSimpleMode:        false,
		config.KeyListsMenuZettel:      api.ZidTOCListsMenu,
		config.KeyShowBackLinks:        "",
		config.KeyShowFolgeLinks:       "",
		config.KeyShowSequelLinks:      "",
		config.KeyShowSubordinateLinks: "",
		config.KeyShowSuccessorLinks:   "",
	}
}
func (cs *configService) GetLogger() *logger.Logger { return cs.logger }

func (cs *configService) Start(*myKernel) error {
	cs.logger.Info().Msg("Start Service")
	data := meta.New(id.ConfigurationZid)
	for _, kv := range cs.GetNextConfigList() {
		data.Set(kv.Key, kv.Value)
	}
	cs.mxService.Lock()
	cs.orig = data
	cs.mxService.Unlock()
	return nil
}

func (cs *configService) IsStarted() bool {
	cs.mxService.RLock()
	defer cs.mxService.RUnlock()
	return cs.orig != nil
}

func (cs *configService) Stop(*myKernel) {
	cs.logger.Info().Msg("Stop Service")
	cs.mxService.Lock()
	cs.orig = nil
	cs.manager = nil
	cs.mxService.Unlock()
}

func (*configService) GetStatistics() []kernel.KeyValue {
	return nil
}

func (cs *configService) setBox(mgr box.Manager) {
	cs.mxService.Lock()
	cs.manager = mgr
	cs.mxService.Unlock()
	mgr.RegisterObserver(cs.observe)
	cs.observe(box.UpdateInfo{Box: mgr, Reason: box.OnZettel, Zid: id.ConfigurationZid})
}

func (cs *configService) doUpdate(p box.BaseBox) error {
	z, err := p.GetZettel(context.Background(), id.ConfigurationZid)
	cs.logger.Trace().Err(err).Msg("got config meta")
	if err != nil {
		return err
	}
	m := z.Meta
	cs.mxService.Lock()
	for _, pair := range cs.orig.Pairs() {
		key := pair.Key
		if val, ok := m.Get(key); ok {
			cs.SetConfig(key, val)
		} else if defVal, defFound := cs.orig.Get(key); defFound {
			cs.SetConfig(key, defVal)
		}
	}
	cs.mxService.Unlock()
	cs.SwitchNextToCur() // Poor man's restart
	return nil
}

func (cs *configService) observe(ci box.UpdateInfo) {
	if (ci.Reason != box.OnZettel && ci.Reason != box.OnDelete) || ci.Zid == id.ConfigurationZid {
		cs.logger.Debug().Uint("reason", uint64(ci.Reason)).Zid(ci.Zid).Msg("observe")
		go func() {
			cs.mxService.RLock()
			mgr := cs.manager
			cs.mxService.RUnlock()
			if mgr != nil {
				cs.doUpdate(mgr)
			} else {
				cs.doUpdate(ci.Box)
			}
		}()
	}
}

// --- config.Config

func (cs *configService) Get(ctx context.Context, m *meta.Meta, key string) string {
	if m != nil {
		if val, found := m.Get(key); found {
			return val
		}
	}
	if user := server.GetUser(ctx); user != nil {
		if val, found := user.Get(key); found {
			return val
		}
	}
	result := cs.GetCurConfig(key)
	if result == nil {
		return ""
	}
	switch val := result.(type) {
	case string:
		return val
	case bool:
		if val {
			return api.ValueTrue
		}
		return api.ValueFalse
	case id.Zid:
		return val.String()
	case int:
		return strconv.Itoa(val)
	case []string:
		return strings.Join(val, " ")
	case meta.Visibility:
		return val.String()
	case fmt.Stringer:
		return val.String()
	case fmt.GoStringer:
		return val.GoString()
	}
	return fmt.Sprintf("%v", result)
}

// AddDefaultValues enriches the given meta data with its default values.
func (cs *configService) AddDefaultValues(ctx context.Context, m *meta.Meta) *meta.Meta {
	if cs == nil {
		return m
	}
	result := m
	cs.mxService.RLock()
	if _, found := m.Get(api.KeyCopyright); !found {
		result = updateMeta(result, m, api.KeyCopyright, cs.GetCurConfig(keyDefaultCopyright).(string))
	}
	if _, found := m.Get(api.KeyLang); !found {
		result = updateMeta(result, m, api.KeyLang, cs.Get(ctx, nil, api.KeyLang))
	}
	if _, found := m.Get(api.KeyLicense); !found {
		result = updateMeta(result, m, api.KeyLicense, cs.GetCurConfig(keyDefaultLicense).(string))
	}
	if _, found := m.Get(api.KeyVisibility); !found {
		result = updateMeta(result, m, api.KeyVisibility, cs.GetCurConfig(keyDefaultVisibility).(meta.Visibility).String())
	}
	cs.mxService.RUnlock()
	return result
}
func updateMeta(result, m *meta.Meta, key, val string) *meta.Meta {
	if result == m {
		result = m.Clone()
	}
	result.Set(key, val)
	return result
}

func (cs *configService) GetHTMLInsecurity() config.HTMLInsecurity {
	return cs.GetCurConfig(kernel.ConfigInsecureHTML).(config.HTMLInsecurity)
}

// GetSiteName returns the current value of the "site-name" key.
func (cs *configService) GetSiteName() string { return cs.GetCurConfig(keySiteName).(string) }

// GetMaxTransclusions return the maximum number of indirect transclusions.
func (cs *configService) GetMaxTransclusions() int {
	return int(cs.GetCurConfig(keyMaxTransclusions).(int64))
}

// GetYAMLHeader returns the current value of the "yaml-header" key.
func (cs *configService) GetYAMLHeader() bool { return cs.GetCurConfig(keyYAMLHeader).(bool) }

// GetZettelFileSyntax returns the current value of the "zettel-file-syntax" key.
func (cs *configService) GetZettelFileSyntax() []string {
	if zfs := cs.GetCurConfig(keyZettelFileSyntax); zfs != nil {
		return zfs.([]string)
	}
	return nil
}

// --- config.AuthConfig

// GetSimpleMode returns true if system tuns in simple-mode.
func (cs *configService) GetSimpleMode() bool { return cs.GetCurConfig(kernel.ConfigSimpleMode).(bool) }

// GetExpertMode returns the current value of the "expert-mode" key.
func (cs *configService) GetExpertMode() bool { return cs.GetCurConfig(keyExpertMode).(bool) }

// GetVisibility returns the visibility value, or "login" if none is given.
func (cs *configService) GetVisibility(m *meta.Meta) meta.Visibility {
	if val, ok := m.Get(api.KeyVisibility); ok {
		if vis := meta.GetVisibility(val); vis != meta.VisibilityUnknown {
			return vis
		}
	}

	vis := cs.GetCurConfig(keyDefaultVisibility).(meta.Visibility)
	if vis != meta.VisibilityUnknown {
		return vis
	}
	cs.mxService.RLock()
	val, _ := cs.orig.Get(keyDefaultVisibility)
	vis = meta.GetVisibility(val)
	cs.mxService.RUnlock()
	return vis
}
