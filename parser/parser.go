//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

// Package parser provides a generic interface to a range of different parsers.
package parser

import (
	"context"
	"fmt"
	"strings"

	"zettelstore.de/client.fossil/api"
	"zettelstore.de/z/ast"
	"zettelstore.de/z/config"
	"zettelstore.de/z/input"
	"zettelstore.de/z/parser/cleaner"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/meta"
)

// Info describes a single parser.
//
// Before ParseBlocks() or ParseInlines() is called, ensure the input stream to
// be valid. This can ce achieved on calling inp.Next() after the input stream
// was created.
type Info struct {
	Name          string
	AltNames      []string
	IsASTParser   bool
	IsTextFormat  bool
	IsImageFormat bool
	ParseBlocks   func(*input.Input, *meta.Meta, string) ast.BlockSlice
	ParseInlines  func(*input.Input, string) ast.InlineSlice
}

var registry = map[string]*Info{}

// Register the parser (info) for later retrieval.
func Register(pi *Info) {
	if _, ok := registry[pi.Name]; ok {
		panic(fmt.Sprintf("Parser %q already registered", pi.Name))
	}
	registry[pi.Name] = pi
	for _, alt := range pi.AltNames {
		if _, ok := registry[alt]; ok {
			panic(fmt.Sprintf("Parser %q already registered", alt))
		}
		registry[alt] = pi
	}
}

// GetSyntaxes returns a list of syntaxes implemented by all registered parsers.
func GetSyntaxes() []string {
	result := make([]string, 0, len(registry))
	for syntax := range registry {
		result = append(result, syntax)
	}
	return result
}

// Get the parser (info) by name. If name not found, use a default parser.
func Get(name string) *Info {
	if pi := registry[name]; pi != nil {
		return pi
	}
	if pi := registry["plain"]; pi != nil {
		return pi
	}
	panic(fmt.Sprintf("No parser for %q found", name))
}

// IsASTParser returns whether the given syntax parses text into an AST or not.
func IsASTParser(syntax string) bool {
	pi, ok := registry[syntax]
	if !ok {
		return false
	}
	return pi.IsASTParser
}

// IsImageFormat returns whether the given syntax is known to be an image format.
func IsImageFormat(syntax string) bool {
	pi, ok := registry[syntax]
	if !ok {
		return false
	}
	return pi.IsImageFormat
}

// ParseBlocks parses some input and returns a slice of block nodes.
func ParseBlocks(inp *input.Input, m *meta.Meta, syntax string, hi config.HTMLInsecurity) ast.BlockSlice {
	bs := Get(syntax).ParseBlocks(inp, m, syntax)
	cleaner.CleanBlockSlice(&bs, hi.AllowHTML(syntax))
	return bs
}

// ParseInlines parses some input and returns a slice of inline nodes.
func ParseInlines(inp *input.Input, syntax string) ast.InlineSlice {
	// Do not clean, because we don't know the context where this function will be called.
	return Get(syntax).ParseInlines(inp, syntax)
}

// ParseMetadata parses a string as Zettelmarkup, resulting in an inline slice.
// Typically used to parse the title or other metadata of type Zettelmarkup.
func ParseMetadata(value string) ast.InlineSlice {
	return ParseInlines(input.NewInput([]byte(value)), meta.SyntaxZmk)
}

// ParseSpacedText returns an inline slice that consists just of test and space node.
// No Zettelmarkup parsing is done. It is typically used to transform the zettel title into an inline slice.
func ParseSpacedText(s string) ast.InlineSlice {
	return ast.CreateInlineSliceFromWords(meta.ListFromValue(s)...)
}

// NormalizedSpacedText returns the given string, but normalize multiple spaces to one space.
func NormalizedSpacedText(s string) string { return strings.Join(meta.ListFromValue(s), " ") }

// ParseDescription returns a suitable description stored in the metadata as an inline slice.
// This is done for an image in most cases.
func ParseDescription(m *meta.Meta) ast.InlineSlice {
	if m == nil {
		return nil
	}
	if descr, found := m.Get(api.KeySummary); found {
		in := ParseMetadata(descr)
		cleaner.CleanInlineLinks(&in)
		return in
	}
	if title, found := m.Get(api.KeyTitle); found {
		return ParseSpacedText(title)
	}
	return ast.CreateInlineSliceFromWords("Zettel", "without", "title:", m.Zid.String())
}

// ParseZettel parses the zettel based on the syntax.
func ParseZettel(ctx context.Context, zettel zettel.Zettel, syntax string, rtConfig config.Config) *ast.ZettelNode {
	m := zettel.Meta
	inhMeta := m
	if rtConfig != nil {
		inhMeta = rtConfig.AddDefaultValues(ctx, inhMeta)
	}
	if syntax == "" {
		syntax = inhMeta.GetDefault(api.KeySyntax, meta.DefaultSyntax)
	}
	parseMeta := inhMeta
	if syntax == meta.SyntaxNone {
		parseMeta = m
	}

	hi := config.NoHTML
	if rtConfig != nil {
		hi = rtConfig.GetHTMLInsecurity()
	}
	return &ast.ZettelNode{
		Meta:    m,
		Content: zettel.Content,
		Zid:     m.Zid,
		InhMeta: inhMeta,
		Ast:     ParseBlocks(input.NewInput(zettel.Content.AsBytes()), parseMeta, syntax, hi),
		Syntax:  syntax,
	}
}
