//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package client_test

import (
	"context"
	"strings"
	"testing"

	"zettelstore.de/c/api"
)

const (
	abcZid   = api.ZettelID("20211020121000")
	abc10Zid = api.ZettelID("20211020121100")
)

func TestZettelTransclusion(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")

	const abc10000Zid = api.ZettelID("20211020121400")
	contentMap := map[api.ZettelID]int{
		abcZid:                         1,
		abc10Zid:                       10,
		api.ZettelID("20211020121145"): 100,
		api.ZettelID("20211020121300"): 1000,
	}
	content, err := c.GetZettel(context.Background(), abcZid, api.PartContent)
	if err != nil {
		t.Error(err)
		return
	}
	baseContent := string(content)
	for zid, siz := range contentMap {
		content, err = c.GetEvaluatedZettel(context.Background(), zid, api.EncoderHTML)
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
		got := sContent[len(prefix) : len(content)-len(suffix)]
		if expect := strings.Repeat(baseContent, siz); expect != got {
			t.Errorf("Unexpected content for zettel %q\nExpect: %q\nGot:    %q", zid, expect, got)
		}
	}

	content, err = c.GetEvaluatedZettel(context.Background(), abc10000Zid, api.EncoderHTML)
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

	zettelData, err := c.GetZettelData(context.Background(), api.ZidEmoji)
	if err != nil {
		t.Error(err)
		return
	}
	expectedEnc := "base64"
	if got := zettelData.Encoding; expectedEnc != got {
		t.Errorf("Zettel %q: encoding %q expected, but got %q", abcZid, expectedEnc, got)
	}

	content, err := c.GetEvaluatedZettel(context.Background(), abc10Zid, api.EncoderHTML)
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
		selfRecursiveZid      = api.ZettelID("20211020182600")
		indirectRecursive1Zid = api.ZettelID("20211020183700")
		indirectRecursive2Zid = api.ZettelID("20211020183800")
	)
	recursiveZettel := map[api.ZettelID]api.ZettelID{
		selfRecursiveZid:      selfRecursiveZid,
		indirectRecursive1Zid: indirectRecursive2Zid,
		indirectRecursive2Zid: indirectRecursive1Zid,
	}
	for zid, errZid := range recursiveZettel {
		content, err := c.GetEvaluatedZettel(context.Background(), zid, api.EncoderHTML)
		if err != nil {
			t.Error(err)
			continue
		}
		sContent := string(content)
		checkContentContains(t, zid, sContent, "Recursive transclusion")
		checkContentContains(t, zid, sContent, string(errZid))
	}
}
func TestNothingToTransclude(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")

	const (
		transZid = api.ZettelID("20211020184342")
		emptyZid = api.ZettelID("20211020184300")
	)
	content, err := c.GetEvaluatedZettel(context.Background(), transZid, api.EncoderHTML)
	if err != nil {
		t.Error(err)
		return
	}
	sContent := string(content)
	checkContentContains(t, transZid, sContent, "<!-- Nothing to transclude")
	checkContentContains(t, transZid, sContent, string(emptyZid))
}

func TestSelfEmbedRef(t *testing.T) {
	t.Parallel()
	c := getClient()
	c.SetAuth("owner", "owner")

	const selfEmbedZid = api.ZettelID("20211020185400")
	content, err := c.GetEvaluatedZettel(context.Background(), selfEmbedZid, api.EncoderHTML)
	if err != nil {
		t.Error(err)
		return
	}
	checkContentContains(t, selfEmbedZid, string(content), "Self embed reference")
}

func checkContentContains(t *testing.T, zid api.ZettelID, content, expected string) {
	if !strings.Contains(content, expected) {
		t.Helper()
		t.Errorf("Zettel %q should contain %q, but does not: %q", zid, expected, content)
	}
}
