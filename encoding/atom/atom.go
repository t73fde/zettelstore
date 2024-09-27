//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

// Package atom provides an Atom encoding.
package atom

import (
	"bytes"
	"time"

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/config"
	"zettelstore.de/z/encoding"
	"zettelstore.de/z/encoding/xml"
	"zettelstore.de/z/kernel"
	"zettelstore.de/z/query"
	"zettelstore.de/z/strfun"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// ContentType specifies the HTTP content type for Atom.
const ContentType = "application/atom+xml"

// Configuration contains data to configure the Atom encoding.
type Configuration struct {
	Title            string
	Generator        string
	NewURLBuilderAbs func() *api.URLBuilder
}

// Setup initializes the Configuration.
func (c *Configuration) Setup(cfg config.Config) {
	baseURL := kernel.Main.GetConfig(kernel.WebService, kernel.WebBaseURL).(string)

	c.Title = cfg.GetSiteName()
	c.Generator = (kernel.Main.GetConfig(kernel.CoreService, kernel.CoreProgname).(string) +
		" " +
		kernel.Main.GetConfig(kernel.CoreService, kernel.CoreVersion).(string))
	c.NewURLBuilderAbs = func() *api.URLBuilder { return api.NewURLBuilder(baseURL, 'h') }
}

// Marshal encodes the result of a query as Atom.
func (c *Configuration) Marshal(q *query.Query, ml []*meta.Meta) []byte {
	atomUpdated := encoding.LastUpdated(ml, time.RFC3339)
	feedLink := c.NewURLBuilderAbs().String()

	var buf bytes.Buffer
	buf.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom">` + "\n")
	xml.WriteTag(&buf, "  ", "title", c.Title)
	xml.WriteTag(&buf, "  ", "id", feedLink)
	buf.WriteString(`  <link rel="self" href="`)
	if s := q.String(); s != "" {
		strfun.XMLEscape(&buf, c.NewURLBuilderAbs().AppendQuery(s).String())
	} else {
		strfun.XMLEscape(&buf, feedLink)
	}
	buf.WriteString(`"/>` + "\n")
	if atomUpdated != "" {
		xml.WriteTag(&buf, "  ", "updated", atomUpdated)
	}
	xml.WriteTag(&buf, "  ", "generator", c.Generator)
	buf.WriteString("  <author><name>Unknown</name></author>\n")

	for _, m := range ml {
		c.marshalMeta(&buf, m)
	}

	buf.WriteString("</feed>")
	return buf.Bytes()
}

func (c *Configuration) marshalMeta(buf *bytes.Buffer, m *meta.Meta) {
	entryUpdated := ""
	if val, found := m.Get(api.KeyPublished); found {
		if published, err := time.ParseInLocation(id.TimestampLayout, val, time.Local); err == nil {
			entryUpdated = published.UTC().Format(time.RFC3339)
		}
	}

	link := c.NewURLBuilderAbs().SetZid(m.Zid.ZettelID()).String()

	buf.WriteString("  <entry>\n")
	xml.WriteTag(buf, "    ", "title", encoding.TitleAsText(m))
	xml.WriteTag(buf, "    ", "id", link)
	buf.WriteString(`    <link rel="self" href="`)
	strfun.XMLEscape(buf, link)
	buf.WriteString(`"/>` + "\n")
	buf.WriteString(`    <link rel="alternate" type="text/html" href="`)
	strfun.XMLEscape(buf, link)
	buf.WriteString(`"/>` + "\n")

	if entryUpdated != "" {
		xml.WriteTag(buf, "    ", "updated", entryUpdated)
	}
	marshalTags(buf, m)
	buf.WriteString("  </entry>\n")
}

func marshalTags(buf *bytes.Buffer, m *meta.Meta) {
	if tags, found := m.GetList(api.KeyTags); found && len(tags) > 0 {
		for _, tag := range tags {
			for len(tag) > 0 && tag[0] == '#' {
				tag = tag[1:]
			}
			if tag != "" {
				buf.WriteString(`    <category term="`)
				strfun.XMLEscape(buf, tag)
				buf.WriteString("\"/>\n")
			}
		}
	}
}
