//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// Zettelstore is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with Zettelstore. If not, see <http://www.gnu.org/licenses/>.
//-----------------------------------------------------------------------------

// Package config provides functions to retrieve configuration data.
package config

import (
	"zettelstore.de/z/domain"
	"zettelstore.de/z/store"
	"zettelstore.de/z/store/stock"
)

// Setup enables the configuration package.
func Setup(store store.Store) {
	if configStock != nil {
		panic("configStock already set")
	}
	configStock = stock.NewStock(store)
	if err := configStock.Subscribe(domain.ConfigurationID); err != nil {
		panic(err)
	}
}

var configStock stock.Stock

// Config is the global configuration object.
var Config configType

type configType struct{}

// getConfigurationMeta returns the meta data of the configuration zettel.
func getConfigurationMeta() *domain.Meta {
	if configStock == nil {
		panic("configStock not set")
	}
	return configStock.GetMeta(domain.ConfigurationID)
}

// GetDefaultTitle returns the current value of the "default-title" key.
func (c configType) GetDefaultTitle() string {
	if config := getConfigurationMeta(); config != nil {
		return config.GetDefault(domain.MetaKeyDefaultTitle, domain.MetaValueTitle)
	}
	return domain.MetaValueTitle
}

// GetDefaultSyntax returns the current value of the "default-syntax" key.
func (c configType) GetDefaultSyntax() string {
	if configStock != nil {
		if config := getConfigurationMeta(); config != nil {
			return config.GetDefault(domain.MetaKeyDefaultSyntax, domain.MetaValueSyntax)
		}
	}
	return domain.MetaValueSyntax
}

// GetDefaultRole returns the current value of the "default-role" key.
func (c configType) GetDefaultRole() string {
	if configStock != nil {
		if config := getConfigurationMeta(); config != nil {
			return config.GetDefault(domain.MetaKeyDefaultRole, domain.MetaValueRole)
		}
	}
	return domain.MetaValueRole
}

// GetDefaultLang returns the current value of the "default-lang" key.
func (c configType) GetDefaultLang() string {
	if config := getConfigurationMeta(); config != nil {
		return config.GetDefault(domain.MetaKeyDefaultLang, domain.MetaValueLang)
	}
	return domain.MetaValueLang
}

var defIconMaterial = "<img class=\"zs-text-icon\" src=\"/c/" + string(domain.MaterialIconID) + "\">"

// GetIconMaterial returns the current value of the "icon-material" key.
func (c configType) GetIconMaterial() string {
	if config := getConfigurationMeta(); config != nil {
		return config.GetDefault(domain.MetaKeyIconMaterial, defIconMaterial)
	}
	return defIconMaterial
}

// GetSiteName returns the current value of the "site-name" key.
func (c configType) GetSiteName() string {
	if config := getConfigurationMeta(); config != nil {
		return config.GetDefault(domain.MetaKeySiteName, domain.MetaValueSiteName)
	}
	return domain.MetaValueSiteName
}

// GetYAMLHeader returns the current value of the "yaml-header" key.
func (c configType) GetYAMLHeader() bool {
	if config := getConfigurationMeta(); config != nil {
		return config.GetBool(domain.MetaKeyYAMLHeader)
	}
	return false
}

// GetZettelFileSyntax returns the current value of the "zettel-file-syntax" key.
func (c configType) GetZettelFileSyntax() []string {
	if config := getConfigurationMeta(); config != nil {
		return config.GetListOrNil(domain.MetaKeyZettelFileSyntax)
	}
	return nil
}
