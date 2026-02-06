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

// Package client provides a client for accessing the Zettelstore via its API.
package client_test

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"testing"

	"t73f.de/r/zsc/client"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/webapi"
	"zettelstore.de/z/internal/kernel"
)

func nextZid(zid id.Zid) id.Zid { return zid + 1 }

func TestNextZid(t *testing.T) {
	testCases := []struct {
		zid, exp id.Zid
	}{
		{1, 2},
	}
	for i, tc := range testCases {
		if got := nextZid(tc.zid); got != tc.exp {
			t.Errorf("%d: zid=%q, exp=%q, got=%q", i, tc.zid, tc.exp, got)
		}

	}
}

func TestListZettel(t *testing.T) {
	const (
		ownerZettel      = 60
		configRoleZettel = 38
		writerZettel     = ownerZettel - 24
		readerZettel     = ownerZettel - 24
		creatorZettel    = 13
		publicZettel     = 8
	)

	testdata := []struct {
		user string
		exp  int
	}{
		{"", publicZettel},
		{"creator", creatorZettel},
		{"reader", readerZettel},
		{"writer", writerZettel},
		{"owner", ownerZettel},
	}

	t.Parallel()
	c := getClient()
	for i, tc := range testdata {
		t.Run(fmt.Sprintf("User %d/%q", i, tc.user), func(tt *testing.T) {
			c.SetAuth(tc.user, tc.user)
			q, h, l, err := c.QueryZettelData(context.Background(), "")
			if err != nil {
				tt.Error(err)
				return
			}
			if q != "" {
				tt.Errorf("Query should be empty, but is %q", q)
			}
			if h != "" {
				tt.Errorf("Human should be empty, but is %q", q)
			}
			got := len(l)
			if got != tc.exp {
				tt.Errorf("List of length %d expected, but got %d\n%v", tc.exp, got, l)
			}
		})
	}
	search := meta.KeyRole + webapi.SearchOperatorHas + meta.ValueRoleConfiguration + " ORDER id"
	q, h, l, err := c.QueryZettelData(context.Background(), search)
	if err != nil {
		t.Error(err)
		return
	}
	expQ := "role:configuration ORDER id"
	if q != expQ {
		t.Errorf("Query should be %q, but is %q", expQ, q)
	}
	expH := "role HAS configuration ORDER id"
	if h != expH {
		t.Errorf("Human should be %q, but is %q", expH, h)
	}
	got := len(l)
	if got != configRoleZettel {
		t.Errorf("List of length %d expected, but got %d\n%v", configRoleZettel, got, l)
	}

	pl, err := c.QueryZettel(context.Background(), search)
	if err != nil {
		t.Error(err)
		return
	}
	compareZettelList(t, pl, l)
}

func compareZettelList(t *testing.T, pl [][]byte, l []webapi.ZidMetaRights) {
	t.Helper()
	if len(pl) != len(l) {
		t.Errorf("Different list length: Plain=%d, Data=%d", len(pl), len(l))
	} else {
		for i, line := range pl {
			got, err := id.Parse(string(line[:14]))
			if err == nil && got != l[i].ID {
				t.Errorf("%d: Data=%q, got=%q", i, l[i].ID, got)
			}
		}
	}
}

func TestGetZettelData(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")
	z, err := c.GetZettelData(context.Background(), id.ZidDefaultHome)
	if err != nil {
		t.Error(err)
		return
	}
	if m := z.Meta; len(m) == 0 {
		t.Errorf("Exptected non-empty meta, but got %v", z.Meta)
	}
	if z.Content == "" || z.Encoding != "" {
		t.Errorf("Expect non-empty content, but empty encoding (got %q)", z.Encoding)
	}

	mr, err := c.GetMetaData(context.Background(), id.ZidDefaultHome)
	if err != nil {
		t.Error(err)
		return
	}
	if mr.Rights == webapi.ZettelCanNone {
		t.Error("rights must be greater zero")
	}
	if len(mr.Meta) != len(z.Meta) {
		t.Errorf("Pure meta differs from zettel meta: %s vs %s", mr.Meta, z.Meta)
		return
	}
	for k, v := range z.Meta {
		got, ok := mr.Meta[k]
		if !ok {
			t.Errorf("Pure meta has no key %q", k)
			continue
		}
		if got != v {
			t.Errorf("Pure meta has different value for key %q: %q vs %q", k, got, v)
		}
	}
}

func TestGetParsedEvaluatedZettel(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")
	encodings := []webapi.EncodingEnum{
		webapi.EncoderHTML,
		webapi.EncoderSz,
		webapi.EncoderText,
	}
	for _, enc := range encodings {
		content, err := c.GetParsedZettel(context.Background(), id.ZidDefaultHome, enc)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("Empty content for parsed encoding %v", enc)
		}
		content, err = c.GetEvaluatedZettel(context.Background(), id.ZidDefaultHome, enc)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("Empty content for evaluated encoding %v", enc)
		}
	}
}

func checkListZid(t *testing.T, l []webapi.ZidMetaRights, pos int, expected id.Zid) {
	t.Helper()
	if got := l[pos].ID; got != expected {
		t.Errorf("Expected result[%d]=%v, but got %v", pos, expected, got)
	}
}

func TestGetZettelOrder(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")
	_, _, metaSeq, err := c.QueryZettelData(context.Background(), id.ZidTOCNewTemplate.String()+" "+webapi.ItemsDirective)
	if err != nil {
		t.Error(err)
		return
	}
	if got := len(metaSeq); got != 4 {
		t.Errorf("Expected list of length 4, got %d", got)
		return
	}
	checkListZid(t, metaSeq, 0, id.ZidTemplateNewZettel)
	checkListZid(t, metaSeq, 1, id.ZidTemplateNewRole)
	checkListZid(t, metaSeq, 2, id.ZidTemplateNewTag)
	checkListZid(t, metaSeq, 3, id.ZidTemplateNewUser)
}

func TestGetZettelContext(t *testing.T) {
	const (
		allUserZid = id.Zid(20211019200500)
		ownerZid   = id.Zid(20210629163300)
		writerZid  = id.Zid(20210629165000)
		readerZid  = id.Zid(20210629165024)
		creatorZid = id.Zid(20210629165050)
		limitAll   = 3
	)
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")
	rl, err := c.QueryZettel(context.Background(), ownerZid.String()+" CONTEXT LIMIT "+strconv.Itoa(limitAll))
	if err != nil {
		t.Error(err)
		return
	}
	checkZidList(t, []id.Zid{ownerZid, allUserZid, writerZid}, rl)

	rl, err = c.QueryZettel(context.Background(), ownerZid.String()+" CONTEXT BACKWARD")
	if err != nil {
		t.Error(err)
		return
	}
	checkZidList(t, []id.Zid{ownerZid, allUserZid}, rl)
}
func checkZidList(t *testing.T, exp []id.Zid, got [][]byte) {
	t.Helper()
	if len(exp) != len(got) {
		t.Errorf("expected a list fo length %d, but got %d", len(exp), len(got))
		return
	}
	for i, expZid := range exp {
		gotZid, err := id.Parse(string(got[i][:14]))
		if err != nil || expZid != gotZid {
			t.Errorf("lists differ at pos %d: expected id %v, but got %v", i, expZid, gotZid)
		}
	}
}

func TestGetUnlinkedReferences(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")
	_, _, metaSeq, err := c.QueryZettelData(context.Background(), id.ZidDefaultHome.String()+" "+webapi.UnlinkedDirective)
	if err != nil {
		t.Error(err)
		return
	}
	if got := len(metaSeq); got != 1 {
		t.Errorf("Expected list of length 1, got %d:\n%v", got, metaSeq)
		return
	}
}

func TestGetReferences(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")
	urls, err := c.GetReferences(context.Background(), id.ZidDefaultHome, "zettel")
	if err != nil {
		t.Error(err)
		return
	}
	exp := []string{
		"https://zettelstore.de/",
		"https://zettelstore.de/home/doc/trunk/www/download.wiki",
		"https://zettelstore.de/home/doc/trunk/www/changes.wiki",
		"mailto:ds@zettelstore.de",
	}
	if !slices.Equal(urls, exp) {
		t.Errorf("wrong references of home zettel: expected\n%v, but got\n%v", exp, urls)
	}
}

func TestExecuteCommand(t *testing.T) {
	c := getClient()
	err := c.ExecuteCommand(context.Background(), webapi.Command("xyz"))
	failNoErrorOrNoCode(t, err, http.StatusBadRequest)
	err = c.ExecuteCommand(context.Background(), webapi.CommandAuthenticated)
	failNoErrorOrNoCode(t, err, http.StatusUnauthorized)
	err = c.ExecuteCommand(context.Background(), webapi.CommandRefresh)
	failNoErrorOrNoCode(t, err, http.StatusForbidden)

	c.SetAuth("owner", "owner")
	err = c.ExecuteCommand(context.Background(), webapi.CommandAuthenticated)
	if err != nil {
		t.Error(err)
	}
	err = c.ExecuteCommand(context.Background(), webapi.CommandRefresh)
	if err != nil {
		t.Error(err)
	}
}
func failNoErrorOrNoCode(t *testing.T, err error, goodCode int) {
	if err != nil {
		if cErr, ok := err.(*client.Error); ok {
			if cErr.StatusCode == goodCode {
				return
			}
			t.Errorf("Expect status code %d, but got client error %v", goodCode, cErr)
		} else {
			t.Errorf("Expect status code %d, but got non-client error %v", goodCode, err)
		}
	} else {
		t.Errorf("No error returned, but status code %d expected", goodCode)
	}
}

func TestListTags(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")
	agg, err := c.QueryAggregate(context.Background(), webapi.ActionSeparator+meta.KeyTags)
	if err != nil {
		t.Error(err)
		return
	}
	tags := []struct {
		key  string
		size int
	}{
		{"#invisible", 1},
		{"#user", 4},
		{"#test", 4},
	}
	if len(agg) != len(tags) {
		t.Errorf("Expected %d different tags, but got %d (%v)", len(tags), len(agg), agg)
	}
	for _, tag := range tags {
		if zl, ok := agg[tag.key]; !ok {
			t.Errorf("No tag %v: %v", tag.key, agg)
		} else if len(zl) != tag.size {
			t.Errorf("Expected %d zettel with tag %v, but got %v", tag.size, tag.key, zl)
		}
	}
	for i, id := range agg["#user"] {
		if id != agg["#test"][i] {
			t.Errorf("Tags #user and #test have different content: %v vs %v", agg["#user"], agg["#test"])
		}
	}
}

func TestTagZettel(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.AllowRedirect(true)
	c.SetAuth("owner", "owner")
	ctx := context.Background()
	zid, err := c.TagZettel(ctx, "nosuchtag")
	if err != nil {
		t.Error(err)
	} else if zid != id.Invalid {
		t.Errorf("no zid expected, but got %q", zid)
	}
	zid, err = c.TagZettel(ctx, "#test")
	exp := id.Zid(20230929102100)
	if err != nil {
		t.Error(err)
	} else if zid != exp {
		t.Errorf("tag zettel for #test should be %q, but got %q", exp, zid)
	}
}

func TestListRoles(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")
	agg, err := c.QueryAggregate(context.Background(), webapi.ActionSeparator+meta.KeyRole)
	if err != nil {
		t.Error(err)
		return
	}
	exp := []string{"configuration", "role", "user", "tag", "zettel"}
	if len(agg) != len(exp) {
		t.Errorf("Expected %d different roles, but got %d (%v)", len(exp), len(agg), agg)
	}
	for _, id := range exp {
		if _, found := agg[id]; !found {
			t.Errorf("Role map expected key %q", id)
		}
	}
}

func TestRoleZettel(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.AllowRedirect(true)
	c.SetAuth("owner", "owner")
	ctx := context.Background()
	zid, err := c.RoleZettel(ctx, "nosuchrole")
	if err != nil {
		t.Error("AAA", err)
	} else if zid != id.Invalid {
		t.Errorf("no zid expected, but got %q", zid)
	}
	zid, err = c.RoleZettel(ctx, "zettel")
	exp := id.Zid(60010)
	if err != nil {
		t.Error(err)
	} else if zid != exp {
		t.Errorf("role zettel for zettel should be %q, but got %q", exp, zid)
	}
}

func TestRedirect(t *testing.T) {
	t.Parallel()
	c := getClient()
	search := "emoji" + webapi.ActionSeparator + webapi.RedirectAction
	ub := c.NewURLBuilder('z').AppendQuery(search)
	respRedirect, err := http.Get(ub.String())
	if err != nil {
		t.Error(err)
		return
	}
	defer func() { _ = respRedirect.Body.Close() }()
	bodyRedirect, err := io.ReadAll(respRedirect.Body)
	if err != nil {
		t.Error(err)
		return
	}
	ub.ClearQuery().SetZid(id.ZidEmoji)
	respEmoji, err := http.Get(ub.String())
	if err != nil {
		t.Error(err)
		return
	}
	defer func() { _ = respEmoji.Body.Close() }()
	bodyEmoji, err := io.ReadAll(respEmoji.Body)
	if err != nil {
		t.Error(err)
		return
	}
	if !slices.Equal(bodyRedirect, bodyEmoji) {
		t.Error("Wrong redirect")
		t.Error("REDIRECT", respRedirect)
		t.Error("EXPECTED", respEmoji)
	}
}

func TestVersion(t *testing.T) {
	t.Parallel()
	c := getClient()
	ver, err := c.GetVersionInfo(context.Background())
	if err != nil {
		t.Error(err)
		return
	}
	if ver.Major != -1 || ver.Minor != -1 || ver.Patch != -1 || ver.Info != kernel.CoreDefaultVersion || ver.Hash != "" {
		t.Error(ver)
	}
}

func TestApplicationZid(t *testing.T) {
	c := getClient()
	c.SetAuth("reader", "reader")
	zid, err := c.GetApplicationZid(context.Background(), "app")
	if err != nil {
		t.Error(err)
		return
	}
	if zid != id.ZidAppDirectory {
		t.Errorf("c.GetApplicationZid(\"app\") should result in %q, but got: %q", id.ZidAppDirectory, zid)
	}
	if zid, err = c.GetApplicationZid(context.Background(), "noappzid"); err == nil {
		t.Errorf(`c.GetApplicationZid("nozid") should result in error, but got: %v`, zid)
	}
	if zid, err = c.GetApplicationZid(context.Background(), "nozid"); err == nil {
		t.Errorf(`c.GetApplicationZid("nozid") should result in error, but got: %v`, zid)
	}
}

var baseURL string

func init() {
	flag.StringVar(&baseURL, "base-url", "", "Base URL")
}

func getClient() *client.Client {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	return client.NewClient(u)
}

// TestMain controls whether client API tests should run or not.
func TestMain(m *testing.M) {
	flag.Parse()
	if baseURL != "" {
		m.Run()
	}
}
