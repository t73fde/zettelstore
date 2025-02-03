//-----------------------------------------------------------------------------
// Copyright (c) 2023-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2023-present Detlef Stern
//-----------------------------------------------------------------------------

// Package main provides a tool to check the validity of HTML zettel documents.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/client"
	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/tools"
)

func main() {
	flag.BoolVar(&tools.Verbose, "v", false, "Verbose output")
	flag.Parse()

	if err := cmdValidateHTML(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
func cmdValidateHTML(args []string) error {
	rawURL := "http://localhost:23123"
	if len(args) > 0 {
		rawURL = args[0]
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	client := client.NewClient(u)
	_, _, metaList, err := client.QueryZettelData(context.Background(), "")
	if err != nil {
		return err
	}
	zids, perm := calculateZids(metaList)
	for _, kd := range keyDescr {
		msgCount := 0
		fmt.Fprintf(os.Stderr, "Now checking: %s\n", kd.text)
		for _, zid := range zidsToUse(zids, perm, kd.sampleSize) {
			var nmsgs int
			nmsgs, err = validateHTML(client, kd.uc, zid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "* error while validating zettel %v with: %v\n", zid, err)
				msgCount++
			} else {
				msgCount += nmsgs
			}
		}
		if msgCount == 1 {
			fmt.Fprintln(os.Stderr, "==> found 1 possible issue")
		} else if msgCount > 1 {
			fmt.Fprintf(os.Stderr, "==> found %v possible issues\n", msgCount)
		}
	}
	return nil
}

func calculateZids(metaList []api.ZidMetaRights) ([]id.Zid, []int) {
	zids := make([]id.Zid, len(metaList))
	for i, m := range metaList {
		zids[i] = m.ID
	}
	slices.Sort(zids)
	return zids, rand.Perm(len(metaList))
}

func zidsToUse(zids []id.Zid, perm []int, sampleSize int) []id.Zid {
	if sampleSize < 0 || len(perm) <= sampleSize {
		return zids
	}
	if sampleSize == 0 {
		return nil
	}
	result := make([]id.Zid, sampleSize)
	for i := range sampleSize {
		result[i] = zids[perm[i]]
	}
	slices.Sort(result)
	return result
}

var keyDescr = []struct {
	uc         urlCreator
	text       string
	sampleSize int
}{
	{getHTMLZettel, "zettel HTML encoding", -1},
	{createJustKey('h'), "zettel web view", -1},
	{createJustKey('i'), "zettel info view", -1},
	{createJustKey('e'), "zettel edit form", 100},
	{createJustKey('c'), "zettel create form", 10},
	{createJustKey('d'), "zettel delete dialog", 200},
}

type urlCreator func(*client.Client, id.Zid) *api.URLBuilder

func createJustKey(key byte) urlCreator {
	return func(c *client.Client, zid id.Zid) *api.URLBuilder {
		return c.NewURLBuilder(key).SetZid(zid)
	}
}

func getHTMLZettel(client *client.Client, zid id.Zid) *api.URLBuilder {
	return client.NewURLBuilder('z').SetZid(zid).
		AppendKVQuery(api.QueryKeyEncoding, api.EncodingHTML).
		AppendKVQuery(api.QueryKeyPart, api.PartZettel)
}

func validateHTML(client *client.Client, uc urlCreator, zid id.Zid) (int, error) {
	ub := uc(client, zid)
	if tools.Verbose {
		fmt.Fprintf(os.Stderr, "GET %v\n", ub)
	}
	data, err := client.Get(context.Background(), ub)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	_, stderr, err := tools.ExecuteFilter(data, nil, "tidy", "-e", "-q", "-lang", "en")
	if err != nil {
		switch err.Error() {
		case "exit status 1":
		case "exit status 2":
		default:
			log.Println("SERR", stderr)
			return 0, err
		}
	}
	if stderr == "" {
		return 0, nil
	}
	if msgs := filterTidyMessages(strings.Split(stderr, "\n")); len(msgs) > 0 {
		fmt.Fprintln(os.Stderr, zid)
		for _, msg := range msgs {
			fmt.Fprintln(os.Stderr, "-", msg)
		}
		return len(msgs), nil
	}
	return 0, nil
}

var reLine = regexp.MustCompile(`line \d+ column \d+ - (.+): (.+)`)

func filterTidyMessages(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := reLine.FindStringSubmatch(line)
		if len(matches) <= 1 {
			if line == "This document has errors that must be fixed before" ||
				line == "using HTML Tidy to generate a tidied up version." {
				continue
			}
			result = append(result, "!!!"+line)
			continue
		}
		if matches[1] == "Error" {
			if len(matches) > 2 {
				if matches[2] == "<search> is not recognized!" {
					continue
				}
			}
		}
		if matches[1] != "Warning" {
			result = append(result, "???"+line)
			continue
		}
		if len(matches) > 2 {
			switch matches[2] {
			case "discarding unexpected <search>",
				"discarding unexpected </search>",
				`<input> proprietary attribute "inputmode"`:
				continue
			}
		}
		result = append(result, line)
	}
	return result
}
