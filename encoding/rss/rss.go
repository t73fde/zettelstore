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

// Package rss provides a RSS encoding.
package rss

import (
	"bytes"
	"context"
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

// ContentType specifies the HTTP content type for RSS.
const ContentType = "application/rss+xml"

// Configuration contains data to configure the RSS encoding.
type Configuration struct {
	Title            string
	Language         string
	Copyright        string
	Generator        string
	NewURLBuilderAbs func() *api.URLBuilder
}

// Setup initializes the Configuration.
func (c *Configuration) Setup(ctx context.Context, cfg config.Config) {
	baseURL := kernel.Main.GetConfig(kernel.WebService, kernel.WebBaseURL).(string)
	defVals := cfg.AddDefaultValues(ctx, &meta.Meta{})

	c.Title = cfg.GetSiteName()
	c.Language = defVals.GetDefault(api.KeyLang, "")
	c.Copyright = defVals.GetDefault(api.KeyCopyright, "")
	c.Generator = (kernel.Main.GetConfig(kernel.CoreService, kernel.CoreProgname).(string) +
		" " +
		kernel.Main.GetConfig(kernel.CoreService, kernel.CoreVersion).(string))
	c.NewURLBuilderAbs = func() *api.URLBuilder { return api.NewURLBuilder(baseURL, 'h') }
}

// Marshal encodes the result of a query as Atom.
func (c *Configuration) Marshal(q *query.Query, ml []*meta.Meta) []byte {
	rssPublished := encoding.LastUpdated(ml, time.RFC1123Z)

	atomLink := ""
	if s := q.String(); s != "" {
		atomLink = c.NewURLBuilderAbs().AppendQuery(s).String()
	}
	var buf bytes.Buffer
	buf.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">` + "\n<channel>\n")
	xml.WriteTag(&buf, "  ", "title", c.Title)
	xml.WriteTag(&buf, "  ", "link", c.NewURLBuilderAbs().String())
	xml.WriteTag(&buf, "  ", "description", "")
	xml.WriteTag(&buf, "  ", "language", c.Language)
	xml.WriteTag(&buf, "  ", "copyright", c.Copyright)
	if rssPublished != "" {
		xml.WriteTag(&buf, "  ", "pubDate", rssPublished)
		xml.WriteTag(&buf, "  ", "lastBuildDate", rssPublished)
	}
	xml.WriteTag(&buf, "  ", "generator", c.Generator)
	buf.WriteString("  <docs>https://www.rssboard.org/rss-specification</docs>\n")
	if atomLink != "" {
		buf.WriteString(`  <atom:link href="`)
		strfun.XMLEscape(&buf, atomLink)
		buf.WriteString(`" rel="self" type="application/rss+xml"></atom:link>` + "\n")
	}
	for _, m := range ml {
		c.marshalMeta(&buf, m)
	}

	buf.WriteString("</channel>\n</rss>")
	return buf.Bytes()
}

func (c *Configuration) marshalMeta(buf *bytes.Buffer, m *meta.Meta) {
	itemPublished := ""
	if val, found := m.Get(api.KeyPublished); found {
		if published, err := time.ParseInLocation(id.TimestampLayout, val, time.Local); err == nil {
			itemPublished = published.UTC().Format(time.RFC1123Z)
		}
	}

	link := c.NewURLBuilderAbs().SetZid(m.Zid.ZettelID()).String()

	buf.WriteString("  <item>\n")
	xml.WriteTag(buf, "    ", "title", encoding.TitleAsText(m))
	xml.WriteTag(buf, "    ", "link", link)
	xml.WriteTag(buf, "    ", "guid", link)
	if itemPublished != "" {
		xml.WriteTag(buf, "    ", "pubDate", itemPublished)
	}
	marshalTags(buf, m)
	buf.WriteString("  </item>\n")
}

func marshalTags(buf *bytes.Buffer, m *meta.Meta) {
	if tags, found := m.GetList(api.KeyTags); found && len(tags) > 0 {
		for _, tag := range tags {
			for len(tag) > 0 && tag[0] == '#' {
				tag = tag[1:]
			}
			if tag != "" {
				xml.WriteTag(buf, "    ", "category", tag)
			}
		}
	}
}
