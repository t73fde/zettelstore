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

package client_test

import (
	"context"
	"strings"
	"testing"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/webapi"
)

const (
	abcZid      = id.Zid(20211020121000)
	abc10Zid    = id.Zid(20211020121100)
	abc100Zid   = id.Zid(20211020121145)
	abc1000Zid  = id.Zid(20211020121300)
	abc10000Zid = id.Zid(20211020121400)
)

func TestZettelTransclusion(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")

	content, err := c.GetZettel(context.Background(), abcZid, webapi.PartContent)
	if err != nil {
		t.Error(err)
		return
	}
	expect := string(content)
	for count, zid := range []id.Zid{abc10Zid, abc100Zid, abc1000Zid} {
		expect = strings.Repeat("<span>"+expect+"</span>", 10)
		content, err = c.GetEvaluatedZettel(context.Background(), zid, webapi.EncoderHTML)
		if err != nil {
			t.Error(err)
			continue
		}
		sContent := string(content)
		prefix := "<p>"
		if !strings.HasPrefix(sContent, prefix) {
			t.Errorf("Content of zettel %q does not start with %q: %q", zid, prefix, stringHead(sContent))
			continue
		}
		suffix := "</p>"
		if !strings.HasSuffix(sContent, suffix) {
			t.Errorf("Content of zettel %q does not end with %q: %q", zid, suffix, stringTail(sContent))
			continue
		}
		if got := sContent[len(prefix) : len(content)-len(suffix)]; expect != got {
			t.Errorf("(10^%d) Unexpected content for zettel %q\nExpect: %q\nGot:    %q", count, zid, expect, got)
		}
	}

	content, err = c.GetEvaluatedZettel(context.Background(), abc10000Zid, webapi.EncoderHTML)
	if err != nil {
		t.Error(err)
		return
	}
	checkContentContains(t, abc10000Zid, string(content), "Too many transclusions")
}

func TestZettelTransclusionNoPrivilegeEscalation(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("reader", "reader")

	zettelData, err := c.GetZettelData(context.Background(), id.ZidEmoji)
	if err != nil {
		t.Error(err)
		return
	}
	expectedEnc := "base64"
	if got := zettelData.Encoding; expectedEnc != got {
		t.Errorf("Zettel %q: encoding %q expected, but got %q", abcZid, expectedEnc, got)
	}

	content, err := c.GetEvaluatedZettel(context.Background(), abc10Zid, webapi.EncoderHTML)
	if err != nil {
		t.Error(err)
		return
	}
	if exp, got := "", string(content); exp != got {
		t.Errorf("Zettel %q must contain %q, but got %q", abc10Zid, exp, got)
	}
}

func stringHead(s string) string {
	const maxLen = 40
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func stringTail(s string) string {
	const maxLen = 40
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-maxLen-3:]
}

func TestRecursiveTransclusion(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")

	const (
		selfRecursiveZid      = id.Zid(20211020182600)
		indirectRecursive1Zid = id.Zid(20211020183700)
		indirectRecursive2Zid = id.Zid(20211020183800)
	)
	recursiveZettel := map[id.Zid]id.Zid{
		selfRecursiveZid:      selfRecursiveZid,
		indirectRecursive1Zid: indirectRecursive2Zid,
		indirectRecursive2Zid: indirectRecursive1Zid,
	}
	for zid, errZid := range recursiveZettel {
		content, err := c.GetEvaluatedZettel(context.Background(), zid, webapi.EncoderHTML)
		if err != nil {
			t.Error(err)
			continue
		}
		sContent := string(content)
		checkContentContains(t, zid, sContent, "Recursive transclusion")
		checkContentContains(t, zid, sContent, errZid.String())
	}
}
func TestNothingToTransclude(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")

	const (
		transZid = id.Zid(20211020184342)
		emptyZid = id.Zid(20211020184300)
	)
	content, err := c.GetEvaluatedZettel(context.Background(), transZid, webapi.EncoderHTML)
	if err != nil {
		t.Error(err)
		return
	}
	sContent := string(content)
	checkContentContains(t, transZid, sContent, "<!-- Nothing to transclude")
	checkContentContains(t, transZid, sContent, emptyZid.String())
}

func TestSelfEmbedRef(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")

	const selfEmbedZid = id.Zid(20211020185400)
	content, err := c.GetEvaluatedZettel(context.Background(), selfEmbedZid, webapi.EncoderHTML)
	if err != nil {
		t.Error(err)
		return
	}
	checkContentContains(t, selfEmbedZid, string(content), "Self embed reference")
}

func checkContentContains(t *testing.T, zid id.Zid, content, expected string) {
	if !strings.Contains(content, expected) {
		t.Helper()
		t.Errorf("Zettel %q should contain %q, but does not: %q", zid, expected, content)
	}
}
