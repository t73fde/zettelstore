//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

// Package startup provides functions to retrieve startup configuration data.
package startup

import (
	"hash/fnv"
	"io"
	"strconv"
	"time"

	"zettelstore.de/z/domain/id"
	"zettelstore.de/z/domain/meta"
	"zettelstore.de/z/place"
)

var config struct {
	simple        bool // was started without run command
	verbose       bool
	readonlyMode  bool
	urlPrefix     string
	listenAddress string
	owner         id.Zid
	withAuth      bool
	secret        []byte
	insecCookie   bool
	persistCookie bool
	htmlLifetime  time.Duration
	apiLifetime   time.Duration
	manager       place.Manager
}

// Predefined keys for startup zettel
const (
	KeyInsecureCookie    = "insecure-cookie"
	KeyListenAddress     = "listen-addr"
	KeyOwner             = "owner"
	KeyPersistentCookie  = "persistent-cookie"
	KeyPlaceOneURI       = "place-1-uri"
	KeyReadOnlyMode      = "read-only-mode"
	KeyTokenLifetimeHTML = "token-lifetime-html"
	KeyTokenLifetimeAPI  = "token-lifetime-api"
	KeyURLPrefix         = "url-prefix"
	KeyVerbose           = "verbose"
)

// SetupStartup initializes the startup data.
func SetupStartup(cfg *meta.Meta, manager place.Manager, simple bool) error {
	if config.urlPrefix != "" {
		panic("startup.config already set")
	}
	config.simple = simple
	config.verbose = cfg.GetBool(KeyVerbose)
	config.readonlyMode = cfg.GetBool(KeyReadOnlyMode)
	config.urlPrefix = cfg.GetDefault(KeyURLPrefix, "/")
	if prefix, ok := cfg.Get(KeyURLPrefix); ok &&
		len(prefix) > 0 && prefix[0] == '/' && prefix[len(prefix)-1] == '/' {
		config.urlPrefix = prefix
	} else {
		config.urlPrefix = "/"
	}
	if val, ok := cfg.Get(KeyListenAddress); ok {
		config.listenAddress = val // TODO: check for valid string
	} else {
		config.listenAddress = "127.0.0.1:23123"
	}
	config.owner = id.Invalid
	if owner, ok := cfg.Get(KeyOwner); ok {
		if zid, err := id.Parse(owner); err == nil {
			config.owner = zid
			config.withAuth = true
		}
	}
	if config.withAuth {
		config.insecCookie = cfg.GetBool(KeyInsecureCookie)
		config.persistCookie = cfg.GetBool(KeyPersistentCookie)
		config.secret = calcSecret(cfg)
		config.htmlLifetime = getDuration(
			cfg, KeyTokenLifetimeHTML, 1*time.Hour, 1*time.Minute, 30*24*time.Hour)
		config.apiLifetime = getDuration(
			cfg, KeyTokenLifetimeAPI, 10*time.Minute, 0, 1*time.Hour)
	}
	config.simple = simple && !config.withAuth
	config.manager = manager
	return nil
}

func calcSecret(cfg *meta.Meta) []byte {
	h := fnv.New128()
	if secret, ok := cfg.Get("secret"); ok {
		io.WriteString(h, secret)
	}
	io.WriteString(h, version.Prog)
	io.WriteString(h, version.Build)
	io.WriteString(h, version.Hostname)
	io.WriteString(h, version.GoVersion)
	io.WriteString(h, version.Os)
	io.WriteString(h, version.Arch)
	return h.Sum(nil)
}

func getDuration(
	cfg *meta.Meta, key string, defDur, minDur, maxDur time.Duration) time.Duration {
	if s, ok := cfg.Get(key); ok && len(s) > 0 {
		if d, err := strconv.ParseUint(s, 10, 64); err == nil {
			secs := time.Duration(d) * time.Minute
			if secs < minDur {
				return minDur
			}
			if secs > maxDur {
				return maxDur
			}
			return secs
		}
	}
	return defDur
}

// IsSimple returns true if Zettelstore was not started with command "run"
// and authentication is disabled.
func IsSimple() bool { return config.simple }

// IsVerbose returns whether the system should be more chatty about its operations.
func IsVerbose() bool { return config.verbose }

// IsReadOnlyMode returns whether the system is in read-only mode or not.
func IsReadOnlyMode() bool { return config.readonlyMode }

// URLPrefix returns the configured prefix to be used when providing URL to
// the service.
func URLPrefix() string { return config.urlPrefix }

// ListenAddress returns the string that specifies the the network card and the ip port
// where the server listens for requests
func ListenAddress() string { return config.listenAddress }

// WithAuth returns true if user authentication is enabled.
func WithAuth() bool { return config.withAuth }

// SecureCookie returns whether the web app should set cookies to secure mode.
func SecureCookie() bool { return config.withAuth && !config.insecCookie }

// PersistentCookie returns whether the web app should set persistent cookies
// (instead of temporary).
func PersistentCookie() bool { return config.persistCookie }

// Owner returns the zid of the zettelkasten's owner.
// If there is no owner defined, the value ZettelID(0) is returned.
func Owner() id.Zid { return config.owner }

// IsOwner returns true, if the given user is the owner of the Zettelstore.
func IsOwner(zid id.Zid) bool { return zid.IsValid() && zid == config.owner }

// Secret returns the interal application secret. It is typically used to
// encrypt session values.
func Secret() []byte { return config.secret }

// TokenLifetime return the token lifetime for the web/HTML access and for the
// API access. If lifetime for API access is equal to zero, no API access is
// possible.
func TokenLifetime() (htmlLifetime, apiLifetime time.Duration) {
	return config.htmlLifetime, config.apiLifetime
}

// PlaceManager returns the managing place.
func PlaceManager() place.Manager { return config.manager }
