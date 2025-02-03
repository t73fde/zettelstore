//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore client is licensed under the latest version of the EUPL
// (European Union Public License). Please see file LICENSE.txt for your rights
// and obligations under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package client_test

import (
	"context"
	"strings"
	"testing"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/client"
	"t73f.de/r/zsc/domain/id"
)

// ---------------------------------------------------------------------------
// Tests that change the Zettelstore must nor run parallel to other tests.

func TestCreateGetDeleteZettel(t *testing.T) {
	// Is not to be allowed to run in parallel with other tests.
	zettel := `title: A Test

Example content.`
	c := getClient()
	c.SetAuth("owner", "owner")
	zid, err := c.CreateZettel(context.Background(), []byte(zettel))
	if err != nil {
		t.Error("Cannot create zettel:", err)
		return
	}
	if !zid.IsValid() {
		t.Error("Invalid zettel ID", zid)
		return
	}
	data, err := c.GetZettel(context.Background(), zid, api.PartZettel)
	if err != nil {
		t.Error("Cannot read zettel", zid, err)
		return
	}
	exp := `title: A Test

Example content.`
	if string(data) != exp {
		t.Errorf("Expected zettel data: %q, but got %q", exp, data)
	}

	doDelete(t, c, zid)
}

func TestCreateGetDeleteZettelDataCreator(t *testing.T) {
	// Is not to be allowed to run in parallel with other tests.
	c := getClient()
	c.SetAuth("creator", "creator")
	zid, err := c.CreateZettelData(context.Background(), api.ZettelData{
		Meta:     nil,
		Encoding: "",
		Content:  "Example",
	})
	if err != nil {
		t.Error("Cannot create zettel:", err)
		return
	}
	if !zid.IsValid() {
		t.Error("Invalid zettel ID", zid)
		return
	}

	c.SetAuth("owner", "owner")
	doDelete(t, c, zid)
}

func TestCreateGetDeleteZettelData(t *testing.T) {
	// Is not to be allowed to run in parallel with other tests.
	c := getClient()
	c.SetAuth("owner", "owner")
	wrongModified := "19691231115959"
	zid, err := c.CreateZettelData(context.Background(), api.ZettelData{
		Meta: api.ZettelMeta{
			api.KeyTitle:    "A\nTitle", // \n must be converted into a space
			api.KeyModified: wrongModified,
		},
	})
	if err != nil {
		t.Error("Cannot create zettel:", err)
		return
	}
	z, err := c.GetZettelData(context.Background(), zid)
	if err != nil {
		t.Error("Cannot get zettel:", zid, err)
	} else {
		exp := "A Title"
		if got := z.Meta[api.KeyTitle]; got != exp {
			t.Errorf("Expected title %q, but got %q", exp, got)
		}
		if got := z.Meta[api.KeyModified]; got != "" {
			t.Errorf("Create allowed to set the modified key: %q", got)
		}
	}
	doDelete(t, c, zid)
}

func TestUpdateZettel(t *testing.T) {
	c := getClient()
	c.SetAuth("owner", "owner")
	z, err := c.GetZettel(context.Background(), api.ZidDefaultHome, api.PartZettel)
	if err != nil {
		t.Error(err)
		return
	}
	if !strings.HasPrefix(string(z), "title: Home\n") {
		t.Error("Got unexpected zettel", z)
		return
	}
	newZettel := `title: Empty Home
role: zettel
syntax: zmk

Empty`
	err = c.UpdateZettel(context.Background(), api.ZidDefaultHome, []byte(newZettel))
	if err != nil {
		t.Error(err)
		return
	}
	zt, err := c.GetZettel(context.Background(), api.ZidDefaultHome, api.PartZettel)
	if err != nil {
		t.Error(err)
		return
	}
	if string(zt) != newZettel {
		t.Errorf("Expected zettel %q, got %q", newZettel, zt)
	}
	// Must delete to clean up for next tests
	doDelete(t, c, api.ZidDefaultHome)
}

func TestUpdateZettelData(t *testing.T) {
	c := getClient()
	c.SetAuth("writer", "writer")
	z, err := c.GetZettelData(context.Background(), api.ZidDefaultHome)
	if err != nil {
		t.Error(err)
		return
	}
	if got := z.Meta[api.KeyTitle]; got != "Home" {
		t.Errorf("Title of zettel is not \"Home\", but %q", got)
		return
	}
	newTitle := "New Home"
	z.Meta[api.KeyTitle] = newTitle
	wrongModified := "19691231235959"
	z.Meta[api.KeyModified] = wrongModified
	err = c.UpdateZettelData(context.Background(), api.ZidDefaultHome, z)
	if err != nil {
		t.Error(err)
		return
	}
	zt, err := c.GetZettelData(context.Background(), api.ZidDefaultHome)
	if err != nil {
		t.Error(err)
		return
	}
	if got := zt.Meta[api.KeyTitle]; got != newTitle {
		t.Errorf("Title of zettel is not %q, but %q", newTitle, got)
	}
	if got := zt.Meta[api.KeyModified]; got == wrongModified {
		t.Errorf("Update did not change the modified key: %q", got)
	}

	// Must delete to clean up for next tests
	c.SetAuth("owner", "owner")
	doDelete(t, c, api.ZidDefaultHome)
}

func doDelete(t *testing.T, c *client.Client, zid id.Zid) {
	err := c.DeleteZettel(context.Background(), zid)
	if err != nil {
		t.Helper()
		t.Error("Cannot delete", zid, ":", err)
	}
}
